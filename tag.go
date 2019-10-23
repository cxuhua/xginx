package xginx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

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
	l[0] = uint32(int32(lng * LocScaleValue))
	l[1] = uint32(int32(lat * LocScaleValue))
}

func (l *Location) Get() []float64 {
	lng := float64(int32(l[0])) / LocScaleValue
	lat := float64(int32(l[1])) / LocScaleValue
	return []float64{lng, lat}
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

func (id TagUID) Bytes() []byte {
	return id[:]
}

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
	TTS   TagTT    //TT S状态 url +2,激活后OO tam map
	TVer  uint32   //版本 from tag
	TLoc  Location //uint32-uint32 位置 from tag
	TUID  TagUID   //标签id from tag
	TPKS  PKBytes  //标签公钥 from tag
	TCTR  TagCTR   //标签记录计数器 from tag map
	TMAC  TagMAC   //标签CMAC值 from tag url + 16
	URL   string   //
	Input string   //cmac valid input DecodeURL set
	pos   TagPos
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

func (t *TagInfo) DecodeURL() error {
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
	input := uv.Host + uv.Path
	t.Input = input[:len(input)-len(t.TMAC)*2]
	return nil
}

func (t *TagInfo) Valid(db DBImp, client *ClientBlock) error {
	if t.Input == "" {
		return errors.New("input miss")
	}
	//获取标签信息
	itag, err := LoadTagInfo(t.TUID, db)
	if err != nil {
		return err
	}
	//检测标签计数器
	if itag.CTR >= t.TCTR.ToUInt() {
		return errors.New("tag counter error")
	}
	//暂时默认使用密钥0
	if !aescmac.Vaild(itag.Keys[0][:], t.TUID[:], t.TCTR[:], t.TMAC[:], t.Input) {
		return errors.New("cmac valid error")
	}
	//更新数据库标签计数器
	if err := itag.SetCtr(t.TCTR.ToUInt(), db); err != nil {
		return err
	}
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
func (t *TagInfo) Encode() (string, error) {
	sb := &strings.Builder{}
	hw := hex.NewEncoder(sb)
	t.pos.TTS = t.pos.OFF + sb.Len()
	sb.WriteString(string(t.TTS[:]))
	if err := binary.Write(hw, Endian, t.TVer); err != nil {
		return "", err
	}
	if err := binary.Write(hw, Endian, t.TLoc); err != nil {
		return "", err
	}
	t.pos.UID = t.pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TUID); err != nil {
		return "", err
	}
	if err := binary.Write(hw, Endian, t.TPKS); err != nil {
		return "", err
	}
	t.pos.CTR = t.pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TCTR); err != nil {
		return "", err
	}
	t.pos.MAC = t.pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TMAC); err != nil {
		return "", err
	}
	return strings.ToUpper(sb.String()), nil
}

//client信息
//POST 编码数据到服务器
type ClientBlock struct {
	CLoc  Location //用户定位信息user location
	Prev  HashID   //上个hash
	CTime int64    //客户端时间，不能和服务器相差太大
	CPKS  PKBytes  //用户公钥 from user
	CSig  SigBytes //用户签名不包含在签名数据中
}

func (c *ClientBlock) EncodeWriter(w io.Writer) error {
	if err := binary.Write(w, Endian, c.CLoc); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.Prev); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.CTime); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.CPKS); err != nil {
		return err
	}
	return nil
}

func (c *ClientBlock) Encode() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := c.EncodeWriter(buf); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, Endian, c.CSig); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *ClientBlock) Sign(pv *PrivateKey, tag string) error {
	buf := bytes.NewBuffer([]byte(tag))
	//设置公钥
	c.CPKS.Set(pv.PublicKey())
	if err := c.EncodeWriter(buf); err != nil {
		return err
	}
	//签名
	hv := HASH256(buf.Bytes())
	sig, err := pv.Sign(hv)
	if err != nil {
		return err
	}
	c.CSig.Set(sig)
	return nil
}

//块信息
type TagBlock struct {
	Nonce int64    //随机值 server full
	SSig  SigBytes //服务器签名
}

func (c *TagBlock) Sign(pv *PrivateKey, tag []byte, client []byte) error {
	return nil
}
