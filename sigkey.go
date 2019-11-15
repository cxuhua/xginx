package xginx

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
)

const (
	PUBLIC_KEY_SIZE  = 33
	P256_PUBKEY_EVEN = byte(0x02)
	P256_PUBKEY_ODD  = byte(0x03)
)

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

func (pk *PrivateKey) Encode() []byte {
	pb := pk.D.Bytes()
	buf := &bytes.Buffer{}
	buf.Write(PREFIX_SECRET_KEY)
	buf.Write(pb)
	buf.WriteByte(1)
	hv := Hash256(buf.Bytes())
	buf.Write(hv[:4])
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
	res := new(bytes.Buffer)
	res.WriteByte(0x30)
	res.WriteByte(byte(4 + len(r) + len(s)))
	res.WriteByte(0x02)
	res.WriteByte(byte(len(r)))
	res.Write(r)
	res.WriteByte(0x02)
	res.WriteByte(byte(len(s)))
	res.Write(s)
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
	uid := HASH160{}
	copy(uid[:], Hash160(b))
	return uid
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

func EncodeAddress(pkh HASH160) (string, error) {
	ver := byte(0)
	b := []byte{ver, byte(len(pkh))}
	b = append(b, pkh[:]...)
	addr, err := SegWitAddressEncode(conf.AddrPrefix, b)
	if err != nil {
		return "", err
	}
	return addr, nil
}

func DecodeAddress(addr string) (HASH160, error) {
	hv := HASH160{}
	hrp, b, err := SegWitAddressDecode(addr)
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
	pks.Set(pk)
	return pks
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
