package xginx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"

	"github.com/mattn/go-colorable"

	"github.com/gin-gonic/gin"
)

func LoadPrivateKeys(file string) []*PrivateKey {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	ss := []string{}
	if err := json.Unmarshal(data, &ss); err != nil {
		panic(err)
	}
	ret := []*PrivateKey{}
	for _, s := range ss {
		pk, err := LoadPrivateKey(s)
		if err != nil {
			panic(err)
		}
		ret = append(ret, pk)
	}
	return ret
}

//配置加载后只读
type Config struct {
	ObjIdPrefix  string                  `json:"oid_prefix"`    //物品id前缀
	AddrPrefix   string                  `json:"addr_prefix"`   //地址前缀
	GenesisBlock string                  `json:"genesis_block"` //第一个区块
	HttpScheme   string                  `json:"http_scheme"`   //http
	LogFile      string                  `json:"log_file"`      //日志文件
	HttpPort     int                     `json:"http_port"`     //http服务器端口
	MinerPKey    string                  `json:"miner_pkey"`    //矿工产出私钥
	PowTime      uint                    `json:"pow_time"`      //14 * 24 * 60 * 60=1209600
	PowLimit     string                  `json:"pow_limit"`     //最小难度设置
	SpanTime     float64                 `json:"span_time"`     //两次记录时间差超过这个时间将被忽略距离计算，单位小时
	MaxSpeed     float64                 `json:"max_speed"`     //最大速度 km/h
	DisRange     []uint                  `json:"dis_range"`     //适合的距离范围500范围内有效-2000范围外无效,500-2000递减
	Halving      uint                    `json:"halving"`       //210000
	Flags        string                  `json:"flags"`         //协议头标记
	Ver          uint32                  `json:"version"`       //节点版本
	Publics      []string                `json:"pubs"`          //节点信任的公钥=只用来验证证书是否正确 +前缀代表可用 -前缀标识弃用的公钥
	TcpPort      int                     `json:"tcp_port"`      //服务端口和ip
	TcplIp       string                  `json:"tcp_lip"`       //服务ip
	TcprIp       string                  `json:"tcp_rip"`       //节点远程连接ip
	Privates     []string                `json:"pris"`          //用于签名的私钥
	Certs        []string                `json:"certs"`         //已经签名的证书
	TimeErr      float64                 `json:"time_err"`      //时间误差 秒 客户端时间与服务器时间差在这个范围内
	pris         map[PKBytes]*PrivateKey `json:"-"`             //
	pubs         map[PKBytes]*PublicKey  `json:"-"`             //
	certs        map[PKBytes]*Cert       `json:"-"`             //
	pubshash     HASH256                 `json:"-"`             //
	mu           sync.RWMutex            `json:"-"`             //
	NodeID       HASH160                 `json:"-"`             //启动时临时生成
	minerpk      *PublicKey              `json:"-"`             //私钥
	logFile      *os.File                `json:"-"`             //日志文件
	genesisId    HASH256                 `json:"-"`             //第一个区块id
	LimitHash    UINT256                 `json:"-"`             //最小工作难度
}

func (c *Config) GetMinerPubKey() *PublicKey {
	return c.minerpk
}

func (c *Config) GetListenAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP(c.TcplIp),
		port: uint16(c.TcpPort),
	}
}

func (c *Config) GetNetAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP(c.TcprIp),
		port: uint16(c.TcpPort),
	}
}

//编码证书用于证书交换
// 32 byte,pubs hash
// 1 byte cert num
// 1 byte cert[0] length
// n cert bytes
// 1 byte cert[1] length
// n cert bytes
func (c *Config) EncodeCerts() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	vhash := c.PubsHash()
	buf := &bytes.Buffer{}
	if _, err := buf.Write(vhash[:]); err != nil {
		return nil, err
	}
	num := uint8(0)
	for _, v := range c.certs {
		if err := v.Verify(); err != nil {
			continue
		}
		num++
	}
	if err := binary.Write(buf, Endian, num); err != nil {
		return nil, err
	}
	for _, v := range c.certs {
		if err := v.Verify(); err != nil {
			continue
		}
		tmp := &bytes.Buffer{}
		if err := v.Encode(tmp); err != nil {
			return nil, err
		}
		if tmp.Len() > 255 {
			return nil, errors.New("cert length is too long")
		}
		if err := binary.Write(buf, Endian, uint8(tmp.Len())); err != nil {
			return nil, err
		}
		if _, err := buf.Write(tmp.Bytes()); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (c *Config) DecodeCerts(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	vhash := c.PubsHash()
	buf := bytes.NewReader(b)
	hash := HASH256{}
	if _, err := buf.Read(hash[:]); err != nil {
		return err
	}
	if hash.IsZero() || !hash.Equal(vhash) {
		return errors.New("publics hash error")
	}
	num := uint8(0)
	if err := binary.Read(buf, Endian, &num); err != nil {
		return err
	}
	for i := 0; i < int(num); i++ {
		bl := uint8(0)
		if err := binary.Read(buf, Endian, &bl); err != nil {
			return err
		}
		cb := make([]byte, bl)
		if _, err := buf.Read(cb); err != nil {
			return err
		}
		cert := &Cert{}
		if err := cert.Decode(bytes.NewReader(cb)); err != nil {
			return err
		}
		if err := cert.Verify(); err != nil {
			log.Println("cert verify error", err, "skip cert", cert.Name)
			continue
		}
		if _, has := c.certs[cert.PubKey]; !has {
			c.certs[cert.PubKey] = cert
		}
	}
	return nil
}

//两个客户端hash公钥配置必须一致
//节点不能任意添加信任公钥
func (c *Config) PubsHash() HASH256 {
	return c.pubshash
}

func (c *Config) SetCert(cert *Cert) (*Cert, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := cert.Verify(); err != nil {
		return nil, err
	}
	c.certs[cert.PubKey] = cert
	return cert, nil
}

func (c *Config) Verify(pk PKBytes, sig *SigValue, hash []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cert, ok := c.certs[pk]
	if !ok {
		return errors.New("cert miss")
	}
	if err := cert.Verify(); err != nil {
		return err
	}
	if !cert.PublicKey().Verify(hash, sig) {
		return errors.New("verify sig error")
	}
	return nil
}

//获取信任的证书公钥
func (c *Config) GetPublicKey(pk PKBytes) *PublicKey {
	return c.pubs[pk]
}

//任意获取一个节点可用的私钥
func (c *Config) GetPrivateKey() *PrivateKey {
	for k, v := range c.certs {
		if err := v.Verify(); err == nil {
			return c.pris[k]
		}
	}
	return nil
}

func (c *Config) Close() {
	if c.logFile != nil {
		_ = c.logFile.Close()
	}
}

func (c *Config) IsGenesisId(id HASH256) bool {
	return c.genesisId.Equal(id)
}

func (c *Config) Init() error {
	//设置日志输出
	logflags := log.Llongfile | log.LstdFlags | log.Lmicroseconds
	if c.LogFile != "" {
		file, err := os.OpenFile(c.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			panic(err)
		}
		c.logFile = file
		log.SetOutput(file)
		log.SetFlags(logflags)
		gin.DefaultWriter = file
		gin.DefaultErrorWriter = file
	} else {
		c.logFile = nil
		log.SetOutput(os.Stdout)
		log.SetFlags(logflags)
		gin.DefaultWriter = colorable.NewColorableStdout()
		gin.DefaultErrorWriter = colorable.NewColorableStderr()
	}
	//设置第一个区块id
	c.genesisId = NewHASH256(c.GenesisBlock)
	//加载矿工私钥
	pk, err := LoadPublicKey(c.MinerPKey)
	if err == nil {
		c.minerpk = pk
	}
	c.LimitHash = NewUINT256(c.PowLimit)
	//随机生成节点ID
	c.NodeID = NewNodeID()
	//加载私钥
	for _, s := range c.Privates {
		pri, err := LoadPrivateKey(s)
		if err != nil {
			return err
		}
		pub := pri.PublicKey()
		pk := new(PKBytes).Set(pub)
		c.pris[pk] = pri
	}
	//加载信任公钥
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, Endian, c.Ver); err != nil {
		return err
	}
	for _, s := range c.Publics {
		pub, err := LoadPublicKey(s)
		if err != nil {
			return err
		}
		pk := new(PKBytes).Set(pub)
		c.pubs[pk] = pub
		if _, err := buf.Write(pub.Encode()); err != nil {
			return err
		}
	}
	//获取版本hash
	copy(c.pubshash[:], Hash256(buf.Bytes()))
	//加载证书
	for _, s := range c.Certs {
		cert, err := LoadCert(s)
		if err != nil {
			return err
		}
		if err := cert.Verify(); err != nil {
			log.Println("cert untrusted", hex.EncodeToString(cert.PubKey[:]), err)
			continue
		}
		if _, ok := c.pubs[cert.VPub]; ok {
			c.certs[cert.PubKey] = cert
		}
	}
	return nil
}

var (
	conf *Config = nil
)

func init() {
	LoadConfig("v10000.json") //测试配置文件
}

func Close() {
	conf.Close()
}

func LoadConfig(f string) {
	d, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}
	conf = &Config{
		pubs:  map[PKBytes]*PublicKey{},
		certs: map[PKBytes]*Cert{},
		pris:  map[PKBytes]*PrivateKey{},
	}
	if err := json.Unmarshal(d, conf); err != nil {
		panic(err)
	}
	if err := conf.Init(); err != nil {
		panic(err)
	}
}
