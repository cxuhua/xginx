package xginx

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"sync"
)

type CertItem struct {
	Priv string `json:"priv"`
	Cert string `json:"cert"`
}

type Config struct {
	Privates []string          `json:"pris"`  //用于签发下级证书
	Publics  []string          `json:"pubs"`  //节点信任的公钥，用来验证本节点的证书
	Certs    []CertItem        `json:"certs"` //节点证书,上级签发的证书，由信任的公钥进行签名验证
	pris     []*PrivateKey     `json:"-"`     //
	pubs     []*PublicKey      `json:"-"`
	certmap  map[PKBytes]*Cert `json:"-"` //按公钥key保存
	certidx  []*Cert           `json:"-"` //顺序跑村
}

//获取根证书公钥
func (c *Config) GetRootPublicKey(idx uint16) *PublicKey {
	return c.pubs[idx]
}

//获取根证书私钥钥
func (c *Config) GetRootPrivateKey(idx uint16) *PrivateKey {
	return c.pris[idx]
}

// 根据公钥获取证书
func (c *Config) GetNodeCert(k interface{}) (*Cert, error) {
	var cert *Cert = nil
	switch k.(type) {
	case uint16:
		cert = c.certidx[k.(uint16)]
	case int:
		cert = c.certidx[k.(int)]
	case PKBytes:
		cert = c.certmap[k.(PKBytes)]
	default:
		return nil, errors.New("args type error")
	}
	if cert == nil {
		return nil, errors.New("not found")
	}
	if err := cert.Verify(); err != nil {
		return nil, err
	}
	return cert, nil
}

func (c *Config) Init() error {
	for _, s := range c.Privates {
		pri, err := LoadPrivateKey(s)
		if err != nil {
			return err
		}
		c.pris = append(c.pris, pri)
	}
	for _, s := range c.Publics {
		pub, err := LoadPublicKey(s)
		if err != nil {
			return err
		}
		c.pubs = append(c.pubs, pub)
	}
	for _, i := range c.Certs {
		cert, err := LoadCert(i.Priv, i.Cert)
		if err != nil {
			return err
		}
		if _, ok := c.certmap[cert.PubKey]; ok {
			continue
		}
		c.certmap[cert.PubKey] = cert
		c.certidx = append(c.certidx, cert)
	}
	return nil
}

var (
	Conf *Config = nil
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
	Conf = &Config{
		certmap: map[PKBytes]*Cert{},
	}
	if err := json.Unmarshal(d, Conf); err != nil {
		panic(err)
	}
	if err := Conf.Init(); err != nil {
		panic(err)
	}
}
