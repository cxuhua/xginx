package xginx

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"time"
)

var (
	//ErrCheckSum 校验和错误，一般密码错误会出现这个问题
	ErrCheckSum = errors.New("check sum error, maybe password error")
)

//TimeDays 获取天
func TimeDays(d int) time.Duration {
	return time.Hour * 24 * time.Duration(d)
}

//TimeHour 获取小时
func TimeHour(d int) time.Duration {
	return time.Hour * time.Duration(d)
}

//TimeMinute 获取分钟
func TimeMinute(d int) time.Duration {
	return time.Minute * time.Duration(d)
}

//TimeSecond 获取秒
func TimeSecond(d int) time.Duration {
	return time.Second * time.Duration(d)
}

//EndianUInt32 用于排序
func EndianUInt32(u32 uint32) []byte {
	hb := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint32(hb, u32)
	return hb
}

//EndianUInt64 用于排序
func EndianUInt64(u64 uint64) []byte {
	hb := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint64(hb, u64)
	return hb
}

//HashDump 将数据导出并添加校验，pass存在将进行加密
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

//HashLoad HashDump 返回的进行解密和校验
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
	//使用hash160作为校验
	dl := len(data) - hl
	if !bytes.Equal(Hash160(data[:dl]), data[dl:]) {
		return nil, ErrCheckSum
	}
	return data[:dl], nil
}

//ONCE 防止被多个线程同时执行
type ONCE int32

//IsRunning 是否在运行
func (f *ONCE) IsRunning() bool {
	return atomic.CompareAndSwapInt32((*int32)(f), 1, 1)
}

//Running 如果没运行返回false，否则设置为允许并返回true
func (f *ONCE) Running() bool {
	if f.IsRunning() {
		return false
	}
	atomic.AddInt32((*int32)(f), 1)
	return true
}

//Reset 重置
func (f *ONCE) Reset() {
	atomic.AddInt32((*int32)(f), -1)
}

//UR32 uint32 随机数
func UR32() uint32 {
	return RandUInt32()
}

//Sha256 sha256 hash
func Sha256(b []byte) []byte {
	hash := sha256.Sum256(b)
	return hash[:]
}

//Ripemd160 hash
func Ripemd160(b []byte) []byte {
	h160 := NewRipemd160()
	h160.Write(b)
	return h160.Sum(nil)
}

//Hash160From 返回hash160值
func Hash160From(b []byte) HASH160 {
	hv := HASH160{}
	copy(hv[:], Hash160(b))
	return hv
}

//Hash160 使用Ripemd160进行hash160计算
func Hash160(b []byte) []byte {
	v1 := Sha256(b)
	return Ripemd160(v1)
}

//Hash256From 返回hash256值
func Hash256From(b []byte) HASH256 {
	hv := HASH256{}
	copy(hv[:], Hash256(b))
	return hv
}

//Hash256 sha256 double
func Hash256(b []byte) []byte {
	s2 := sha256.New()
	s2.Write(b)
	v1 := s2.Sum(nil)
	s2.Reset()
	s2.Write(v1)
	return s2.Sum(nil)
}

//Rand ret >= min,ret <= max
func Rand(min uint, max uint) uint {
	v := uint32(0)
	SetRandInt(&v)
	return (min + uint(v)%(max+1-min))
}

//SetRandInt 读取随机值
func SetRandInt(v interface{}) {
	_ = binary.Read(rand.Reader, binary.LittleEndian, v)
}

//RandUInt32 获取u32随机值
func RandUInt32() uint32 {
	v := uint32(0)
	_ = binary.Read(rand.Reader, binary.LittleEndian, &v)
	return v
}

//HexToBytes hex解码
func HexToBytes(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return d
}

//HexDecode hex解码
func HexDecode(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return d
}

//String 获取以0结束的字符串，\0截断,不包括\0之后的
func String(b []byte) string {
	for idx, c := range b {
		if c == 0 {
			return string(b[:idx])
		}
	}
	return string(b)
}
