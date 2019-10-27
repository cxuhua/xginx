package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"
)

//证书池
type CertPool struct {
	mu    sync.Mutex
	certs map[PKBytes]*Cert
}

func (cp *CertPool) Set(cert *Cert) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.certs[cert.PubKey] = cert
}

//指定公钥校验签名
func (cp *CertPool) Verify(pk PKBytes, sig *SigValue, msg []byte) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cert, ok := cp.certs[pk]
	if !ok {
		return errors.New("miss public key")
	}
	if cert.IsExpire() {
		return errors.New("cert expire")
	}
	if err := cert.Verify(); err != nil {
		return err
	}
	if !cert.pubv.Verify(msg, sig) {
		return errors.New("verify error")
	}
	return nil
}

//获取一个可用于签名的私钥
func (cp *CertPool) SignCert() (*Cert, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	for _, v := range cp.certs {
		if v.priv == nil {
			continue
		}
		if v.IsExpire() {
			continue
		}
		if err := v.Verify(); err != nil {
			continue
		}
		return v, nil
	}
	return nil, errors.New("sign cert miss")
}

func NewCertPool() *CertPool {
	cp := &CertPool{}
	cp.certs = map[PKBytes]*Cert{}
	return cp
}

type Cert struct {
	URL    VarStr      //使用域名 api.xginx.com
	PubKey PKBytes     //证书公钥
	Expire int64       //过期时间:unix 秒
	VPub   PKBytes     //验证公钥，对应config中信任的公钥,必须在config信任列表中
	CSig   SigBytes    //公钥签名，信任的公钥检测签名，通过说明证书有效，如果不过期
	vsig   bool        //是否验证了签名
	priv   *PrivateKey //如果有可用来签名数据
	pubv   *PublicKey  //如果有可用来验证签名
}

func (c *Cert) IsExpire() bool {
	return time.Now().Unix() > c.Expire
}

func (c *Cert) Clone() (*Cert, error) {
	if c.IsExpire() {
		return nil, errors.New("cert expire")
	}
	nc := &Cert{}
	nc.URL = c.URL
	nc.PubKey = c.PubKey
	nc.Expire = c.Expire
	nc.VPub = c.VPub
	nc.CSig = c.CSig
	pub, err := NewPublicKey(nc.PubKey[:])
	if err != nil {
		return nil, err
	}
	nc.pubv = pub
	if c.priv != nil {
		nc.priv = c.priv.Clone()
	}
	return nc, nil
}

func NewCert(pub *PublicKey, url string, exp time.Duration) *Cert {
	c := &Cert{}
	c.URL = VarStr(url)
	c.PubKey.Set(pub)
	c.Expire = time.Now().Add(exp).Unix()
	return c
}

//开始用idx私钥来签发证书
func (c *Cert) Sign(idx uint16) error {
	if len(c.URL) == 0 {
		return errors.New("url miss")
	}
	if c.Expire < time.Now().Unix() {
		return errors.New("expire time error")
	}
	pri := conf.GetPrivateKey(idx)
	//设置上级证书公钥
	c.VPub.Set(pri.PublicKey())
	buf := &bytes.Buffer{}
	if err := c.EncodeWriter(buf); err != nil {
		return err
	}
	hash := HASH256(buf.Bytes())
	sig, err := pri.Sign(hash)
	if err != nil {
		return err
	}
	c.CSig.Set(sig)
	return nil
}

func (c *Cert) Decode(r io.Reader) error {
	if err := c.URL.DecodeReader(r); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.PubKey); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.Expire); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.VPub); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.CSig); err != nil {
		return err
	}
	return nil
}

func (c *Cert) ExpireTime() int64 {
	t := c.Expire - time.Now().Unix()
	if t < 0 {
		t = 0
	}
	return t
}

//验证证书是否正确
func (c *Cert) Verify() error {
	//如果已经验证了签名就只验证过期时间
	if c.vsig && time.Now().Unix() > c.Expire {
		return errors.New("cert expire")
	}
	buf := &bytes.Buffer{}
	if err := c.EncodeWriter(buf); err != nil {
		return err
	}
	sig, err := NewSigValue(c.CSig[:])
	if err != nil {
		return err
	}
	//获取我信任的证书校验证书数据
	pub := conf.GetPublicKey(c.VPub)
	if pub == nil {
		return errors.New("public untrusted")
	}
	hash := HASH256(buf.Bytes())
	if !pub.Verify(hash, sig) {
		return errors.New("verify cert failed")
	}
	if time.Now().Unix() > c.Expire {
		return errors.New("cert expire")
	}
	c.vsig = true
	return nil
}

//获取对应的私钥
func (c *Cert) PrivateKey() *PrivateKey {
	return c.priv
}

//获取对应的公钥
func (c *Cert) PublicKey() *PublicKey {
	return c.pubv
}

func LoadCert(ss string, ps string) (*Cert, error) {
	cert, err := new(Cert).Load(ps)
	if err != nil {
		return nil, err
	}
	cert.pubv, err = NewPublicKey(cert.PubKey[:])
	if err != nil {
		return nil, err
	}
	pri, err := LoadPrivateKey(ss)
	if err == nil {
		cert.priv = pri
	}
	if cert.pubv != nil && cert.priv != nil && !cert.priv.PublicKey().Equal(cert.PubKey[:]) {
		return nil, errors.New("public private map error")
	}
	if err := cert.Verify(); err != nil {
		return nil, err
	}
	return cert, nil
}

//加载证书
func (c *Cert) Load(s string) (*Cert, error) {
	b, err := B58Decode(s, BitcoinAlphabet)
	if err != nil {
		return nil, err
	}
	l := len(b)
	hv := HASH256(b[:l-4])
	if !bytes.Equal(b[l-4:], hv[:4]) {
		return nil, errors.New("check sum error")
	}
	buf := bytes.NewReader(b[:l-4])
	return c, c.Decode(buf)
}

//导出证书
func (c *Cert) Dump() (string, error) {
	buf := &bytes.Buffer{}
	if err := c.Encode(buf); err != nil {
		return "", err
	}
	b := buf.Bytes()
	hv := HASH256(b)
	b = append(b, hv[:4]...)
	s := B58Encode(b, BitcoinAlphabet)
	return s, nil
}

func (c *Cert) ToSigBinary() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := c.EncodeWriter(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *Cert) Encode(w io.Writer) error {
	if err := c.EncodeWriter(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.CSig); err != nil {
		return err
	}
	return nil
}

func (c *Cert) EncodeWriter(w io.Writer) error {
	if err := c.URL.EncodeWriter(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.PubKey[:]); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.Expire); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.VPub); err != nil {
		return err
	}
	return nil
}
