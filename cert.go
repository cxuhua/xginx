package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

type Cert struct {
	Name   VarStr   //证书名称
	PubKey PKBytes  //证书公钥
	Expire int64    //过期时间:unix 秒,过期后可以验证旧数据，但无法签名新数据
	CSig   SigBytes //签名，信任的公钥检测签名，通过说明证书有效，如果不过期
	VPub   PKBytes  //验证公钥，对应config中信任的公钥,必须在config信任列表中
	vsig   bool     //是否验证了签名
	pubv   *PublicKey
}

func NewCert(pub *PublicKey, name string, exp time.Duration) *Cert {
	c := &Cert{}
	c.Name = VarStr(name)
	c.PubKey.Set(pub)
	c.Expire = time.Now().Add(exp).Unix()
	return c
}

//开始用pri私钥来签发新证书
func (c *Cert) Sign(pri *PrivateKey) error {
	if len(c.Name) == 0 {
		return errors.New("name miss")
	}
	if c.Expire < time.Now().Unix() {
		return errors.New("expire time error")
	}
	//设置证书验证公钥
	c.VPub.Set(pri.PublicKey())
	buf := &bytes.Buffer{}
	if err := c.EncodeWriter(buf); err != nil {
		return err
	}
	hash := Hash256(buf.Bytes())
	sig, err := pri.Sign(hash)
	if err != nil {
		return err
	}
	c.CSig.Set(sig)
	return nil
}

func (c *Cert) Decode(r io.Reader) error {
	if err := c.Name.DecodeReader(r); err != nil {
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

//验证证书是否签名正确
func (c *Cert) Verify() error {
	//如果已经验证了签名就只验证过期时间
	if c.vsig {
		return nil
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
		return errors.New("public key untrusted")
	}
	hash := Hash256(buf.Bytes())
	if !pub.Verify(hash, sig) {
		return errors.New("verify cert failed")
	}
	c.vsig = true
	return nil
}

//获取对应的公钥
func (c *Cert) PublicKey() *PublicKey {
	return c.pubv
}

func LoadCert(ss string) (*Cert, error) {
	return new(Cert).Load(ss)
}

//加载证书
func (c *Cert) Load(s string) (*Cert, error) {
	b, err := B58Decode(s, BitcoinAlphabet)
	if err != nil {
		return nil, err
	}
	l := len(b)
	hv := Hash256(b[:l-4])
	if !bytes.Equal(b[l-4:], hv[:4]) {
		return nil, errors.New("check sum error")
	}
	buf := bytes.NewReader(b[:l-4])
	if err := c.Decode(buf); err != nil {
		return nil, err
	}
	c.pubv, err = NewPublicKey(c.PubKey[:])
	if err != nil {
		return nil, err
	}
	if err := c.Verify(); err != nil {
		return nil, err
	}
	return c, nil
}

//导出证书
func (c *Cert) Dump() (string, error) {
	buf := &bytes.Buffer{}
	if err := c.Encode(buf); err != nil {
		return "", err
	}
	b := buf.Bytes()
	hv := Hash256(b)
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
	if err := c.Name.EncodeWriter(w); err != nil {
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
