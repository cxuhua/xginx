package xginx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	Endian     = binary.LittleEndian
	VarSizeErr = errors.New("var size too big")
)

const (
	VAR_INT_MAX   = VarInt(^uint64(0) >> 1)
	VAR_MAX_BYTES = 10
	VAR_INT_MIN   = (^VAR_INT_MAX + 1)

	LocScaleValue = float64(10000000)
)

//last sigle bit
type VarInt int64

func ReadVarInt(s io.Reader) (VarInt, error) {
	v := VarInt(0)
	err := v.Read(s)
	return v, err
}

func WriteVarInt(s io.Writer, v VarInt) error {
	return v.Write(s)
}

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

func (vi VarInt) Write(s io.Writer) error {
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

func (vi *VarInt) Read(s io.Reader) error {
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

type Location [2]uint32

//编码解码
func (l Location) Encode(s io.Writer) error {
	if err := binary.Write(s, Endian, l[0]); err != nil {
		return err
	}
	if err := binary.Write(s, Endian, l[1]); err != nil {
		return err
	}
	return nil
}

func (l *Location) Decode(s io.Reader) error {
	if err := binary.Read(s, Endian, &l[0]); err != nil {
		return err
	}
	if err := binary.Read(s, Endian, &l[1]); err != nil {
		return err
	}
	return nil
}

func (l Location) Equal(v Location) bool {
	return l[0] == v[0] && l[1] == v[1]
}

//设置经纬度
func (l *Location) Set(lng, lat float64) {
	l[0] = uint32(lng * LocScaleValue)
	l[1] = uint32(lat * LocScaleValue)
}

func (l *Location) Get() (float64, float64) {
	lng := float64(l[0]) / LocScaleValue
	lat := float64(l[1]) / LocScaleValue
	return lng, lat
}

func (l Location) Distance(v Location) float64 {
	return 0
}

//
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

type TagUID [7]byte

type TagMAC [8]byte

type TagTT [2]byte

type TagEncodePos struct {
	UID int
	CTR int
	TTS int
	MAC int
}

//终端记录
type TagRecord struct {
	TVer  uint8        //版本 from tag
	TLoc  Location     //uint32-uint32 位置 from tag
	TUID  TagUID       //标签id from tag
	TPK   PKBytes      //标签公钥 from tag
	TCTR  UInt24       //标签记录计数器 from tag
	TTS   TagTT        //TT S状态 url +2,激活后OO
	TMAC  TagMAC       //标签CMAC值 from tag url + 16
	CPC   byte         //C pubkey count 4(bit)公钥数量-4(bit)需要的私钥数量最大15
	CSig  []SigBytes   //用户签名 from user b[0] = 1 user sig
	CPK   []PKBytes    //用户公钥 from user
	Nonce uint64       //随机值 server full
	STime uint64       //uint64 create time serve full
	SSig  SigBytes     //标签签名 b[0] = 2 tag sig server full
	Hash  HashID       //最最终hash
	pos   TagEncodePos //记录偏移位置用
}

func (t TagRecord) TEqual(v TagRecord) bool {
	if t.TVer != v.TVer {
		return false
	}
	if !t.TLoc.Equal(v.TLoc) {
		return false
	}
	if !bytes.Equal(t.TUID[:], v.TUID[:]) {
		return false
	}
	if !bytes.Equal(t.TPK[:], v.TPK[:]) {
		return false
	}
	if !bytes.Equal(t.TCTR[:], v.TCTR[:]) {
		return false
	}
	if !bytes.Equal(t.TTS[:], v.TTS[:]) {
		return false
	}
	if !bytes.Equal(t.TMAC[:], v.TMAC[:]) {
		return false
	}
	return true
}

func (tag *TagRecord) Decode(s string) error {
	sb := strings.NewReader(s)
	hr := hex.NewDecoder(sb)
	b1 := []byte{0}
	if _, err := hr.Read(b1); err != nil {
		return err
	} else {
		tag.TVer = b1[0]
	}
	if err := tag.TLoc.Decode(hr); err != nil {
		return err
	}
	if _, err := hr.Read(tag.TPK[:]); err != nil {
		return err
	}
	if _, err := hr.Read(tag.TUID[:]); err != nil {
		return err
	}
	if _, err := hr.Read(tag.TCTR[:]); err != nil {
		return err
	}
	if _, err := hr.Read(tag.TTS[:]); err != nil {
		return err
	}
	if _, err := hr.Read(tag.TMAC[:]); err != nil {
		return err
	}
	return nil
}

func (tag TagRecord) EncodeTag() (string, error) {
	sb := &strings.Builder{}
	hw := hex.NewEncoder(sb)
	if _, err := hw.Write([]byte{tag.TVer}); err != nil {
		return "", err
	}
	if err := tag.TLoc.Encode(hw); err != nil {
		return "", err
	}
	if _, err := hw.Write(tag.TPK[:]); err != nil {
		return "", err
	}
	tag.pos.UID = sb.Len()
	if _, err := hw.Write(tag.TUID[:]); err != nil {
		return "", err
	}
	tag.pos.CTR = sb.Len()
	if _, err := hw.Write(tag.TCTR[:]); err != nil {
		return "", err
	}
	tag.pos.TTS = sb.Len()
	if _, err := hw.Write(tag.TTS[:]); err != nil {
		return "", err
	}
	tag.pos.MAC = sb.Len()
	if _, err := hw.Write(tag.TMAC[:]); err != nil {
		return "", err
	}
	return sb.String(), nil
}
