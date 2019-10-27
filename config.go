package xginx

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"sync"
)

type CertItem struct {
	Priv string `json:"priv"`
	Cert string `json:"cert"`
}

//配置加载后只读
type Config struct {
	ListenPort int                    `json:"listen_port"` //服务端口和ip
	ListenIp   string                 `json:"listen_ip"`   //服务ip
	Privates   []string               `json:"pris"`        //用于签发下级证书
	Publics    []string               `json:"pubs"`        //节点信任的公钥，用来验证本节点的证书
	Certs      []CertItem             `json:"certs"`       //节点证书,上级和自己签发的证书，由信任的公钥进行签名验证
	pris       []*PrivateKey          `json:"-"`           //
	pubs       map[PKBytes]*PublicKey `json:"-"`           //
	certs      []*Cert                `json:"-"`           //顺序保存
}

func (c *Config) NewCertPool() *CertPool {
	cp := NewCertPool()
	for _, v := range c.certs {
		cc, err := v.Clone()
		if err != nil {
			continue
		}
		cp.Set(cc)
	}
	return cp
}

//获取信任的证书公钥
func (c *Config) GetPublicKey(pk PKBytes) *PublicKey {
	return c.pubs[pk]
}

//获取节点设置的私钥
func (c *Config) GetPrivateKey(idx uint16) *PrivateKey {
	return c.pris[idx]
}

func (c *Config) Init() error {
	for _, s := range c.Privates {
		pri, err := LoadPrivateKey(s)
		if err != nil {
			return err
		}
		c.pris = append(c.pris, pri)
		pub := pri.PublicKey()
		pk := new(PKBytes).Set(pub)
		if _, ok := c.pubs[pk]; !ok {
			c.pubs[pk] = pub
		}
	}
	for _, s := range c.Publics {
		pub, err := LoadPublicKey(s)
		if err != nil {
			return err
		}
		pk := new(PKBytes).Set(pub)
		if _, ok := c.pubs[pk]; !ok {
			c.pubs[pk] = pub
		}
	}
	for _, i := range c.Certs {
		cert, err := LoadCert(i.Priv, i.Cert)
		if err != nil {
			return err
		}
		if err := cert.Verify(); err != nil {
			log.Println("cert untrusted", hex.EncodeToString(cert.PubKey[:]), err)
			continue
		}
		if _, ok := c.pubs[cert.VPub]; ok {
			c.certs = append(c.certs, cert)
		}
	}
	return nil
}

var (
	conf *Config = nil
	once         = sync.Once{}
)

func init() {
	once.Do(func() {
		LoadConfig("test.json") //测试配置文件
	})
}

func LoadConfig(f string) {
	d, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}
	conf = &Config{
		pubs: map[PKBytes]*PublicKey{},
	}
	if err := json.Unmarshal(d, conf); err != nil {
		panic(err)
	}
	if err := conf.Init(); err != nil {
		panic(err)
	}
}
