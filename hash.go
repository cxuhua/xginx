package xginx

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
)

const (
	UIHashWidth = 256 / 32
)

var (
	SizeError = errors.New("size data error")
)

//bytes hash
type HashID [32]byte

func (v HashID) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}
func (v *HashID) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

func (v HashID) EqualBytes(b []byte) bool {
	return bytes.Equal(b, v[:])
}

func (v *HashID) Set(b []byte) {
	copy(v[:], b)
}

//unsigned int hash
type UIHash [UIHashWidth]uint32

func NewUIHash(v interface{}) UIHash {
	n := UIHash{}
	n.SetValue(v)
	return n
}

func (h UIHash) Equal(v UIHash) bool {
	return h.Cmp(v) == 0
}

func (h UIHash) ToDouble() float64 {
	ret := float64(0)
	fact := float64(1)
	for i := 0; i < UIHashWidth; i++ {
		ret += fact * float64(h[i])
		fact *= 4294967296.0
	}
	return ret
}

func (h *UIHash) SetValue(v interface{}) {
	*h = UIHash{}
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
		ui := UIHash{}
		for i := 0; i < vl; i++ {
			ui[i] = Endian.Uint32(bv[i*4 : i*4+4])
		}
		*h = ui
	case []byte:
		sv := v.([]byte)
		vl := ((len(sv) + 3) / 4)
		bv := make([]byte, vl*4)
		copy(bv, sv)
		ui := UIHash{}
		for i := 0; i < vl; i++ {
			ui[i] = Endian.Uint32(bv[i*4 : i*4+4])
		}
		*h = ui
	default:
		panic(errors.New("v type error" + reflect.TypeOf(v).String()))
	}
}

func (h HashID) GetUint64(idx int) uint64 {
	return Endian.Uint64(h[idx*8 : idx*8+8])
}

func (h HashID) ToUHash() UIHash {
	x := UIHash{}
	for i := 0; i < UIHashWidth; i++ {
		x[i] = Endian.Uint32(h[i*4 : i*4+4])
	}
	return x
}

func (h UIHash) IsZero() bool {
	for _, v := range h {
		if v != 0 {
			return false
		}
	}
	return true
}

func (h UIHash) String() string {
	s := ""
	for i := UIHashWidth - 1; i >= 0; i-- {
		b4 := []byte{0, 0, 0, 0}
		Endian.PutUint32(b4, h[i])
		s += fmt.Sprintf("%.2x%.2x%.2x%.2x", b4[3], b4[2], b4[1], b4[0])
	}
	return s
}

func (b UIHash) Low64() uint64 {
	return uint64(b[0]) | (uint64(b[1]) << 32)
}

func (b UIHash) Bits() uint {
	for pos := UIHashWidth - 1; pos >= 0; pos-- {
		if b[pos] != 0 {
			for bits := uint(31); bits > 0; bits-- {
				if b[pos]&uint32(1<<bits) != 0 {
					return uint(32*pos) + bits + 1
				}
			}
			return uint(32*pos) + 1
		}
	}
	return 0
}

func (h UIHash) MulUInt32(v uint32) UIHash {
	a := UIHash{}
	carry := uint64(0)
	for i := 0; i < UIHashWidth; i++ {
		n := carry + uint64(v)*uint64(h[i])
		a[i] = uint32(n & 0xffffffff)
		carry = n >> 32
	}
	return a
}

// c = a * b
func (h UIHash) Mul(v UIHash) UIHash {
	a := UIHash{}
	for j := 0; j < UIHashWidth; j++ {
		carry := uint64(0)
		for i := 0; i+j < UIHashWidth; i++ {
			n := carry + uint64(a[i+j]) + uint64(h[j])*uint64(v[i])
			a[i+j] = uint32(n & 0xffffffff)
			carry = n >> 32
		}
	}
	return a
}

//a = ^h
func (h UIHash) Neg() UIHash {
	a := UIHash{}
	for i := 0; i < UIHashWidth; i++ {
		a[i] = ^h[i]
	}
	return a.Add(NewUIHash(1))
}

// >0 =  >
// <0 =  <
// =0 =  =
func (h UIHash) Cmp(b UIHash) int {
	for i := UIHashWidth - 1; i >= 0; i-- {
		if h[i] < b[i] {
			return -1
		}
		if h[i] > b[i] {
			return +1
		}
	}
	return 0
}

//a = b - c
func (h UIHash) Sub(b UIHash) UIHash {
	return h.Add(b.Neg())
}

//a = b + c
func (h UIHash) Add(b UIHash) UIHash {
	a := UIHash{}
	carry := uint64(0)
	for i := 0; i < UIHashWidth; i++ {
		n := carry + uint64(h[i]) + uint64(b[i])
		a[i] = uint32(n & 0xffffffff)
		carry = n >> 32
	}
	return a
}

// a = b /c

func (h UIHash) Div(b UIHash) UIHash {
	a := UIHash{}
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

//>>
func (b UIHash) Rshift(shift uint) UIHash {
	x := b
	for i := 0; i < UIHashWidth; i++ {
		b[i] = 0
	}
	k := int(shift / 32)
	shift = shift % 32
	for i := 0; i < UIHashWidth; i++ {
		if i-k-1 >= 0 && shift != 0 {
			b[i-k-1] |= (x[i] << (32 - shift))
		}
		if i-k >= 0 {
			b[i-k] |= (x[i] >> shift)
		}
	}
	return b
}

//<<
func (b UIHash) Lshift(shift uint) UIHash {
	x := b
	for i := 0; i < UIHashWidth; i++ {
		b[i] = 0
	}
	k := int(shift / 32)
	shift = shift % 32
	for i := 0; i < UIHashWidth; i++ {
		if i+k+1 < UIHashWidth && shift != 0 {
			b[i+k+1] |= (x[i] >> (32 - shift))
		}
		if i+k < UIHashWidth {
			b[i+k] |= (x[i] << shift)
		}
	}
	return b
}

//return Negative,Overflow
func (b *UIHash) SetCompact(c uint32) (bool, bool) {
	size := c >> 24
	word := c & 0x007fffff
	if size <= 3 {
		word >>= 8 * (3 - size)
		*b = NewUIHash(word)
	} else {
		*b = NewUIHash(word)
		*b = b.Lshift(8 * uint(size-3))
	}
	negative := word != 0 && (c&0x00800000) != 0
	overflow := word != 0 && ((size > 34) || (word > 0xff && size > 33) || (word > 0xffff && size > 32))
	return negative, overflow
}

func (b UIHash) Compact(negative bool) uint32 {
	size := (b.Bits() + 7) / 8
	compact := uint64(0)
	if size <= 3 {
		compact = b.Low64() << (8 * (3 - uint64(size)))
	} else {
		nb := b.Rshift(8 * (size - 3))
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

func (h UIHash) ToHashID() HashID {
	x := HashID{}
	for i := 0; i < UIHashWidth; i++ {
		b4 := []byte{0, 0, 0, 0}
		Endian.PutUint32(b4, h[i])
		copy(x[i*4+0:i*4+4], b4)
	}
	return x
}

func (h HashID) String() string {
	sv := h.Swap()
	return hex.EncodeToString(sv[:])
}

func (b HashID) IsZero() bool {
	bz := make([]byte, len(b))
	return bytes.Equal(b[:], bz)
}

func (b HashID) Equal(v HashID) bool {
	return bytes.Equal(b[:], v[:])
}

func (b HashID) Bytes() []byte {
	return b[:]
}

func (b HashID) Swap() HashID {
	v := HashID{}
	j := 0
	for i := len(b) - 1; i >= 0; i-- {
		v[j] = b[i]
		j++
	}
	return v
}

func NewHashID(v interface{}) HashID {
	b := HashID{}
	switch v.(type) {
	case []byte:
		bs := v.([]byte)
		l := len(bs)
		if l > len(b) {
			panic(SizeError)
		}
		copy(b[len(b)-l:], bs)
	case string:
		s := v.(string)
		if len(s) > 64 {
			panic(SizeError)
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
