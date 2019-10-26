package xginx

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"
)

func TestGenPrivateKey(t *testing.T) {

	//生成主节点私钥和公钥
	//keys/privates.json
	//keys/publics.json
	publics := []string{}
	privates := []string{}
	for i := 0; i < 16; i++ {
		cpk, err := NewPrivateKey()
		if err != nil {
			panic(err)
		}
		privates = append(privates, cpk.Dump())
		cpb := cpk.PublicKey()
		publics = append(publics, cpb.Dump())
	}
	conf := Config{
		Privates: privates,
		Publics:  publics,
	}
	d, err := json.Marshal(conf)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile("test.json", d, os.ModePerm)
}

func TestVerityCert(t *testing.T) {
	s := "4Dv1YyqFAHpKqWwdnENcugG4dG3P3WhJyY6oQxEPFjSiFuFRuVhziZMkqfJLAkCybqaBUTBkurBxyWZMSNYvNuSykfCo52PHkc7GdNgRk4m1HmwtKSUian5truJCKWSEzJBsdsZ7A11KCX8ek39AnGaW3Zic55jX8ECnBbTL9wyc7gky3WWCdKZKezLKAuS9qH"
	cert, err := new(Cert).Load(s)
	if err != nil {
		panic(err)
	}
	log.Println(cert.Verify(), cert.ExpireTime())
}

func TestCert(t *testing.T) {
	//生成待签名证书
	cpk, err := NewPrivateKey()
	if err != nil {
		panic(err)
	}
	//导出私钥
	//导出已签名证书
	//获取待签名证书公钥
	cpp := cpk.PublicKey()
	//有效期1年
	cert := NewCert(cpp, "https://api.cai4.cn", time.Hour*24*365)
	if err := cert.Sign(5); err != nil {
		panic(err)
	}
	item := CertItem{}
	item.Priv = cpk.Dump()
	item.Cert, _ = cert.Dump()
	dd, _ := json.Marshal(item)
	log.Println(string(dd))
}
