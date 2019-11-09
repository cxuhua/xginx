package xginx

import (
	"encoding/json"
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
	DataDir      string       `json:"data_dir"`      //数据路径
	ObjIdPrefix  string       `json:"oid_prefix"`    //物品id前缀
	AddrPrefix   string       `json:"addr_prefix"`   //地址前缀
	GenesisBlock string       `json:"genesis_block"` //第一个区块
	HttpScheme   string       `json:"http_scheme"`   //http服务类型
	LogFile      string       `json:"log_file"`      //日志文件路径
	HttpPort     int          `json:"http_port"`     //http服务器端口
	MinerPKey    string       `json:"miner_pkey"`    //矿工产出公钥
	PowTime      uint         `json:"pow_time"`      //14 * 24 * 60 * 60=1209600
	PowLimit     string       `json:"pow_limit"`     //最小难度设置
	PowSpan      uint32       `json:"pow_span"`      //难度计算间隔 2016
	SpanTime     float64      `json:"span_time"`     //两次记录时间差超过这个时间将被忽略距离计算，单位小时
	MaxSpeed     float64      `json:"max_speed"`     //最大速度 km/h
	DisRange     []uint       `json:"dis_range"`     //适合的距离范围500范围内有效-2000范围外无效,500-2000递减
	Halving      uint         `json:"halving"`       //210000
	Flags        string       `json:"flags"`         //协议头标记
	Ver          uint32       `json:"version"`       //节点版本
	TcpPort      int          `json:"tcp_port"`      //服务端口和ip
	TcplIp       string       `json:"tcp_lip"`       //服务ip
	TcprIp       string       `json:"tcp_rip"`       //节点远程连接ip
	Privates     []string     `json:"pris"`          //用于签名的私钥
	Certs        []string     `json:"certs"`         //已经签名的证书
	TimeErr      float64      `json:"time_err"`      //时间误差 秒 客户端时间与服务器时间差在这个范围内
	mu           sync.RWMutex `json:"-"`             //
	NodeID       HASH160      `json:"-"`             //启动时临时生成 MinerPKey 生成
	minerpk      *PublicKey   `json:"-"`             //矿工公钥
	logFile      *os.File     `json:"-"`             //日志文件
	genesisId    HASH256      `json:"-"`             //第一个区块id
	LimitHash    UINT256      `json:"-"`             //最小工作难度
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
	if pk, err := LoadPublicKey(c.MinerPKey); err != nil {
		panic(err)
	} else {
		c.minerpk = pk
	}
	c.LimitHash = NewUINT256(c.PowLimit)
	//随机生成节点ID
	c.NodeID = NewNodeID()
	return nil
}

var (
	conf *Config = nil
)

func init() {
	//加载默认配置文件
	LoadConfig("v10000.json") //测试配置文件
	//创建默认存储器
	store = NewLevelDBStore(conf.DataDir)
}

func Close() {
	store.Close()
	conf.Close()
}

func LoadConfig(f string) {
	d, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}
	conf = &Config{}
	if err := json.Unmarshal(d, conf); err != nil {
		panic(err)
	}
	if err := conf.Init(); err != nil {
		panic(err)
	}
}
