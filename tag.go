package xginx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	Endian     = binary.LittleEndian
	VarSizeErr = errors.New("var size too big")
)

const (
	VAR_INT_MAX   = VarInt(^uint64(0) >> 1)
	VAR_MAX_BYTES = 10
	VAR_INT_MIN   = (^VAR_INT_MAX + 1)
)

//last sigle bit
type VarInt int64

//0-63
func MaxBits(v uint64) uint {
	i := uint(63)
	for ; i > 0; i-- {
		if v&(uint64(1)<<i) != 0 {
			break
		}
	}
	return i
}

func (vi VarInt) IsValid() bool {
	return vi >= VAR_INT_MIN && vi <= VAR_INT_MAX
}

func (vi VarInt) Write(s io.ReadWriter) error {
	if !vi.IsValid() {
		return VarSizeErr
	}
	v := uint64(0)
	sb := vi < 0
	if sb {
		v = uint64(-vi)<<1 | 1
	} else {
		v = uint64(vi) << 1
	}
	tmp := make([]byte, VAR_MAX_BYTES)
	l := 0
	for {
		if l > 0 {
			tmp[l] = byte(v&0x7F) | 0x80
		} else {
			tmp[l] = byte(v & 0x7F)
		}
		if v <= 0x7F {
			break
		}
		v = (v >> 7) - 1
		l++
	}
	for l >= 0 {
		if err := binary.Write(s, Endian, tmp[l]); err != nil {
			return err
		}
		l--
	}
	return nil
}

func (vi *VarInt) Read(s io.ReadWriter) error {
	n := uint64(0)
	b := 0
	for i := 0; i < VAR_MAX_BYTES; i++ {
		ch := uint8(0)
		if err := binary.Read(s, Endian, &ch); err != nil {
			return fmt.Errorf("var int read error %w", err)
		}
		b++
		n = (n << 7) | uint64(ch&0x7F)
		if ch&0x80 != 0 {
			n++
		} else {
			break
		}
	}
	if n&0b1 != 0 {
		*vi = VarInt(^(n >> 1)) + 1
	} else {
		*vi = VarInt(n >> 1)
	}
	if !vi.IsValid() {
		return VarSizeErr
	}
	return nil
}

type Location [2]int32

func (l *Location) Read(s io.ReadWriter) error {
	return nil
}

func (l *Location) Write(s io.ReadWriter) error {
	return nil
}

const (
	LocScaleValue = float64(10000000)
)

//设置经纬度
func (l *Location) Set(lng, lat float64) {
	l[0] = int32(lng * LocScaleValue)
	l[1] = int32(lat * LocScaleValue)
}

func (l *Location) Get() (float64, float64) {
	lng := float64(l[0]) / LocScaleValue
	lat := float64(l[1]) / LocScaleValue
	return lng, lat
}

func (l Location) Distance(v Location) float64 {
	return 0
}

type UInt24 [3]byte

func (v *UInt24) ToUInt32() uint32 {
	b := []byte{v[0], v[1], v[2], 0x00}
	return binary.LittleEndian.Uint32(b)
}

func (v *UInt24) SetUInt32(x uint32) {
	b := []byte{0x00, 0x00, 0x00, 0x00}
	binary.LittleEndian.PutUint32(b, x)
	v[0], v[1], v[2] = b[0], b[1], b[2]
}

type PKBytes [33]byte

type SigBytes [73]byte

type HashID [32]byte

type Block struct {
	Ver   uint32   //版本 from tag encdata +2
	TLoc  Location //uint32-uint32 位置 from tag encdata + 16 标签初始化写入
	TPK   PKBytes  //标签公钥 from tag encdata + 66
	UID   [7]byte  //标签id from tag picc
	CTR   UInt24   //标签记录计数器 from picc tag picc total:32
	TTS   [2]byte  //TT S状态 url +2,激活后OO
	CMAC  [8]byte  //标签CMAC值 from tag url + 16
	CLoc  Location //用户打卡位置 from user 从手机定位获取
	CPK   PKBytes  //用户公钥 from user
	CSig  SigBytes //用户签名 from user b[0] = 1 user sig
	Nonce uint64   //随机值 server full
	Time  uint64   //uint64 create time serve full
	TSig  SigBytes //标签签名 b[0] = 2 tag sig server full
}
