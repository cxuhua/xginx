package xginx

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
)

const (
	//当前版本
	Version = "v0.0.1"
)

// 启动参数
var (
	ConfFile = flag.String("conf", "v10000.json", "config file name")
	IsDebug  = flag.Bool("debug", true, "startup mode")
)

//Config 配置加载后只读
type Config struct {
	Name      string   `json:"name"`      //配置文件名称
	Confirms  uint32   `json:"confirms"`  //安全确认数 = 6
	MinerNum  int      `json:"miner_num"` //挖掘机数量,=0不会启动协程挖矿
	MaxConn   int      `json:"max_conn"`  //最大激活的连接，包括连入和连出的
	Seeds     []string `json:"seeds"`     //dns seed服务器
	DataDir   string   `json:"data_dir"`  //数据路径
	Genesis   string   `json:"genesis"`   //第一个区块
	LogFile   string   `json:"log_file"`  //日志文件路径
	PowTime   uint     `json:"pow_time"`  //14 * 24 * 60 * 60=1209600
	PowLimit  string   `json:"pow_limit"` //最小难度设置
	PowSpan   uint32   `json:"pow_span"`  //难度计算间隔 2016
	Halving   int      `json:"halving"`   //210000减产配置
	Ver       uint32   `json:"version"`   //节点版本
	TCPPort   int      `json:"tcp_port"`  //服务端口和ip
	TCPIp     string   `json:"tcp_ip"`    //节点远程连接ip
	LimitHash UINT256  `json:"-"`         //最小工作难度
	Nodes     []string `json:"nodes"`     //配置的可用节点
	flags     [4]byte  //协议标识
	logFile   *os.File //日志文件
	genesis   HASH256  //第一个区块id
	nodeid    uint64   //节点随机id
}

//GetTCPListenAddr 获取服务地址
func (c *Config) GetTCPListenAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP("0.0.0.0"),
		port: uint16(c.TCPPort),
	}
}

//GetLogFile 获取日志文件
func (c *Config) GetLogFile() *os.File {
	return c.logFile
}

//GetNetAddr 获取网络地址
func (c *Config) GetNetAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP(c.TCPIp),
		port: uint16(c.TCPPort),
	}
}

//Close 关闭
func (c *Config) Close() {
	if c.logFile != nil {
		_ = c.logFile.Close()
	}
}

//IsGenesisID 检测id是否是第一个区块
func (c *Config) IsGenesisID(id HASH256) bool {
	return c.genesis.Equal(id)
}

//GenUInt64 创建64位随机值
func (c *Config) GenUInt64() uint64 {
	//生成节点随机id
	kb0 := make([]byte, 8)
	_, err := rand.Read(kb0)
	if err != nil {
		panic(err)
	}
	kb1 := make([]byte, 8)
	_, err = rand.Read(kb1)
	if err != nil {
		panic(err)
	}
	buf := NewWriter()
	addr := c.GetNetAddr()
	err = addr.Encode(buf)
	if err != nil {
		panic(err)
	}
	k0 := Endian.Uint64(kb0)
	k1 := Endian.Uint64(kb1)
	return SipHash(k0, k1, buf.Bytes())
}

//Init 初始化
func (c *Config) Init() *Config {
	c.flags = [4]byte{'x', 'h', 'l', 'm'}
	//设置日志输出
	logflags := log.Llongfile | log.LstdFlags | log.Lmicroseconds
	if c.LogFile != "" {
		file, err := os.OpenFile(c.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		c.logFile = file
	} else {
		c.logFile = os.Stdout
	}
	log.SetOutput(c.logFile)
	log.SetFlags(logflags)
	//
	c.nodeid = c.GenUInt64()
	LogInfof("gen new node id %x", c.nodeid)
	//设置第一个区块id
	c.genesis = NewHASH256(c.Genesis)
	c.LimitHash = NewUINT256(c.PowLimit)
	return c
}

var (
	conf *Config = nil
)

//GetConfig 获取当前配置
func GetConfig() *Config {
	return conf
}

//InitConfig 初始化配置
func InitConfig(file ...string) *Config {
	if *ConfFile == "" && len(file) == 0 {
		panic(errors.New("config file miss -conf"))
	}
	if len(file) > 0 {
		conf = LoadConfig(file[0])
	} else {
		conf = LoadConfig(*ConfFile)
	}
	return conf
}

//LoadConfig 加载配置
func LoadConfig(f string) *Config {
	d, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}
	sconf := &Config{}
	if err := json.Unmarshal(d, sconf); err != nil {
		panic(err)
	}
	return sconf.Init()
}
