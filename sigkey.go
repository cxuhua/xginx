package xginx

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
)

const (
	PUBLIC_KEY_SIZE  = 33
	P256_PUBKEY_EVEN = byte(0x02)
	P256_PUBKEY_ODD  = byte(0x03)
)

//确定性私钥地址
type DeterKey struct {
	Root []byte `bson:"root"` //种子私钥
	Key  []byte `bson:"key"`  //私钥编码
}

//加载key
func LoadDeterKey(s string) (*DeterKey, error) {
	data, err := B58Decode(s, BitcoinAlphabet)
	if err != nil {
		return nil, err
	}
	if len(data) != 68 {
		return nil, errors.New("data length error")
	}
	dl := len(data)
	hbytes := Hash256(data[:dl-4])
	if !bytes.Equal(hbytes[:4], data[dl-4:]) {
		return nil, errors.New("checksum error")
	}
	dk := &DeterKey{
		Root: data[:32],
		Key:  data[32 : dl-4],
	}
	return dk, nil
}

func (k DeterKey) Dump() string {
	data := append([]byte{}, k.Root...)
	data = append(data, k.Key...)
	hbytes := Hash256(data)
	data = append(data, hbytes[:4]...)
	return B58Encode(data, BitcoinAlphabet)
}

func (k DeterKey) String() string {
	return fmt.Sprintf("%s %s", hex.EncodeToString(k.Root), hex.EncodeToString(k.Key))
}

//派生一个密钥
func (k DeterKey) New(idx uint32) *DeterKey {
	h := hmac.New(func() hash.Hash {
		return sha512.New()
	}, k.Key)
	_, err := h.Write(k.Root)
	if err != nil {
		panic(err)
	}
	err = binary.Write(h, binary.BigEndian, idx)
	if err != nil {
		panic(err)
	}
	b := h.Sum(nil)
	if len(b) != 64 {
		panic(errors.New("hmac sha512 sum error"))
	}
	return &DeterKey{
		Root: b[:32],
		Key:  b[32:],
	}
}

func NewDeterKey() *DeterKey {
	pri, err := NewPrivateKey()
	if err != nil {
		panic(err)
	}
	k := &DeterKey{}
	k.Root = pri.Bytes()
	k.Key = make([]byte, 32)
	_, err = rand.Read(k.Key)
	if err != nil {
		panic(err)
	}
	return k
}

var (
	curve             = SECP256K1()
	PREFIX_SECRET_KEY = []byte{128}
)

type PKBytes [33]byte

func (v PKBytes) Bytes() []byte {
	return v[:]
}

func (v PKBytes) Hash() HASH160 {
	return Hash160From(v[:])
}

func (v PKBytes) Cmp(b PKBytes) int {
	vu := NewUINT256(v[:])
	bu := NewUINT256(b[:])
	return vu.Cmp(bu)
}

func (v PKBytes) Equal(b PKBytes) bool {
	return bytes.Equal(v[:], b[:])
}

func (v PKBytes) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

func (v *PKBytes) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

func (p *PKBytes) SetBytes(b []byte) {
	copy(p[:], b)
}

func (p *PKBytes) Set(pk *PublicKey) PKBytes {
	copy(p[:], pk.Encode())
	return *p
}

type SigBytes [75]byte

func (v SigBytes) Bytes() []byte {
	return v[:]
}

func (v SigBytes) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

func (v *SigBytes) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

func (p *SigBytes) SetBytes(b []byte) {
	copy(p[:], b)
}

func (p *SigBytes) Set(sig *SigValue) {
	copy(p[:], sig.Encode())
}

type PrivateKey struct {
	D *big.Int
}

//当前私钥基础上hash新的私钥
func (p PrivateKey) New(plus []byte) *PrivateKey {
	b := p.D.Bytes()
	plus = append(plus, b...)
	b = Hash256(plus)
	pk := &PrivateKey{}
	pk.D = new(big.Int).SetBytes(b)
	return pk
}

func (p *PrivateKey) Clone() *PrivateKey {
	np := &PrivateKey{}
	np.D = new(big.Int).SetBytes(p.D.Bytes())
	return np
}

//prefix[1] key[32] checknum[HASH256-prefix-4]
func LoadPrivateKey(s string) (*PrivateKey, error) {
	key := &PrivateKey{}
	err := key.Load(s)
	return key, err
}

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
	pl := len(PREFIX_SECRET_KEY)
	if (dl == pl+32 || (dl == pl+33 && data[dl-1] == 1)) && bytes.Equal(PREFIX_SECRET_KEY, data[:pl]) {
		pk.SetBytes(data[pl : dl-1])
	}
	return nil
}

func (pk *PrivateKey) Bytes() []byte {
	return pk.D.Bytes()
}

func (pk *PrivateKey) Encode() []byte {
	pb := pk.D.Bytes()
	buf := NewWriter()
	err := buf.TWrite(PREFIX_SECRET_KEY)
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

func (pk *PrivateKey) Dump() string {
	bb := pk.Encode()
	return B58Encode(bb, BitcoinAlphabet)
}

func (pk *PrivateKey) Load(s string) error {
	data, err := B58Decode(s, BitcoinAlphabet)
	if err != nil {
		return err
	}
	return pk.Decode(data)
}

func (pk *PrivateKey) IsValid() bool {
	return pk.PublicKey().IsValid()
}

func (pk *PrivateKey) SetBytes(b []byte) *PrivateKey {
	pk.D = new(big.Int).SetBytes(b)
	return pk
}

func (pk PrivateKey) String() string {
	return hex.EncodeToString(pk.D.Bytes())
}

func GenPrivateKey() (k *big.Int, err error) {
	params := curve.Params()
	b := make([]byte, params.BitSize/8+8)
	_, err = io.ReadFull(rand.Reader, b)
	if err != nil {
		return
	}
	k = new(big.Int).SetBytes(b)
	n := new(big.Int).Sub(params.N, one)
	k.Mod(k, n)
	k.Add(k, one)
	return
}

func NewPrivateKey() (*PrivateKey, error) {
	d, err := GenPrivateKey()
	if err != nil {
		return nil, err
	}
	return &PrivateKey{D: d}, nil
}

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

func (pk PrivateKey) Marshal() []byte {
	return pk.D.Bytes()
}

func (pk *PrivateKey) PublicKey() *PublicKey {
	pub := &PublicKey{}
	pub.X, pub.Y = curve.ScalarBaseMult(pk.Marshal())
	return pub
}

type SigValue struct {
	R *big.Int
	S *big.Int
}

func (sig *SigValue) GetSigs() SigBytes {
	sb := SigBytes{}
	sb.Set(sig)
	return sb
}

func NewSigValue(b []byte) (*SigValue, error) {
	sig := &SigValue{}
	err := sig.Decode(b)
	return sig, err
}

func (sig *SigValue) FromHEX(s string) error {
	data, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	return sig.Decode(data)
}

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

func (sig *SigValue) Decode(b []byte) error {
	if r, s, err := sig.CheckLow(b); err != nil {
		return err
	} else {
		sig.R = new(big.Int).SetBytes(b[4 : 4+r])
		sig.S = new(big.Int).SetBytes(b[6+r : 6+r+s])
	}
	return nil
}

type PublicKey struct {
	X  *big.Int
	Y  *big.Int
	b0 byte
}

func NewPublicKey(data []byte) (*PublicKey, error) {
	pk := &PublicKey{}
	err := pk.Decode(data)
	return pk, err
}

func (pk *PublicKey) Equal(sb []byte) bool {
	pb := pk.Encode()
	return bytes.Equal(pb, sb)
}

func (pk *PublicKey) FromHEX(s string) error {
	data, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	return pk.Decode(data)
}

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

func (pk *PublicKey) Decode(data []byte) error {
	byteLen := (curve.Params().BitSize + 7) >> 3
	if len(data) == 0 {
		return errors.New("data empty")
	}
	pk.b0 = data[0]
	if len(data) != PUBLIC_KEY_SIZE {
		return errors.New("data size error")
	}
	if pk.b0 != P256_PUBKEY_EVEN && pk.b0 != P256_PUBKEY_ODD {
		return errors.New(" compressed head byte error")
	}
	p := curve.Params().P
	x := new(big.Int).SetBytes(data[1 : 1+byteLen])
	ybit := uint(0)
	if pk.b0 == P256_PUBKEY_ODD {
		ybit = 1
	}
	y := DecompressY(x, ybit)
	d := byte(y.Bit(0))
	if pk.b0 == P256_PUBKEY_ODD && d != 1 {
		return errors.New("decompress public key odd error")
	}
	if pk.b0 == P256_PUBKEY_EVEN && d != 0 {
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

func (pb *PublicKey) Hash() HASH160 {
	b := pb.Encode()
	return Hash160From(b)
}

func (pb *PublicKey) IsValid() bool {
	return curve.IsOnCurve(pb.X, pb.Y)
}

func (pk *PublicKey) Verify(hash []byte, sig *SigValue) bool {
	pub := new(ecdsa.PublicKey)
	pub.Curve = curve
	pub.X, pub.Y = pk.X, pk.Y
	return ecdsa.Verify(pub, hash, sig.R, sig.S)
}

func LoadPublicKey(s string) (*PublicKey, error) {
	return new(PublicKey).Load(s)
}

//账号地址
type Address string

//创建一个输出
func (a Address) NewTxOut(v Amount, ext ...[]byte) (*TxOut, error) {
	if !v.IsRange() {
		return nil, errors.New("amount error")
	}
	out := &TxOut{}
	out.Value = v
	pkh, err := a.GetPkh()
	if err != nil {
		return nil, err
	}
	if script, err := NewLockedScript(pkh, ext...); err != nil {
		return nil, err
	} else {
		out.Script = script
	}
	return out, nil
}

func (a Address) Check() error {
	_, err := DecodeAddress(a)
	return err
}

func (a Address) GetPkh() (HASH160, error) {
	return DecodeAddress(a)
}

//编码地址用指定前缀
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

func EncodeAddress(pkh HASH160) (Address, error) {
	a, err := EncodeAddressWithPrefix(conf.AddrPrefix, pkh)
	return Address(a), err
}

func DecodeAddress(addr Address) (HASH160, error) {
	hv := HASH160{}
	hrp, b, err := SegWitAddressDecode(string(addr))
	if hrp != conf.AddrPrefix {
		return hv, errors.New("address prefix error")
	}
	if err != nil {
		return hv, err
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

func (pk *PublicKey) Load(s string) (*PublicKey, error) {
	b, err := B58Decode(s, BitcoinAlphabet)
	if err != nil {
		return nil, err
	}
	l := len(b)
	if l < 16 {
		return nil, errors.New("pub length error")
	}
	hv := Hash256(b[:l-4])
	if !bytes.Equal(hv[:4], b[l-4:]) {
		return nil, errors.New("check sum error")
	}
	return pk, pk.Decode(b[:l-4])
}

func (pk *PublicKey) GetPks() PKBytes {
	pks := PKBytes{}
	return pks.Set(pk)
}

func (pk *PublicKey) Dump() string {
	b := pk.Encode()
	hv := Hash256(b)
	b = append(b, hv[:4]...)
	return B58Encode(b, BitcoinAlphabet)
}

func (pk *PublicKey) Encode() []byte {
	ret := []byte{}
	d := byte(pk.Y.Bit(0))
	ret = append(ret, P256_PUBKEY_EVEN+d)
	ret = append(ret, pk.X.Bytes()...)
	return ret
}
