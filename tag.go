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

func NewHashID(v []byte) HashID {
	id := HashID{}
	copy(id[:], v)
	return id
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

func (p *PKBytes) Set(pk *PublicKey) PKBytes {
	copy(p[:], pk.Encode())
	return *p
}

type SigBytes [75]byte

func (p *SigBytes) Set(sig *SigValue) {
	copy(p[:], sig.Encode())
}

type HashID [32]byte

func (v HashID) String() string {
	return hex.EncodeToString(v[:])
}

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

//标签信息
type TagInfo struct {
	TTS   TagTT    //TT状态 url +2,激活后OO tam map
	TVer  uint32   //版本 from tag
	TLoc  Location //uint32-uint32 位置 from tag
	TUID  TagUID   //标签id from tag
	TCTR  TagCTR   //标签记录计数器 from tag map
	TMAC  TagMAC   //标签CMAC值 from tag url + 16
	url   string
	input string
	pos   TagPos
}

func NewTagInfo(url ...string) *TagInfo {
	tag := &TagInfo{}
	tag.TVer = 1
	tag.TTS = NewTagTT("II")
	if len(url) > 0 {
		tag.url = url[0]
	}
	return tag
}

func (t *TagInfo) SetTLoc(lng, lat float64) {
	t.TLoc.Set(lng, lat)
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
	TAG_PATH_FIX = "/sign/"
)

func (t *TagInfo) DecodeURL() error {
	if len(t.url) > 512 {
		return errors.New("url too long")
	}
	if !strings.HasPrefix(t.url, TAG_SCHEME) {
		return errors.New("url scheme error")
	}
	uv, err := url.Parse(t.url)
	if err != nil {
		return err
	}
	if strings.ToLower(uv.Scheme) != TAG_SCHEME {
		return errors.New("must use https")
	}
	if len(uv.Path) > 256 {
		return errors.New("path too long")
	}
	hurl := uv.Path[len(TAG_PATH_FIX):]
	if err := t.DecodeHex([]byte(hurl)); err != nil {
		return err
	}
	input := uv.Host + uv.Path
	t.input = input[:len(input)-len(t.TMAC)*2]
	return nil
}

func (t *TagInfo) Valid(db DBImp, client *ClientBlock) error {
	if t.input == "" {
		return errors.New("input miss")
	}
	//获取标签信息
	itag, err := LoadTagInfo(t.TUID, db)
	if err != nil {
		return fmt.Errorf("get tag info error %w", err)
	}
	//检测标签计数器
	if itag.CTR >= t.TCTR.ToUInt() {
		return errors.New("tag counter error")
	}
	//暂时默认使用密钥0
	if !aescmac.Vaild(itag.Mackey(), t.TUID[:], t.TCTR[:], t.TMAC[:], t.input) {
		return errors.New("cmac valid error")
	}
	//更新数据库标签计数器
	if err := db.AtomicCtr(t.TUID[:], t.TCTR.ToUInt()); err != nil {
		return fmt.Errorf("update counter error %w", err)
	}
	//校验用户签名
	sig, err := NewSigValue(client.CSig[:])
	if err != nil {
		return fmt.Errorf("sig error %w", err)
	}
	pub, err := NewPublicKey(client.CPKS[:])
	if err != nil {
		return fmt.Errorf("pub error %w", err)
	}
	data, err := t.ToSigBinary()
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(data)
	if err := client.EncodeWriter(buf); err != nil {
		return err
	}
	//校验客户签名
	hv := HASH256(buf.Bytes())
	if !pub.Verify(hv, sig) {
		return fmt.Errorf("client sig verify error")
	}
	return nil
}

func (t *TagInfo) DecodeReader(hr io.Reader) error {
	if err := binary.Read(hr, Endian, &t.TTS); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TVer); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TLoc); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TUID); err != nil {
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

func (t *TagInfo) EncodeWriter(hw io.Writer) error {
	if err := binary.Write(hw, Endian, t.TTS); err != nil {
		return err
	}
	if err := binary.Write(hw, Endian, t.TVer); err != nil {
		return err
	}
	if err := binary.Write(hw, Endian, t.TLoc); err != nil {
		return err
	}
	if err := binary.Write(hw, Endian, t.TUID); err != nil {
		return err
	}
	if err := binary.Write(hw, Endian, t.TCTR); err != nil {
		return err
	}
	if err := binary.Write(hw, Endian, t.TMAC); err != nil {
		return err
	}
	return nil
}

func (t *TagInfo) DecodeHex(s []byte) error {
	b := hex.EncodeToString(s[:2])
	b = b + string(s[2:])
	sr := bytes.NewReader([]byte(b))
	hr := hex.NewDecoder(sr)
	return t.DecodeReader(hr)
}

//将标签encode数据转换为二进制签名数据
//开头两字节直接转，因为不是hex编码
func (t *TagInfo) ToSigBinary() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := t.EncodeWriter(buf)
	return buf.Bytes(), err
}

//编码成url一部分写入标签
func (t *TagInfo) EncodeHex() ([]byte, error) {
	sb := &strings.Builder{}
	hw := hex.NewEncoder(sb)
	t.pos.TTS = t.pos.OFF + sb.Len()
	sb.WriteString(string(t.TTS[:]))
	if err := binary.Write(hw, Endian, t.TVer); err != nil {
		return nil, err
	}
	if err := binary.Write(hw, Endian, t.TLoc); err != nil {
		return nil, err
	}
	t.pos.UID = t.pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TUID); err != nil {
		return nil, err
	}
	t.pos.CTR = t.pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TCTR); err != nil {
		return nil, err
	}
	t.pos.MAC = t.pos.OFF + sb.Len()
	if err := binary.Write(hw, Endian, t.TMAC); err != nil {
		return nil, err
	}
	return []byte(strings.ToUpper(sb.String())), nil
}

const (
	VAR_STR_MAX = int(^uint8(0))
)

//最大支持255长度字节
type VarStr string

func (s *VarStr) DecodeReader(r io.Reader) error {
	b0 := []byte{0}
	_, err := r.Read(b0)
	if err != nil {
		return err
	}
	if b0[0] == 0 {
		*s = ""
		return nil
	}
	sb := make([]byte, b0[0])
	_, err = r.Read(sb)
	if err != nil {
		return err
	}
	*s = VarStr(sb)
	return nil
}

func (s VarStr) EncodeWriter(w io.Writer) error {
	if len(s) > VAR_STR_MAX {
		panic(errors.New("var too big long"))
	}
	r := []byte{byte(len(s))}
	if len(s) > 0 {
		r = append(r, []byte(s)...)
	}
	_, err := w.Write(r)
	return err
}

//client信息
//POST 编码数据到服务器
type ClientBlock struct {
	CLoc  Location //用户定位信息user location
	Prev  HashID   //上个hash
	CTime int64    //客户端时间，不能和服务器相差太大
	CPKS  PKBytes  //用户公钥
	CSig  SigBytes //用户签名
}

//cpks hash160
func (c *ClientBlock) ClientID() []byte {
	return HASH160(c.CPKS[:])
}

func (c *ClientBlock) DecodeReader(r io.Reader) error {
	if err := binary.Read(r, Endian, &c.CLoc); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.Prev); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.CTime); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.CPKS); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.CSig); err != nil {
		return err
	}
	return nil
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

func (c *ClientBlock) ToSigBinary() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := c.EncodeWriter(buf); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, Endian, c.CSig); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *ClientBlock) Sign(pv *PrivateKey, tag []byte) error {
	buf := bytes.NewBuffer(tag)
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
type ServerBlock struct {
	Nnoce int64    //随机值 server full
	STime int64    //服务器时间
	SPKS  PKBytes  //服务器公钥
	SSig  SigBytes //服务器签名
}

func (c *ServerBlock) EncodeWriter(w io.Writer) error {
	if err := binary.Write(w, Endian, &c.Nnoce); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, &c.STime); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, &c.SPKS); err != nil {
		return err
	}
	return nil
}

func (c *ServerBlock) DecodeReader(r io.Reader) error {
	if err := binary.Read(r, Endian, &c.Nnoce); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.STime); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.SPKS); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.SSig); err != nil {
		return err
	}
	return nil
}

type BlockData []byte

type BlockInfo struct {
	TagInfo
	ClientBlock
	ServerBlock
}

func (b *BlockInfo) Verify() error {
	buf := &bytes.Buffer{}
	tb, err := b.TagInfo.ToSigBinary()
	if err != nil {
		return err
	}
	if _, err := buf.Write(tb); err != nil {
		return err
	}
	if err := b.ClientBlock.EncodeWriter(buf); err != nil {
		return err
	}
	//verify client sig
	pub, err := NewPublicKey(b.ClientBlock.CPKS[:])
	if err != nil {
		return err
	}
	sig, err := NewSigValue(b.ClientBlock.CSig[:])
	if err != nil {
		return err
	}
	hash := HASH256(buf.Bytes())
	if !pub.Verify(hash, sig) {
		return errors.New("verify client data sig error")
	}
	if _, err := buf.Write(b.ClientBlock.CSig[:]); err != nil {
		return err
	}
	if err := b.ServerBlock.EncodeWriter(buf); err != nil {
		return err
	}
	//verify server sig
	sig, err = NewSigValue(b.ServerBlock.SSig[:])
	if err != nil {
		return err
	}
	//获取证书
	cert, err := conf.GetNodeCert(b.ServerBlock.SPKS)
	if err != nil {
		return err
	}
	if err := cert.Verify(); err != nil {
		return err
	}
	hash = HASH256(buf.Bytes())
	if !cert.PublicKey().Verify(hash, sig) {
		return errors.New("verify server data sig error")
	}
	return nil
}

func NewBlockInfo(b []byte) (*BlockInfo, error) {
	info := &BlockInfo{}
	buf := bytes.NewBuffer(b)
	if err := info.TagInfo.DecodeReader(buf); err != nil {
		return nil, err
	}
	if err := info.ClientBlock.DecodeReader(buf); err != nil {
		return nil, err
	}
	if err := info.ServerBlock.DecodeReader(buf); err != nil {
		return nil, err
	}
	return info, nil
}

func (d BlockData) Decode() (*BlockInfo, error) {
	b := &BlockInfo{}
	hr := bytes.NewReader(d)
	if err := b.TagInfo.DecodeReader(hr); err != nil {
		return nil, err
	}
	if err := b.ClientBlock.DecodeReader(hr); err != nil {
		return nil, err
	}
	if err := b.ServerBlock.DecodeReader(hr); err != nil {
		return nil, err
	}
	return b, nil
}

func (d BlockData) Save(db DBImp) error {
	return nil
}

func (d BlockData) Hash() HashID {
	return NewHashID(HASH256(d))
}

func (c *ServerBlock) Sign(cert *Cert, tag *TagInfo, client *ClientBlock) (BlockData, error) {
	if err := cert.Verify(); err != nil {
		return nil, fmt.Errorf("sign server bock error %w", err)
	}
	buf := &bytes.Buffer{}
	tdata, err := tag.ToSigBinary()
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(tdata); err != nil {
		return nil, err
	}
	cdata, err := client.ToSigBinary()
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(cdata); err != nil {
		return nil, err
	}
	//设置随机值
	SetRandInt(&c.Nnoce)
	//设置服务器时间
	c.STime = time.Now().UnixNano()
	//设置签名公钥
	c.SPKS.Set(cert.PublicKey())
	if err := binary.Write(buf, Endian, c.Nnoce); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, Endian, c.STime); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, Endian, c.SPKS); err != nil {
		return nil, err
	}
	//计算服务器签名
	hv := HASH256(buf.Bytes())
	sig, err := cert.PrivateKey().Sign(hv)
	if err != nil {
		return nil, err
	}
	c.SSig.Set(sig)
	if err := binary.Write(buf, Endian, c.SSig); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
