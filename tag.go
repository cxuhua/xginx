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

func (l *Location) SetLoc(loc []uint32) {
	l[0] = loc[0]
	l[1] = loc[1]
}

//设置经纬度
func (l *Location) Set(lng, lat float64) {
	l[0] = uint32(int32(lng * LocScaleValue))
	l[1] = uint32(int32(lat * LocScaleValue))
}

func (l *Location) ToUInt() []uint32 {
	return l[:]
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

func (p *PKBytes) SetBytes(b []byte) {
	copy(p[:], b)
}

func (p *PKBytes) Set(pk *PublicKey) PKBytes {
	copy(p[:], pk.Encode())
	return *p
}

type SigBytes [75]byte

func (p *SigBytes) SetBytes(b []byte) {
	copy(p[:], b)
}

func (p *SigBytes) Set(sig *SigValue) {
	copy(p[:], sig.Encode())
}

type HashID [32]byte

func (v *HashID) Set(b []byte) {
	copy(v[:], b)
}

func (v HashID) String() string {
	return hex.EncodeToString(v[:])
}

type TagUID [7]byte

func (id *TagUID) Set(b []byte) {
	copy(id[:], b)
}

func (id TagUID) Bytes() []byte {
	return id[:]
}

type TagMAC [8]byte

func (m *TagMAC) Set(b []byte) {
	copy(m[:], b)
}

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

func (tt *TagTT) Set(b []byte) {
	tt[0] = b[0]
	tt[1] = b[1]
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
	pub, err := NewPublicKey(client.CPks[:])
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

func (t *TagInfo) Encode(w io.Writer) error {
	return t.EncodeWriter(w)
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
	err := t.Encode(buf)
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
	CPks  PKBytes  //用户公钥
	CSig  SigBytes //用户签名
}

//b=待签名数据，tag数据+client数据
func (c *ClientBlock) Verify(pool *CertPool, b []byte) error {
	if len(b) < len(c.CPks)+len(c.CSig) {
		return errors.New("data size error")
	}
	pub, err := NewPublicKey(c.CPks[:])
	if err != nil {
		return err
	}
	sig, err := NewSigValue(c.CSig[:])
	if err != nil {
		return err
	}
	hash := HASH256(b[:len(b)-len(c.CSig)])
	if !pub.Verify(hash, sig) {
		return errors.New("sig verify error")
	}
	return nil
}

//cpks hash160
func (c *ClientBlock) ClientID() []byte {
	return HASH160(c.CPks[:])
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
	if err := binary.Read(r, Endian, &c.CPks); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.CSig); err != nil {
		return err
	}
	return nil
}

func (c *ClientBlock) Encode(w io.Writer) error {
	if err := c.EncodeWriter(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.CSig); err != nil {
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
	if err := binary.Write(w, Endian, c.CPks); err != nil {
		return err
	}
	return nil
}

func (c *ClientBlock) ToSigBinary() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := c.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *ClientBlock) Sign(pv *PrivateKey, tag []byte) error {
	buf := bytes.NewBuffer(tag)
	//设置公钥
	c.CPks.Set(pv.PublicKey())
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
	Nonce int64    //随机值 server full
	STime int64    //服务器时间
	SPks  PKBytes  //服务器公钥
	SSig  SigBytes //服务器签名
}

func (c *ServerBlock) Encode(w io.Writer) error {
	if err := c.EncodeWriter(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.SSig); err != nil {
		return err
	}
	return nil
}

func (c *ServerBlock) EncodeWriter(w io.Writer) error {
	if err := binary.Write(w, Endian, &c.Nonce); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, &c.STime); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, &c.SPks); err != nil {
		return err
	}
	return nil
}

func (c *ServerBlock) DecodeReader(r io.Reader) error {
	if err := binary.Read(r, Endian, &c.Nonce); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.STime); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.SPks); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.SSig); err != nil {
		return err
	}
	return nil
}

//b=校验数据
func (s *ServerBlock) Verify(pool *CertPool, b []byte) error {
	if len(b) < len(s.SPks)+len(s.SSig) {
		return errors.New("data size error")
	}
	sig, err := NewSigValue(s.SSig[:])
	if err != nil {
		return err
	}
	hash := HASH256(b[:len(b)-len(s.SSig)])
	return pool.Verify(s.SPks, sig, hash)
}

func (c *ServerBlock) Sign(pool *CertPool, tag *TagInfo, client *ClientBlock) ([]byte, error) {
	cert, err := pool.SignCert()
	if err != nil {
		return nil, err
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
	SetRandInt(&c.Nonce)
	//设置服务器时间
	c.STime = time.Now().UnixNano()
	//设置签名公钥
	c.SPks.Set(cert.PublicKey())
	if err := binary.Write(buf, Endian, c.Nonce); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, Endian, c.STime); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, Endian, c.SPks); err != nil {
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

func (b *TBlockInfo) Encode(w io.Writer) error {
	tag := &TagInfo{}
	tag.TTS.Set(b.TTS)
	tag.TVer = b.TVer
	tag.TLoc.SetLoc(b.TLoc)
	tag.TUID.Set(b.TUID)
	tag.TCTR.Set(b.TCTR)
	tag.TMAC.Set(b.TMAC)
	if err := tag.Encode(w); err != nil {
		return err
	}
	cli := &ClientBlock{}
	cli.CLoc.SetLoc(b.CLoc)
	cli.Prev.Set(b.Prev)
	cli.CTime = b.CTime
	cli.CPks.SetBytes(b.CPks)
	cli.CSig.SetBytes(b.CSig)
	if err := cli.Encode(w); err != nil {
		return err
	}
	ser := &ServerBlock{}
	ser.Nonce = b.Nonce
	ser.STime = b.STime
	ser.SPks.SetBytes(b.SPks)
	ser.SSig.SetBytes(b.SSig)
	if err := ser.Encode(w); err != nil {
		return err
	}
	return nil
}

//校验块数据并返回对象
func VerifyBlockInfo(pool *CertPool, bs []byte) (*TBlockInfo, error) {
	if len(bs) > 512 {
		return nil, errors.New("data error")
	}
	buf := bytes.NewBuffer(bs)
	tag := &TagInfo{}
	if err := tag.DecodeReader(buf); err != nil {
		return nil, err
	}
	cli := &ClientBlock{}
	if err := cli.DecodeReader(buf); err != nil {
		return nil, err
	}
	ser := &ServerBlock{}
	if err := ser.DecodeReader(buf); err != nil {
		return nil, err
	}
	if err := ser.Verify(pool, bs); err != nil {
		return nil, err
	}
	v := &TBlockInfo{}
	v.Hash = HASH256(bs)
	v.TTS = tag.TTS[:]
	v.TVer = tag.TVer
	v.TLoc = tag.TLoc.ToUInt()
	v.TUID = tag.TUID[:]
	v.TCTR = tag.TCTR.ToUInt()
	v.TMAC = tag.TMAC[:]
	v.CLoc = cli.CLoc.ToUInt()
	v.Prev = cli.Prev[:]
	v.CTime = cli.CTime
	v.CPks = cli.CPks[:]
	v.CSig = cli.CSig[:]
	v.Nonce = ser.Nonce
	v.STime = ser.STime
	v.SPks = ser.SPks[:]
	v.SSig = ser.SSig[:]
	return v, nil
}
