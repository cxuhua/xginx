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
	"sync"
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
	//当前节点版本
	Ver        uint32                  `json:"version"`     //版本
	Publics    []string                `json:"pubs"`        //节点信任的公钥=只用来验证证书是否正确 +前缀代表可用 -前缀标识弃用的公钥
	ListenPort int                     `json:"listen_port"` //服务端口和ip
	ListenIp   string                  `json:"listen_ip"`   //服务ip
	RemoteIp   string                  `json:"remote_ip"`   //远程连接ip
	Privates   []string                `json:"pris"`        //用于签名的私钥
	Certs      []string                `json:"certs"`       //已经签名的证书
	pris       map[PKBytes]*PrivateKey `json:"-"`           //
	pubs       map[PKBytes]*PublicKey  `json:"-"`           //
	certs      map[PKBytes]*Cert       `json:"-"`           //
	verhash    HashID
	mu         sync.RWMutex
}

func (c *Config) GetListenAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP(c.ListenIp),
		port: uint16(c.ListenPort),
	}
}

func (c *Config) GetNetAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP(c.RemoteIp),
		port: uint16(c.ListenPort),
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
	vhash := c.VerHash()
	buf := &bytes.Buffer{}
	if _, err := buf.Write(vhash[:]); err != nil {
		return nil, err
	}
	num := uint8(len(c.certs))
	if err := binary.Write(buf, Endian, num); err != nil {
		return nil, err
	}
	for _, v := range c.certs {
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
	vhash := c.VerHash()
	buf := bytes.NewReader(b)
	hash := HashID{}
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
			continue
		}
		c.certs[cert.PubKey] = cert
	}
	return nil
}

//两个客户端hash公钥配置必须一致
//节点不能任意添加信任公钥
func (c *Config) VerHash() HashID {
	return c.verhash
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

func (c *Config) Init() error {
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
	hash := HASH256(buf.Bytes())
	copy(c.verhash[:], hash)
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
	//加载版本v10000配置文件
	LoadConfig("v10000.json") //测试配置文件
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
