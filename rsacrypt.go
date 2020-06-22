package xginx

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

const (
	//RSABits 默认rsa加密强度
	RSABits = 2048
	//RSAPrivateType 私钥类型描述
	RSAPrivateType = "RSA PRIVATE KEY"
	//RSAPublicType 公钥类型描述
	RSAPublicType = "RSA PUBLIC KEY"
)

//RSAPublicKey rsa公钥
type RSAPublicKey struct {
	pp *rsa.PublicKey
}

//LoadRSAPublicKey 从树加载一个私钥
func LoadRSAPublicKey(str string) (*RSAPublicKey, error) {
	pp := &RSAPublicKey{}
	err := pp.Load(str)
	return pp, err
}

//Load 加载公钥
func (ptr *RSAPublicKey) Load(str string) error {
	bb, err := HashLoad(str)
	if err != nil {
		return err
	}
	pb, _ := pem.Decode(bb)
	if pb == nil {
		return fmt.Errorf("pem decode error")
	}
	pp, err := x509.ParsePKCS1PublicKey(pb.Bytes)
	if err != nil {
		return err
	}
	ptr.pp = pp
	return nil
}

//Verify 公钥验签
func (ptr RSAPublicKey) Verify(src []byte, sign []byte) error {
	h := sha256.New()
	h.Write(src)
	hashed := h.Sum(nil)
	return rsa.VerifyPKCS1v15(ptr.pp, crypto.SHA256, hashed, sign)
}

//Encrypt 公钥加密
func (ptr RSAPublicKey) Encrypt(bb []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, ptr.pp, bb)
}

//Dump 导出公钥
func (ptr RSAPublicKey) Dump() (string, error) {
	bb := x509.MarshalPKCS1PublicKey(ptr.pp)
	pb := &pem.Block{
		Type:  RSAPublicType,
		Bytes: bb,
	}
	buf := NewWriter()
	err := pem.Encode(buf, pb)
	if err != nil {
		return "", err
	}
	return HashDump(buf.Bytes())
}

//RSAPrivateKey rsa私钥
type RSAPrivateKey struct {
	pk *rsa.PrivateKey
}

//LoadRSAPrivateKey 从树加载一个私钥
func LoadRSAPrivateKey(str string, pass ...string) (*RSAPrivateKey, error) {
	pk := &RSAPrivateKey{}
	err := pk.Load(str, pass...)
	return pk, err
}

//Load 加载rsa私钥
func (ptr *RSAPrivateKey) Load(str string, pass ...string) error {
	bb, err := HashLoad(str, pass...)
	if err != nil {
		return err
	}
	pb, _ := pem.Decode(bb)
	if pb == nil {
		return fmt.Errorf("pem decode error")
	}
	pk, err := x509.ParsePKCS1PrivateKey([]byte(pb.Bytes))
	if err != nil {
		return err
	}
	err = pk.Validate()
	if err != nil {
		return err
	}
	ptr.pk = pk
	return nil
}

//Sign 私钥签名
func (ptr RSAPrivateKey) Sign(src []byte) ([]byte, error) {
	h := sha256.New()
	h.Write(src)
	hashed := h.Sum(nil)
	return rsa.SignPKCS1v15(rand.Reader, ptr.pk, crypto.SHA256, hashed)
}

//Decrypt 私钥解密
func (ptr RSAPrivateKey) Decrypt(bb []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, ptr.pk, bb)
}

//PublicKey 获取公钥
func (ptr RSAPrivateKey) PublicKey() *RSAPublicKey {
	return &RSAPublicKey{pp: &ptr.pk.PublicKey}
}

//Dump 导出密钥
func (ptr RSAPrivateKey) Dump(pass ...string) (string, error) {
	bb := x509.MarshalPKCS1PrivateKey(ptr.pk)
	pb := &pem.Block{
		Type:  RSAPrivateType,
		Bytes: bb,
	}
	buf := NewWriter()
	err := pem.Encode(buf, pb)
	if err != nil {
		return "", err
	}
	return HashDump(buf.Bytes(), pass...)
}

//NewRSAPrivateKey 创建一个rsa密钥对
func NewRSAPrivateKey() (*RSAPrivateKey, error) {
	ret := &RSAPrivateKey{}
	pk, err := rsa.GenerateKey(rand.Reader, RSABits)
	if err != nil {
		return nil, err
	}
	ret.pk = pk
	return ret, nil
}
