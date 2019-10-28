package xginx

import (
	"log"
	"testing"
	"time"
)

func TestConfigEnDeCert(t *testing.T) {
	b, err := conf.EncodeCerts()
	if err != nil {
		panic(err)
	}
	err = conf.DecodeCerts(b)
	if err != nil {
		panic(err)
	}
}

//签发证书测试
func TestCert(t *testing.T) {
	//获取证书签名私钥
	pris := LoadPrivateKeys("pris.json")
	//生成待签名证书
	cpk, err := NewPrivateKey()
	if err != nil {
		panic(err)
	}
	log.Println(cpk.Dump())
	cpp := cpk.PublicKey()
	//有效期1年
	cert := NewCert(cpp, "api.cai4.cn", time.Hour*24*365)
	//私钥导出，只有做签名的节点才能导出私钥
	//cert.SetKey(cpk)
	if err := cert.Sign(pris[0]); err != nil {
		panic(err)
	}
	s, err := cert.Dump()
	if err != nil {
		t.Error(err)
	}
	_, err = new(Cert).Load(s)
	if err != nil {
		t.Error(err)
	}
	log.Println(s)
}
