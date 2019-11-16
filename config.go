package xginx

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"

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
	Seeds        []string     `json:"seeds"`         //dns seed服务器
	WalletDir    string       `json:"wallet_dir"`    //钱包地址
	DataDir      string       `json:"data_dir"`      //数据路径
	AddrPrefix   string       `json:"addr_prefix"`   //地址前缀
	GenesisBlock string       `json:"genesis_block"` //第一个区块
	LogFile      string       `json:"log_file"`      //日志文件路径
	HttpPort     int          `json:"http_port"`     //http服务器端口
	PowTime      uint         `json:"pow_time"`      //14 * 24 * 60 * 60=1209600
	PowLimit     string       `json:"pow_limit"`     //最小难度设置
	PowSpan      uint32       `json:"pow_span"`      //难度计算间隔 2016
	Halving      int          `json:"halving"`       //210000
	Flags        string       `json:"flags"`         //协议头标记
	Ver          uint32       `json:"version"`       //节点版本
	TcpPort      int          `json:"tcp_port"`      //服务端口和ip
	TcplIp       string       `json:"tcp_lip"`       //服务ip
	TcprIp       string       `json:"tcp_rip"`       //节点远程连接ip
	Debug        bool         `json:"debug"`         //是否在测试模式
	RpcPort      int          `json:"rpc_port"`      //rpc服务端口
	RpclIp       string       `json:"rpc_lip"`       //rpc服务器地址
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

func (c *Config) GetRpcListenAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP(c.RpclIp),
		port: uint16(c.RpcPort),
	}
}

func (c *Config) GetTcpListenAddr() NetAddr {
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
		file, err := os.OpenFile(c.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
		if err != nil {
			panic(err)
		}
		c.logFile = file
	} else {
		c.logFile = os.Stdout
	}
	log.SetOutput(c.logFile)
	gin.DefaultWriter = c.logFile
	gin.DefaultErrorWriter = c.logFile
	log.SetFlags(logflags)
	if c.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	//设置第一个区块id
	c.genesisId = NewHASH256(c.GenesisBlock)
	c.LimitHash = NewUINT256(c.PowLimit)
	c.NodeID = NewNodeID(c)
	return nil
}

var (
	conf *Config = nil
)

func InitConfig(f string) *Config {
	conf = LoadConfig(f)
	return conf
}

func LoadConfig(f string) *Config {
	d, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}
	sconf := &Config{}
	if err := json.Unmarshal(d, sconf); err != nil {
		panic(err)
	}
	if err := sconf.Init(); err != nil {
		panic(err)
	}
	return sconf
}
