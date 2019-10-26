package xginx

import (
	"encoding/json"
	"log"
	"testing"
	"time"
)

func TestVerityCert(t *testing.T) {
	s := "4Dv1YyqFAHpKqWwdnENcugG4dG3P3WhJyY6oQxEPFjSiFuFRuVhziZMkqfJLAkCybqaBUTBkurBxyWZMSNYvNuSykfCo52PHkc7GdNgRk4m1HmwtKSUian5truJCKWSEzJBsdsZ7A11KCX8ek39AnGaW3Zic55jX8ECnBbTL9wyc7gky3WWCdKZKezLKAuS9qH"
	cert, err := new(Cert).Load(s)
	if err != nil {
		panic(err)
	}
	log.Println(cert.Verify(), cert.ExpireTime())
}

//签发证书测试
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
