package xginx

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cxuhua/aescmac"
)

var (
	Endian     = binary.LittleEndian
	VarSizeErr = errors.New("var size too big")
)

const (
	LocScaleValue = float64(10000000)
)

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

type Location [2]uint32

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
type TagCTR [3]byte

func (c TagCTR) ToUInt() uint {
	b4 := []byte{0, 0, 0, 0}
	copy(b4[1:], c[:])
	return uint(binary.BigEndian.Uint32(b4))
}

func (c *TagCTR) Set(v uint) {
	b4 := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint32(b4, uint32(v))
	copy(c[:], b4[1:])
}

type PKBytes [33]byte

func (p *PKBytes) Set(pk *PublicKey) {
	copy(p[:], pk.Encode())
}

type SigBytes [75]byte

func (p *SigBytes) Set(sig *SigValue) {
	copy(p[:], sig.Encode())
}

type HashID [32]byte

type TagUID [7]byte

type TagMAC [8]byte

var (
	TTS = []string{"I", "C", "O"}
)

func CheckTTS(b byte) bool {
	for _, v := range TTS {
		if b == v[0] {
			return true
		}
	}
	return false
}

type TagTT [2]byte

func (tt TagTT) IsValid() bool {
	return CheckTTS(tt[0]) && CheckTTS(tt[1])
}

func NewTagTT(s string) TagTT {
	return TagTT{s[0], s[1]}
}

// https://api.xginx.com/sign/
// 01000000
// bcb45f6b 764532cd
// 00000000000000
// c5e581f79ac615bd1563a7fe457aeeec2be2e2a538a4dfbe8395f0ff336d4a082f
// 000000
// 00
// 00
// 0000000000000000
//标签信息
type TagInfo struct {
	TTS  TagTT    //TT S状态 url +2,激活后OO tam map
	TVer uint32   //版本 from tag
	TLoc Location //uint32-uint32 位置 from tag
	TUID TagUID   //标签id from tag
	TPKS PKBytes  //标签公钥 from tag
	TCTR TagCTR   //标签记录计数器 from tag map
	TMAC TagMAC   //标签CMAC值 from tag url + 16
	URL  string
}

func NewTagInfo(surl string) *TagInfo {
	tag := &TagInfo{}
	tag.TVer = 1
	tag.TTS = NewTagTT("II")
	tag.URL = surl
	return tag
}

func (t *TagInfo) SetTLoc(lng, lat float64) {
	t.TLoc.Set(lng, lat)
}

func (t *TagInfo) SetTPK(pk *PublicKey) {
	copy(t.TPKS[:], pk.Encode())
}

type TagPos struct {
	OFF int
	UID int
	CTR int
	TTS int
	MAC int
}

func (p TagPos) String() string {
	return fmt.Sprintf("UID=%d CTR=%d TTS=%d MAC=%d", p.UID, p.CTR, p.TTS, p.MAC)
}

const (
	TAG_SCHEME   = "https"
	TAG_PATH_LEN = 134
	TAG_PATH_FIX = "/sign/"
)

func (t *TagInfo) Valid(client *ClientBlock) error {
	if !strings.HasPrefix(t.URL, TAG_SCHEME) {
		return errors.New("url scheme error")
	}
	uv, err := url.Parse(t.URL)
	if err != nil {
		return err
	}
	if strings.ToLower(uv.Scheme) != TAG_SCHEME {
		return errors.New("must use https")
	}
	if len(uv.Path) != TAG_PATH_LEN {
		return errors.New("path length error")
	}
	hurl := uv.Path[len(TAG_PATH_FIX):]
	if err := t.Decode(hurl); err != nil {
		return err
	}
	//检测计数器
	itag, err := LoadTagInfo(t.TUID)
	if err == nil && itag.CCTR.ToUInt() >= t.TCTR.ToUInt() {
		return errors.New("counter error")
	}
	//检测cmac
	tkey, err := LoadTagKeys(t.TUID)
	if err != nil {
		return err
	}
	//不包括cmac hex格式
	input := uv.Host + uv.Path
	input = input[:len(input)-len(t.TMAC)*2]
	//暂时默认使用密钥0
	if !aescmac.Vaild(tkey.Keys[0][:], t.TUID[:], t.TCTR[:], t.TMAC[:], input) {
		return errors.New("cmac valid error")
	}
	itag.Time = time.Now().UnixNano()
	itag.CCTR = t.TCTR
	itag.CPKS = client.CPKS
	if err := itag.Save(t.TUID); err != nil {
		return err
	}
	//检测用户签名
	return nil
}

func (t *TagInfo) Decode(s string) error {
	copy(t.TTS[:], s[:2])
	sr := strings.NewReader(s[2:])
	hr := hex.NewDecoder(sr)
	if err := binary.Read(hr, Endian, &t.TVer); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TLoc); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TUID); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TPKS); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TCTR); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TMAC); err != nil {
		return err
	}
	return nil
}

//编码成url一部分写入标签
func (t TagInfo) Encode(pos *TagPos) (string, error) {
	sb := &strings.Builder{}
	hw := hex.NewEncoder(sb)
	pos.TTS = pos.OFF + sb.Len()
	sb.WriteString(string(t.TTS[:]))
	if err := binary.Write(hw, Endian, t.TVer); err != nil {
		return "", err
	}
	if err := binary.Write(hw, Endian, t.TLoc); err != nil {
		return "", err
	}
	pos.UID = pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TUID); err != nil {
		return "", err
	}
	if err := binary.Write(hw, Endian, t.TPKS); err != nil {
		return "", err
	}
	pos.CTR = pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TCTR); err != nil {
		return "", err
	}
	pos.MAC = pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TMAC); err != nil {
		return "", err
	}
	return strings.ToUpper(sb.String()), nil
}

func (t TagInfo) GetSignData() ([]byte, error) {
	pos := &TagPos{}
	str, err := t.Encode(pos)
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}

//client信息
//POST 编码数据到服务器
type ClientBlock struct {
	CLoc  Location //用户定位信息user location
	Prev  HashID   //上个hash
	CTime uint64   //客户端时间，不能和服务器相差太大
	CPKS  PKBytes  //用户公钥 from user
	CSig  SigBytes //用户签名不包含在签名数据中
}

//块信息
type TagBlock struct {
	Client ClientBlock
	Nonce  uint64   //随机值 server full
	SSig   SigBytes //服务器签名
}
