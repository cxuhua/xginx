package xginx

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"sync"
)

type CertItem struct {
	Priv string `json:"priv"`
	Cert string `json:"cert"`
}

type Config struct {
	Privates []string               `json:"pris"`  //用于签发下级证书
	Publics  []string               `json:"pubs"`  //节点信任的公钥，用来验证本节点的证书
	Certs    []CertItem             `json:"certs"` //节点证书,上级和自己签发的证书，由信任的公钥进行签名验证
	pris     []*PrivateKey          `json:"-"`     //
	pubs     map[PKBytes]*PublicKey `json:"-"`     //
	certmap  map[PKBytes]*Cert      `json:"-"`     //按公钥key保存
	certidx  []*Cert                `json:"-"`     //顺序保存
	mu       sync.Mutex             `json:"-"`
}

//获取根证书公钥
func (c *Config) GetPublicKey(pk PKBytes) *PublicKey {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pubs[pk]
}

//获取根证书私钥钥
func (c *Config) GetPrivateKey(idx uint16) *PrivateKey {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pris[idx]
}

func (c *Config) GetIndexCert(idx int) (*Cert, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if idx < 0 || idx >= len(c.certidx) {
		return nil, errors.New("not found")
	}
	return c.certidx[idx], nil
}

// 根据公钥获取证书
func (c *Config) GetNodeCert(pk PKBytes) (*Cert, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cert, ok := c.certmap[pk]
	if !ok || cert == nil {
		return nil, errors.New("not found")
	}
	return cert, nil
}

func (c *Config) SetCert(cert *Cert) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.pubs[cert.Prev]; ok {
		c.certmap[cert.PubKey] = cert
		c.certidx = append(c.certidx, cert)
	} else {
		log.Println("cert untrusted", hex.EncodeToString(cert.PubKey[:]))
	}
}

func (c *Config) Init() error {
	for _, s := range c.Privates {
		pri, err := LoadPrivateKey(s)
		if err != nil {
			return err
		}
		c.pris = append(c.pris, pri)
		//自己有私钥肯定信任自己的公钥
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
		c.SetCert(cert)
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
		certmap: map[PKBytes]*Cert{},
		pubs:    map[PKBytes]*PublicKey{},
	}
	if err := json.Unmarshal(d, conf); err != nil {
		panic(err)
	}
	if err := conf.Init(); err != nil {
		panic(err)
	}
}
