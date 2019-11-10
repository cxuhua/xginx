package xginx

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/ripemd160"
)

var (
	MAX_COMPRESS_UINT = uint64(0b1111 << 57)
)

//max : 60 bits
func CompressUInt(n uint64) uint64 {
	if n > MAX_COMPRESS_UINT {
		panic(VarSizeErr)
	}
	if n == 0 {
		return 0
	}
	e := uint64(0)
	for ((n % 10) == 0) && e < 9 {
		n /= 10
		e++
	}
	if e < 9 {
		d := (n % 10)
		n /= 10
		n = 1 + (n*9+d-1)*10 + e
	} else {
		n = 1 + (n-1)*10 + 9
	}
	return n
}

//max : 60 bits
func DecompressUInt(x uint64) uint64 {
	if x == 0 {
		return 0
	}
	x--
	e := x % 10
	x /= 10
	n := uint64(0)
	if e < 9 {
		d := (x % 9) + 1
		x /= 9
		n = x*10 + d
	} else {
		n = x + 1
	}
	for e != 0 {
		n *= 10
		e--
	}
	if n > MAX_COMPRESS_UINT {
		panic(VarSizeErr)
	}
	return n
}

func Sha256(b []byte) []byte {
	hash := sha256.Sum256(b)
	return hash[:]
}

func Ripemd160(b []byte) []byte {
	h160 := ripemd160.New()
	h160.Write(b)
	return h160.Sum(nil)
}

func Hash160From(b []byte) HASH160 {
	hv := HASH160{}
	copy(hv[:], Hash160(b))
	return hv
}

func Hash160(b []byte) []byte {
	v1 := Sha256(b)
	return Ripemd160(v1)
}

func Hash256From(b []byte) HASH256 {
	hv := HASH256{}
	copy(hv[:], Hash256(b))
	return hv
}

func Hash256(b []byte) []byte {
	s2 := sha256.New()
	s2.Write(b)
	v1 := s2.Sum(nil)
	s2.Reset()
	s2.Write(v1)
	return s2.Sum(nil)
}

//ret >= min,ret <= max
func Rand(min uint, max uint) uint {
	v := uint(0)
	SetRandInt(&v)
	return (min + v%(max+1-min))
}

func SetRandInt(v interface{}) {
	_ = binary.Read(rand.Reader, binary.LittleEndian, v)
}

func HexToBytes(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func HexDecode(s string) []byte {
	s = strings.Replace(s, "_", "", -1)
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func String(b []byte) string {
	for idx, c := range b {
		if c == 0 {
			return string(b[:idx])
		}
	}
	return string(b)
}
