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

var (
	ConfFile = flag.String("conf", "v10000.json", "config file name")
	IsDebug  = flag.Bool("debug", true, "startup mode")
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
	Confirms   uint32   `json:"confirms"`    //确认数 = 6
	MinerNum   int      `json:"miner_num"`   //挖掘机数量,=0不会启动协程计算
	MinerAddr  Address  `json:"miner_addr"`  //矿工默认地址
	MaxConn    int      `json:"max_conn"`    //最大激活的连接，包括连入和连出的
	Seeds      []string `json:"seeds"`       //dns seed服务器
	DataDir    string   `json:"data_dir"`    //数据路径
	AddrPrefix string   `json:"addr_prefix"` //地址前缀
	Genesis    string   `json:"genesis"`     //第一个区块
	LogFile    string   `json:"log_file"`    //日志文件路径
	PowTime    uint     `json:"pow_time"`    //14 * 24 * 60 * 60=1209600
	PowLimit   string   `json:"pow_limit"`   //最小难度设置
	PowSpan    uint32   `json:"pow_span"`    //难度计算间隔 2016
	Halving    int      `json:"halving"`     //210000
	Ver        uint32   `json:"version"`     //节点版本
	TcpPort    int      `json:"tcp_port"`    //服务端口和ip
	TcpIp      string   `json:"tcp_ip"`      //节点远程连接ip
	Flags      [4]byte  `json:"-"`           //协议标识
	logFile    *os.File `json:"-"`           //日志文件
	genesis    HASH256  `json:"-"`           //第一个区块id
	LimitHash  UINT256  `json:"-"`           //最小工作难度
	nodeid     uint64   `json:"-"`           //节点随机id
}

func (c *Config) GetTcpListenAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP("0.0.0.0"),
		port: uint16(c.TcpPort),
	}
}

func (c *Config) GetLogFile() *os.File {
	return c.logFile
}

func (c *Config) GetNetAddr() NetAddr {
	return NetAddr{
		ip:   net.ParseIP(c.TcpIp),
		port: uint16(c.TcpPort),
	}
}

func (c *Config) Close() {
	if c.logFile != nil {
		_ = c.logFile.Close()
	}
}

func (c *Config) IsGenesisId(id HASH256) bool {
	return c.genesis.Equal(id)
}

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

func (c *Config) Init() *Config {
	c.Flags = [4]byte{'x', 'h', 'l', 'm'}
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

func GetConfig() *Config {
	return conf
}

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
