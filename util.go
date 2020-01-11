package xginx

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sync/atomic"
)

//dump b58,设定密码会加密
func HashDump(b []byte, pass ...string) (string, error) {
	hash := Hash160(b)
	data := append(b, hash...)
	if len(pass) > 0 && pass[0] != "" {
		block := NewAESCipher([]byte(pass[0]))
		d, err := AesEncrypt(block, data)
		if err != nil {
			return "", err
		}
		data = d
	}
	return B58Encode(data, BitcoinAlphabet), nil
}

//load b58 string
func HashLoad(s string, pass ...string) ([]byte, error) {
	hl := len(HASH160{})
	data, err := B58Decode(s, BitcoinAlphabet)
	if err != nil {
		return nil, err
	}
	if len(pass) > 0 && pass[0] != "" {
		block := NewAESCipher([]byte(pass[0]))
		d, err := AesDecrypt(block, data)
		if err != nil {
			return nil, err
		}
		data = d
	}
	dl := len(data) - hl
	if !bytes.Equal(Hash160(data[:dl]), data[dl:]) {
		return nil, errors.New("checksum error")
	}
	return data[:dl], nil
}

//防止被多个线程同时执行

type ONCE int32

func (f *ONCE) IsRunning() bool {
	return atomic.CompareAndSwapInt32((*int32)(f), 1, 1)
}

func (f *ONCE) Running() bool {
	if f.IsRunning() {
		return false
	} else {
		atomic.AddInt32((*int32)(f), 1)
		return true
	}
}

func (f *ONCE) Reset() {
	atomic.AddInt32((*int32)(f), -1)
}

func UR32() uint32 {
	v := uint32(0)
	SetRandInt(&v)
	return v
}

func Sha256(b []byte) []byte {
	hash := sha256.Sum256(b)
	return hash[:]
}

func Ripemd160(b []byte) []byte {
	h160 := NewRipemd160()
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

func RandUInt32() uint32 {
	v := uint32(0)
	_ = binary.Read(rand.Reader, binary.LittleEndian, &v)
	return v
}

func HexToBytes(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func HexDecode(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return d
}

//获取以0结束的字符串，\0截断,不包括\0之后的
func String(b []byte) string {
	for idx, c := range b {
		if c == 0 {
			return string(b[:idx])
		}
	}
	return string(b)
}
