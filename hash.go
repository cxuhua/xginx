package xginx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
)

//hash宽度
const (
	UInt256Width = 256 / 32
	c1           = uint32(0xcc9e2d51)
	c2           = uint32(0x1b873593)
)

func rotl(x uint32, r int8) uint32 {
	return (x << r) | (x >> (32 - r))
}

//MurmurHash Murmur Hash算法
func MurmurHash(seed uint32, b []byte) uint32 {
	h1 := seed
	nb := len(b) / 4
	r := bytes.NewReader(b)
	b4 := []byte{0, 0, 0, 0}
	for i := 0; i < nb; i++ {
		_, _ = r.Read(b4)
		k1 := Endian.Uint32(b4)
		k1 *= c1
		k1 = rotl(k1, 15)
		k1 *= c2
		h1 ^= k1
		h1 = rotl(h1, 13)
		h1 = h1*5 + 0xe6546b64
	}
	rl, _ := r.Read(b4)
	k1 := uint32(0)
	if rl >= 3 {
		k1 ^= uint32(b4[2]) << 16
	}
	if rl >= 2 {
		k1 ^= uint32(b4[1]) << 8
	}
	if rl >= 1 {
		k1 ^= uint32(b4[0])
		k1 *= c1
		k1 = rotl(k1, 15)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint32(len(b))
	h1 ^= h1 >> 16
	h1 *= 0x85ebca6b
	h1 ^= h1 >> 13
	h1 *= 0xc2b2ae35
	h1 ^= h1 >> 16
	return h1
}

//HashCacher hash 缓存
type HashCacher struct {
	hash HASH256
	set  bool
}

//Reset 重置
func (h *HashCacher) Reset() {
	h.set = false
}

//IsSet 是否设置了hash
func (h HashCacher) IsSet() (HASH256, bool) {
	return h.hash, h.set
}

//SetHash 设置hash
func (h *HashCacher) SetHash(hv HASH256) {
	h.hash = hv
	h.set = true
}

//Hash 计算hash
func (h *HashCacher) Hash(b []byte) HASH256 {
	if h.set {
		return h.hash
	}
	copy(h.hash[:], Hash256(b))
	h.set = true
	return h.hash
}

//HASH160 公钥HASH160
type HASH160 [20]byte

//NewHASH160 初始化hash160
func NewHASH160(v interface{}) HASH160 {
	var hash HASH160
	switch v.(type) {
	case *PublicKey:
		pub := v.(*PublicKey)
		hash = pub.Hash()
	case HASH160:
		hash = v.(HASH160)
	case []byte:
		bb := v.([]byte)
		copy(hash[:], bb)
	case PKBytes:
		pks := v.(PKBytes)
		hash = Hash160From(pks[:])
	case string:
		pub, err := LoadPublicKey(v.(string))
		if err != nil {
			panic(err)
		}
		hash = pub.Hash()
	default:
		panic(errors.New("v args type error"))
	}
	return hash
}

func (v HASH160) String() string {
	return hex.EncodeToString(v[:])
}

//SetPK 使用公钥初始化
func (v *HASH160) SetPK(pk *PublicKey) {
	*v = pk.Hash()
}

//Set 二进制初始化
func (v *HASH160) Set(b []byte) {
	copy(v[:], b)
}

//Cmp 转换位hash256比较大小
func (v HASH160) Cmp(b HASH160) int {
	u1 := NewUINT256(v[:])
	u2 := NewUINT256(b[:])
	return u1.Cmp(u2)
}

//Equal 是否相等
func (v HASH160) Equal(b HASH160) bool {
	return bytes.Equal(v[:], b[:])
}

//Encode 编码到写缓存
func (v HASH160) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

//Decode 从读缓存解码
func (v *HASH160) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

//Bytes 返回二进制数据
func (v HASH160) Bytes() []byte {
	return v[:]
}

//hash定义
var (
	ErrDataSize = errors.New("size data error")
	Endian      = binary.LittleEndian
	ErrVarSize  = errors.New("var size too big")
	ZERO256     = HASH256{}
	ZERO160     = HASH160{}
)

//HASH256 bit256 hash
type HASH256 [32]byte

//Encode encode
func (h HASH256) Encode(w IWriter) error {
	_, err := w.Write(h[:])
	return err
}

//Decode decode
func (h *HASH256) Decode(r IReader) error {
	_, err := r.Read(h[:])
	return err
}

//EqualBytes 余字节二进制对比
func (h HASH256) EqualBytes(b []byte) bool {
	return bytes.Equal(b, h[:])
}

//Set 二进制初始化
func (h *HASH256) Set(b []byte) {
	copy(h[:], b)
}

//UINT256 unsigned int hash
type UINT256 [UInt256Width]uint32

//NewUINT256 创建256位大数
func NewUINT256(v interface{}) UINT256 {
	n := UINT256{}
	n.SetValue(v)
	return n
}

//Equal 比较是否相等
func (h UINT256) Equal(v UINT256) bool {
	return h.Cmp(v) == 0
}

//ToDouble 转位double
func (h UINT256) ToDouble() float64 {
	ret := float64(0)
	fact := float64(1)
	for i := 0; i < UInt256Width; i++ {
		ret += fact * float64(h[i])
		fact *= 4294967296.0
	}
	return ret
}

//SetValue 设置值
func (h *UINT256) SetValue(v interface{}) {
	*h = UINT256{}
	switch v.(type) {
	case uint32:
		h[0] = v.(uint32)
	case int32:
		h[0] = uint32(v.(int32))
	case uint:
		cv := uint64(v.(uint))
		h[0] = uint32(cv)
		h[1] = uint32(cv >> 32)
	case int:
		cv := uint64(v.(int))
		h[0] = uint32(cv)
		h[1] = uint32(cv >> 32)
	case int64:
		cv := v.(int64)
		h[0] = uint32(cv)
		h[1] = uint32(cv >> 32)
	case uint64:
		cv := v.(uint64)
		h[0] = uint32(cv)
		h[1] = uint32(cv >> 32)
	case string:
		str := v.(string)
		if len(str)%2 != 0 {
			str = "0" + str
		}
		sv, err := hex.DecodeString(str)
		if err != nil {
			panic(err)
		}
		vl := ((len(sv) + 3) / 4)
		bv := make([]byte, vl*4)
		for i := 0; i < len(sv); i++ {
			bv[i] = sv[len(sv)-i-1]
		}
		ui := UINT256{}
		for i := 0; i < vl; i++ {
			ui[i] = Endian.Uint32(bv[i*4 : i*4+4])
		}
		*h = ui
	case []byte:
		sv := v.([]byte)
		sl := len(sv)
		if sl > UInt256Width*4 {
			sl = UInt256Width * 4
		}
		vl := ((sl + 3) / 4)
		bv := make([]byte, vl*4)
		copy(bv, sv)
		ui := UINT256{}
		for i := 0; i < vl; i++ {
			ui[i] = Endian.Uint32(bv[i*4 : i*4+4])
		}
		*h = ui
	default:
		panic(errors.New("v type error" + reflect.TypeOf(v).String()))
	}
}

//GetUint64 获取64位值
func (h HASH256) GetUint64(idx int) uint64 {
	return Endian.Uint64(h[idx*8 : idx*8+8])
}

//ToUHash hash256转为大数
func (h HASH256) ToUHash() UINT256 {
	x := UINT256{}
	for i := 0; i < UInt256Width; i++ {
		x[i] = Endian.Uint32(h[i*4 : i*4+4])
	}
	return x
}

//IsZero 是否为空
func (h UINT256) IsZero() bool {
	for _, v := range h {
		if v != 0 {
			return false
		}
	}
	return true
}

func (h UINT256) String() string {
	s := ""
	for i := UInt256Width - 1; i >= 0; i-- {
		b4 := []byte{0, 0, 0, 0}
		Endian.PutUint32(b4, h[i])
		s += fmt.Sprintf("%.2x%.2x%.2x%.2x", b4[3], b4[2], b4[1], b4[0])
	}
	return s
}

//Low64 获取低位
func (h UINT256) Low64() uint64 {
	return uint64(h[0]) | (uint64(h[1]) << 32)
}

//Bits 获取位数
func (h UINT256) Bits() uint {
	for pos := UInt256Width - 1; pos >= 0; pos-- {
		if h[pos] != 0 {
			for bits := uint(31); bits > 0; bits-- {
				if h[pos]&uint32(1<<bits) != 0 {
					return uint(32*pos) + bits + 1
				}
			}
			return uint(32*pos) + 1
		}
	}
	return 0
}

//MulUInt32 *
func (h UINT256) MulUInt32(v uint32) UINT256 {
	a := UINT256{}
	carry := uint64(0)
	for i := 0; i < UInt256Width; i++ {
		n := carry + uint64(v)*uint64(h[i])
		a[i] = uint32(n & 0xffffffff)
		carry = n >> 32
	}
	return a
}

//Mul c = a * b
func (h UINT256) Mul(v UINT256) UINT256 {
	a := UINT256{}
	for j := 0; j < UInt256Width; j++ {
		carry := uint64(0)
		for i := 0; i+j < UInt256Width; i++ {
			n := carry + uint64(a[i+j]) + uint64(h[j])*uint64(v[i])
			a[i+j] = uint32(n & 0xffffffff)
			carry = n >> 32
		}
	}
	return a
}

//Neg a = ^h
func (h UINT256) Neg() UINT256 {
	a := UINT256{}
	for i := 0; i < UInt256Width; i++ {
		a[i] = ^h[i]
	}
	return a.Add(NewUINT256(1))
}

//Cmp 比较
// >0 =  >
// <0 =  <
// =0 =  =
func (h UINT256) Cmp(b UINT256) int {
	for i := UInt256Width - 1; i >= 0; i-- {
		if h[i] < b[i] {
			return -1
		}
		if h[i] > b[i] {
			return +1
		}
	}
	return 0
}

//Sub a = b - c
func (h UINT256) Sub(b UINT256) UINT256 {
	return h.Add(b.Neg())
}

//Add a = b + c
func (h UINT256) Add(b UINT256) UINT256 {
	a := UINT256{}
	carry := uint64(0)
	for i := 0; i < UInt256Width; i++ {
		n := carry + uint64(h[i]) + uint64(b[i])
		a[i] = uint32(n & 0xffffffff)
		carry = n >> 32
	}
	return a
}

//Div a = b /c
func (h UINT256) Div(b UINT256) UINT256 {
	a := UINT256{}
	num := h
	div := b
	nbits := num.Bits()
	dbits := div.Bits()
	if dbits == 0 {
		panic(errors.New("Division by zero"))
	}
	if dbits > nbits {
		return a
	}
	shift := int(nbits - dbits)
	div = div.Lshift(uint(shift))
	for shift >= 0 {
		if num.Cmp(div) >= 0 {
			num = num.Sub(div)
			a[shift/32] |= (1 << (shift & 31))
		}
		div = div.Rshift(1)
		shift--
	}
	return a
}

//Rshift >>
func (h UINT256) Rshift(shift uint) UINT256 {
	x := h
	for i := 0; i < UInt256Width; i++ {
		h[i] = 0
	}
	k := int(shift / 32)
	shift = shift % 32
	for i := 0; i < UInt256Width; i++ {
		if i-k-1 >= 0 && shift != 0 {
			h[i-k-1] |= (x[i] << (32 - shift))
		}
		if i-k >= 0 {
			h[i-k] |= (x[i] >> shift)
		}
	}
	return h
}

//Lshift <<
func (h UINT256) Lshift(shift uint) UINT256 {
	x := h
	for i := 0; i < UInt256Width; i++ {
		h[i] = 0
	}
	k := int(shift / 32)
	shift = shift % 32
	for i := 0; i < UInt256Width; i++ {
		if i+k+1 < UInt256Width && shift != 0 {
			h[i+k+1] |= (x[i] >> (32 - shift))
		}
		if i+k < UInt256Width {
			h[i+k] |= (x[i] << shift)
		}
	}
	return h
}

//SetCompact return Negative,Overflow
func (h *UINT256) SetCompact(c uint32) (bool, bool) {
	size := c >> 24
	word := c & 0x007fffff
	if size <= 3 {
		word >>= 8 * (3 - size)
		*h = NewUINT256(word)
	} else {
		*h = NewUINT256(word)
		*h = h.Lshift(8 * uint(size-3))
	}
	negative := word != 0 && (c&0x00800000) != 0
	overflow := word != 0 && ((size > 34) || (word > 0xff && size > 33) || (word > 0xffff && size > 32))
	return negative, overflow
}

//Compact Compact
func (h UINT256) Compact(negative bool) uint32 {
	size := (h.Bits() + 7) / 8
	compact := uint64(0)
	if size <= 3 {
		compact = h.Low64() << (8 * (3 - uint64(size)))
	} else {
		nb := h.Rshift(8 * (size - 3))
		compact = nb.Low64()
	}
	if compact&0x00800000 != 0 {
		compact >>= 8
		size++
	}
	compact |= uint64(size) << 24
	if negative && (compact&0x007fffff) != 0 {
		compact |= 0x00800000
	} else {
		compact |= 0
	}
	return uint32(compact)
}

//ToHASH256 大数转hash256
func (h UINT256) ToHASH256() HASH256 {
	x := HASH256{}
	for i := 0; i < UInt256Width; i++ {
		b4 := []byte{0, 0, 0, 0}
		Endian.PutUint32(b4, h[i])
		copy(x[i*4+0:i*4+4], b4)
	}
	return x
}

func (h HASH256) String() string {
	sv := h.Swap()
	return hex.EncodeToString(sv[:])
}

//IsZero 是否为空
func (h HASH256) IsZero() bool {
	bz := make([]byte, len(h))
	return bytes.Equal(h[:], bz)
}

//Equal ==
func (h HASH256) Equal(v HASH256) bool {
	return bytes.Equal(h[:], v[:])
}

//Bytes 获取二进制数据
func (h HASH256) Bytes() []byte {
	return h[:]
}

//Swap 反转
func (h HASH256) Swap() HASH256 {
	v := HASH256{}
	j := 0
	for i := len(h) - 1; i >= 0; i-- {
		v[j] = h[i]
		j++
	}
	return v
}

//NewHASH256 动态类型创建
func NewHASH256(v interface{}) HASH256 {
	b := HASH256{}
	switch v.(type) {
	case []byte:
		bs := v.([]byte)
		l := len(bs)
		if l > len(b) {
			l = len(b)
		}
		copy(b[len(b)-l:], bs)
	case string:
		s := v.(string)
		if len(s) > 64 {
			panic(ErrDataSize)
		}
		if len(s)%2 != 0 {
			s = "0" + s
		}
		bs, err := hex.DecodeString(s)
		if err != nil {
			panic(err)
		}
		copy(b[len(b)-len(bs):], bs)
		b = b.Swap()
	default:
		panic(errors.New("v type error" + reflect.TypeOf(v).String()))
	}
	return b
}
