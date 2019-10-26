package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

//2级证书
type Cert struct {
	URL    VarStr      //使用域名 https://api.xginx.com
	PubKey PKBytes     //证书公钥
	Expire int64       //过期时间:unix 秒
	Prev   PKBytes     //证书上级公钥
	CSig   SigBytes    //根证书签名
	vsig   bool        //是否验证了签名
	priv   *PrivateKey //加载后用
	pubv   *PublicKey  //加载后用
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
	c.Prev.Set(pri.PublicKey())
	buf := &bytes.Buffer{}
	if err := c.Encode(buf); err != nil {
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
	if err := binary.Read(r, Endian, &c.Prev); err != nil {
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
	if err := c.Encode(buf); err != nil {
		return err
	}
	sig, err := NewSigValue(c.CSig[:])
	if err != nil {
		return err
	}
	//获取我信任的证书校验证书数据
	pub := conf.GetPublicKey(c.Prev)
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
	if cert.pubv != nil && cert.priv != nil && !pri.PublicKey().Equal(cert.PubKey[:]) {
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
	if err := binary.Write(buf, Endian, c.CSig); err != nil {
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
	if err := c.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *Cert) Encode(w io.Writer) error {
	if err := c.URL.EncodeWriter(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.PubKey[:]); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.Expire); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.Prev); err != nil {
		return err
	}
	return nil
}
