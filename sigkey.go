package xginx

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"math/big"
)

//公钥定义
const (
	PublicKeySize  = 33
	P256PubKeyEven = byte(0x02)
	P256PubKeyOdd  = byte(0x03)
)

//算法
var (
	curve           = SECP256K1()
	PrefixSecretKey = []byte{128}
)

//PKBytes 公钥字节存储
type PKBytes [33]byte

//Bytes 获取字节
func (v PKBytes) Bytes() []byte {
	return v[:]
}

//Hash 计算 hash
func (v PKBytes) Hash() HASH160 {
	return Hash160From(v[:])
}

//Cmp 转为大数比较
func (v PKBytes) Cmp(b PKBytes) int {
	vu := NewUINT256(v[:])
	bu := NewUINT256(b[:])
	return vu.Cmp(bu)
}

//Equal ==
func (v PKBytes) Equal(b PKBytes) bool {
	return bytes.Equal(v[:], b[:])
}

//Encode 编码
func (v PKBytes) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

//Decode 解码数据
func (v *PKBytes) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

//SetBytes  使用二进制初始化数据
func (v *PKBytes) SetBytes(b []byte) {
	copy(v[:], b)
}

//Set 使用公钥初始化
func (v *PKBytes) Set(pk *PublicKey) PKBytes {
	copy(v[:], pk.Encode())
	return *v
}

//SigBytes 签名数据
type SigBytes [75]byte

//Bytes 获取二进制
func (v SigBytes) Bytes() []byte {
	return v[:]
}

//Encode 编码数据
func (v SigBytes) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

//Decode 解码签名数据
func (v *SigBytes) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

//SetBytes 使用二进制初始化
func (v *SigBytes) SetBytes(b []byte) {
	copy(v[:], b)
}

//Set 使用签名初始化
func (v *SigBytes) Set(sig *SigValue) {
	copy(v[:], sig.Encode())
}

//PrivateKey 私钥
type PrivateKey struct {
	D *big.Int
}

//New 当前私钥基础上hash新的私钥
func (pk PrivateKey) New(plus []byte) *PrivateKey {
	b := pk.D.Bytes()
	plus = append(plus, b...)
	b = Hash256(plus)
	pkv := &PrivateKey{}
	pkv.D = new(big.Int).SetBytes(b)
	return pkv
}

//Clone 复制私钥
func (pk *PrivateKey) Clone() *PrivateKey {
	np := &PrivateKey{}
	np.D = new(big.Int).SetBytes(pk.D.Bytes())
	return np
}

//LoadPrivateKey 加载私钥
func LoadPrivateKey(s string, pass ...string) (*PrivateKey, error) {
	key := &PrivateKey{}
	err := key.Load(s, pass...)
	return key, err
}

//Decode 解码私钥
func (pk *PrivateKey) Decode(data []byte) error {
	if len(data) < 4 {
		return errors.New("size error")
	}
	dl := len(data)
	hv := Hash256(data[:dl-4])
	if bytes.Equal(hv[:4], data[dl-4:]) {
		data = data[:dl-4]
	}
	dl = len(data)
	pl := len(PrefixSecretKey)
	if (dl == pl+32 || (dl == pl+33 && data[dl-1] == 1)) && bytes.Equal(PrefixSecretKey, data[:pl]) {
		pk.SetBytes(data[pl : dl-1])
	}
	return nil
}

//Bytes 获取二进制数据
func (pk *PrivateKey) Bytes() []byte {
	return pk.D.Bytes()
}

//Encode 编码数据
func (pk *PrivateKey) Encode() []byte {
	pb := pk.D.Bytes()
	buf := NewWriter()
	err := buf.TWrite(PrefixSecretKey)
	if err != nil {
		panic(err)
	}
	err = buf.TWrite(pb)
	if err != nil {
		panic(err)
	}
	err = buf.WriteByte(1)
	if err != nil {
		panic(err)
	}
	hv := Hash256(buf.Bytes())
	err = buf.TWrite(hv[:4])
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

//Dump 导出私钥
func (pk *PrivateKey) Dump(pass ...string) (string, error) {
	bb := pk.Encode()
	return HashDump(bb, pass...)
}

//Load 加载私钥
func (pk *PrivateKey) Load(s string, pass ...string) error {
	data, err := HashLoad(s, pass...)
	if err != nil {
		return err
	}
	return pk.Decode(data)
}

//IsValid 是否有效
func (pk *PrivateKey) IsValid() bool {
	return pk.PublicKey().IsValid()
}

//SetBytes 二进制初始化
func (pk *PrivateKey) SetBytes(b []byte) *PrivateKey {
	pk.D = new(big.Int).SetBytes(b)
	return pk
}

func (pk PrivateKey) String() string {
	return hex.EncodeToString(pk.D.Bytes())
}

//NewPrivateKeyWithBytes 使用二进制创建私钥
func NewPrivateKeyWithBytes(b []byte) (*PrivateKey, error) {
	params := curve.Params()
	k := new(big.Int).SetBytes(b)
	n := new(big.Int).Sub(params.N, one)
	k.Mod(k, n)
	k.Add(k, one)
	return &PrivateKey{D: k}, nil
}

//GenPrivateKey 自动生成私钥
func GenPrivateKey() (k *big.Int, err error) {
	params := curve.Params()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	bb, err := x509.MarshalPKCS8PrivateKey(pk)
	if err != nil {
		return nil, err
	}
	hb := Hash256(bb)
	k = new(big.Int).SetBytes(hb)
	n := new(big.Int).Sub(params.N, one)
	k.Mod(k, n)
	k.Add(k, one)
	return
}

//NewPrivateKey 创建私钥
func NewPrivateKey() (*PrivateKey, error) {
	d, err := GenPrivateKey()
	if err != nil {
		return nil, err
	}
	return &PrivateKey{D: d}, nil
}

//Sign 签名hash256数据
func (pk PrivateKey) Sign(hash []byte) (*SigValue, error) {
	sig := &SigValue{}
	priv := new(ecdsa.PrivateKey)
	priv.Curve = curve
	priv.D = pk.D
	pub := pk.PublicKey()
	priv.X, priv.Y = pub.X, pub.Y
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash)
	if err != nil {
		return nil, err
	}
	sig.R, sig.S = r, s
	return sig, nil
}

//Marshal 编码数据
func (pk PrivateKey) Marshal() []byte {
	return pk.D.Bytes()
}

//PublicKey 获取私钥对应的公钥
func (pk *PrivateKey) PublicKey() *PublicKey {
	pub := &PublicKey{}
	pub.X, pub.Y = curve.ScalarBaseMult(pk.Marshal())
	return pub
}

//SigValue 签名
type SigValue struct {
	R *big.Int
	S *big.Int
}

//GetSigs 导出签名数据
func (sig *SigValue) GetSigs() SigBytes {
	sb := SigBytes{}
	sb.Set(sig)
	return sb
}

//NewSigValue 从二进制创建签名
func NewSigValue(b []byte) (*SigValue, error) {
	sig := &SigValue{}
	err := sig.Decode(b)
	return sig, err
}

//FromHEX 从hex格式创建
func (sig *SigValue) FromHEX(s string) error {
	data, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	return sig.Decode(data)
}

//Encode 编码签名
func (sig SigValue) Encode() []byte {
	r := sig.R.Bytes()
	if r[0] >= 0x80 {
		r = append([]byte{0}, r...)
	}
	s := sig.S.Bytes()
	if s[0] >= 0x80 {
		s = append([]byte{0}, s...)
	}
	res := NewWriter()
	err := res.WriteByte(0x30)
	if err != nil {
		panic(err)
	}
	err = res.WriteByte(byte(4 + len(r) + len(s)))
	if err != nil {
		panic(err)
	}
	err = res.WriteByte(0x02)
	if err != nil {
		panic(err)
	}
	err = res.WriteByte(byte(len(r)))
	if err != nil {
		panic(err)
	}
	err = res.WriteFull(r)
	if err != nil {
		panic(err)
	}
	err = res.WriteByte(0x02)
	if err != nil {
		panic(err)
	}
	err = res.WriteByte(byte(len(s)))
	if err != nil {
		panic(err)
	}
	err = res.WriteFull(s)
	if err != nil {
		panic(err)
	}
	return res.Bytes()
}

//CheckLow 检测签名数据
func (sig *SigValue) CheckLow(b []byte) (int, int, error) {
	if b[0] != 0x30 || len(b) < 5 {
		return 0, 0, errors.New("der format error")
	}
	lenr := int(b[3])
	if lenr == 0 || 5+lenr >= len(b) || b[lenr+4] != 0x02 {
		return 0, 0, errors.New("der length error")
	}
	lens := int(b[lenr+5])
	if lens == 0 || int(b[1]) != lenr+lens+4 || lenr+lens+6 > len(b) || b[2] != 0x02 {
		return 0, 0, errors.New("der length error")
	}
	return lenr, lens, nil
}

//Decode 解码签名
func (sig *SigValue) Decode(b []byte) error {
	r, s, err := sig.CheckLow(b)
	if err != nil {
		return err
	}
	sig.R = new(big.Int).SetBytes(b[4 : 4+r])
	sig.S = new(big.Int).SetBytes(b[6+r : 6+r+s])
	return nil
}

//PublicKey 公钥
type PublicKey struct {
	X  *big.Int
	Y  *big.Int
	b0 byte
}

//NewPublicKey 从二进制创建公钥
func NewPublicKey(data []byte) (*PublicKey, error) {
	pk := &PublicKey{}
	err := pk.Decode(data)
	return pk, err
}

//Equal ==
func (pk *PublicKey) Equal(sb []byte) bool {
	pb := pk.Encode()
	return bytes.Equal(pb, sb)
}

//FromHEX 从16进制格式创建私钥
func (pk *PublicKey) FromHEX(s string) error {
	data, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	return pk.Decode(data)
}

//DecompressY 为压缩公钥计算y
// y^2 = x^3 + b
// y   = sqrt(x^3 + b)
func DecompressY(x *big.Int, ybit uint) *big.Int {
	c := curve.Params()
	var y, x3b big.Int
	x3b.Mul(x, x)
	x3b.Mul(&x3b, x)
	x3b.Add(&x3b, c.B)
	x3b.Mod(&x3b, c.P)
	y.ModSqrt(&x3b, c.P)
	if y.Bit(0) != ybit {
		y.Sub(c.P, &y)
	}
	return &y
}

// 标准算法计算y
// y^2 = x^3 -3x + b
// y = sqrt(x^3 -3x + b)
//func DecompressY(x *big.Int, ybit uint) *big.Int {
//	c := curve.Params()
//	var y, x3b, x3 big.Int
//	x3.SetInt64(3)
//	x3.Mul(&x3, x)
//	x3b.Mul(x, x)
//	x3b.Mul(&x3b, x)
//	x3b.Add(&x3b, c.B)
//	x3b.Sub(&x3b, &x3)
//	x3b.Mod(&x3b, c.P)
//	y.ModSqrt(&x3b, c.P)
//	if y.Bit(0) != ybit {
//		y.Sub(c.P, &y)
//	}
//	return &y
//}

//Decode 解码公钥
func (pk *PublicKey) Decode(data []byte) error {
	byteLen := (curve.Params().BitSize + 7) >> 3
	if len(data) == 0 {
		return errors.New("data empty")
	}
	pk.b0 = data[0]
	if len(data) != PublicKeySize {
		return errors.New("data size error")
	}
	if pk.b0 != P256PubKeyEven && pk.b0 != P256PubKeyOdd {
		return errors.New(" compressed head byte error")
	}
	p := curve.Params().P
	x := new(big.Int).SetBytes(data[1 : 1+byteLen])
	ybit := uint(0)
	if pk.b0 == P256PubKeyOdd {
		ybit = 1
	}
	y := DecompressY(x, ybit)
	d := byte(y.Bit(0))
	if pk.b0 == P256PubKeyOdd && d != 1 {
		return errors.New("decompress public key odd error")
	}
	if pk.b0 == P256PubKeyEven && d != 0 {
		return errors.New("decompress public key even error")
	}
	if x.Cmp(p) >= 0 || y.Cmp(p) >= 0 {
		return errors.New("decompress x,y error")
	}
	if !curve.IsOnCurve(x, y) {
		return errors.New("cpmpressed x,y not at curve error")
	}
	pk.X, pk.Y = x, y
	return nil
}

//Hash hash公钥
func (pk *PublicKey) Hash() HASH160 {
	b := pk.Encode()
	return Hash160From(b)
}

//IsValid 检查公钥是否有效
func (pk *PublicKey) IsValid() bool {
	return curve.IsOnCurve(pk.X, pk.Y)
}

//Verify 验证hash签名
func (pk *PublicKey) Verify(hash []byte, sig *SigValue) bool {
	pub := new(ecdsa.PublicKey)
	pub.Curve = curve
	pub.X, pub.Y = pk.X, pk.Y
	return ecdsa.Verify(pub, hash, sig.R, sig.S)
}

//LoadPublicKey 加载公钥数据
func LoadPublicKey(s string, pass ...string) (*PublicKey, error) {
	return new(PublicKey).Load(s, pass...)
}

//Address 账号地址
type Address string

const (
	//EmptyAddress 空地址定义
	EmptyAddress Address = ""
)

//NewTxOut 创建一个输出
func (a Address) NewTxOut(v Amount, meta string, execs ...[]byte) (*TxOut, error) {
	if !v.IsRange() {
		return nil, errors.New("amount error")
	}
	out := &TxOut{}
	out.Value = v
	pkh, err := a.GetPkh()
	if err != nil {
		return nil, err
	}
	lcks, err := NewLockedScript(pkh, meta, execs...)
	if err != nil {
		return nil, err
	}
	script, err := lcks.ToScript()
	if err != nil {
		return nil, err
	}
	out.Script = script
	return out, nil
}

//Check 检测地址是否正确
func (a Address) Check() error {
	_, err := DecodeAddress(a)
	return err
}

//GetPkh 获取公钥hash
func (a Address) GetPkh() (HASH160, error) {
	return DecodeAddress(a)
}

//EncodeAddressWithPrefix 编码地址用指定前缀
func EncodeAddressWithPrefix(prefix string, pkh HASH160) (string, error) {
	ver := byte(0)
	b := []byte{ver, byte(len(pkh))}
	b = append(b, pkh[:]...)
	addr, err := SegWitAddressEncode(prefix, b)
	if err != nil {
		return "", err
	}
	return addr, nil
}

//EncodeAddress 编码地址
func EncodeAddress(pkh HASH160) (Address, error) {
	st := "st"
	if conf != nil {
		st = conf.AddrPrefix
	}
	a, err := EncodeAddressWithPrefix(st, pkh)
	return Address(a), err
}

//DecodeAddress 解码地址
func DecodeAddress(addr Address) (HASH160, error) {
	st := "st"
	if conf != nil {
		st = conf.AddrPrefix
	}
	hv := HASH160{}
	hrp, b, err := SegWitAddressDecode(string(addr))
	if err != nil {
		return hv, err
	}
	if hrp != st {
		return hv, errors.New("address prefix error")
	}
	if b[0] != 0 {
		return hv, errors.New("ver error")
	}
	if int(b[1]) != len(b[2:]) {
		return hv, errors.New("address length error")
	}
	copy(hv[:], b[2:])
	return hv, nil
}

//Load 加载公钥
func (pk *PublicKey) Load(s string, pass ...string) (*PublicKey, error) {
	b, err := HashLoad(s, pass...)
	if err != nil {
		return nil, err
	}
	return pk, pk.Decode(b)
}

//GetPks 获取公钥数据
func (pk *PublicKey) GetPks() PKBytes {
	pks := PKBytes{}
	return pks.Set(pk)
}

//Dump 导出公钥
func (pk *PublicKey) Dump(pass ...string) (string, error) {
	b := pk.Encode()
	return HashDump(b, pass...)
}

//Encode 编码公钥
func (pk *PublicKey) Encode() []byte {
	ret := []byte{}
	d := byte(pk.Y.Bit(0))
	ret = append(ret, P256PubKeyEven+d)
	ret = append(ret, pk.X.Bytes()...)
	return ret
}
