package xginx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
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
	EARTH_RADIUS  = float64(6378137)
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

func (l Location) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, l[0]); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, l[1]); err != nil {
		return err
	}
	return nil
}

func (l *Location) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &l[0]); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &l[1]); err != nil {
		return err
	}
	return nil
}

func (l Location) IsZero() bool {
	return l[0] == 0 || l[1] == 0
}

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

func rad(d float64) (r float64) {
	r = d * math.Pi / 180.0
	return
}

func (l Location) Distance(v Location) float64 {
	lv := l.Get()
	lng1, lat1 := rad(lv[0]), rad(lv[1])
	vv := v.Get()
	lng2, lat2 := rad(vv[0]), rad(vv[1])
	if lat1 < 0 {
		lat1 = math.Pi/2 + math.Abs(lat1)
	}
	if lat1 > 0 {
		lat1 = math.Pi/2 - math.Abs(lat1)
	}
	if lng1 < 0 {
		lng1 = math.Pi*2 - math.Abs(lng1)
	}
	if lat2 < 0 {
		lat2 = math.Pi/2 + math.Abs(lat2)
	}
	if lat2 > 0 {
		lat2 = math.Pi/2 - math.Abs(lat2)
	}
	if lng2 < 0 {
		lng2 = math.Pi*2 - math.Abs(lng2)
	}
	x1 := EARTH_RADIUS * math.Cos(lng1) * math.Sin(lat1)
	y1 := EARTH_RADIUS * math.Sin(lng1) * math.Sin(lat1)
	z1 := EARTH_RADIUS * math.Cos(lat1)
	x2 := EARTH_RADIUS * math.Cos(lng2) * math.Sin(lat2)
	y2 := EARTH_RADIUS * math.Sin(lng2) * math.Sin(lat2)
	z2 := EARTH_RADIUS * math.Cos(lat2)
	d := math.Sqrt((x1-x2)*(x1-x2) + (y1-y2)*(y1-y2) + (z1-z2)*(z1-z2))
	theta := math.Acos((EARTH_RADIUS*EARTH_RADIUS + EARTH_RADIUS*EARTH_RADIUS - d*d) / (2 * EARTH_RADIUS * EARTH_RADIUS))
	return theta * EARTH_RADIUS
}

//
type TagCTR [3]byte

func (c TagCTR) ToUInt() uint32 {
	b4 := []byte{0, 0, 0, 0}
	copy(b4[1:], c[:])
	return binary.BigEndian.Uint32(b4)
}

func (c *TagCTR) Set(v uint) {
	b4 := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint32(b4, uint32(v))
	copy(c[:], b4[1:])
}

type PKBytes [33]byte

func (v PKBytes) Hash() HASH160 {
	return Hash160To(v[:])
}

func (v PKBytes) Cmp(b PKBytes) int {
	vu := NewUINT256(v[:])
	bu := NewUINT256(b[:])
	return vu.Cmp(bu)
}

func (v PKBytes) Equal(b PKBytes) bool {
	return bytes.Equal(v[:], b[:])
}

func (v PKBytes) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

func (v *PKBytes) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

func (p *PKBytes) SetBytes(b []byte) {
	copy(p[:], b)
}

func (p *PKBytes) Set(pk *PublicKey) PKBytes {
	copy(p[:], pk.Encode())
	return *p
}

type SigBytes [75]byte

func (v SigBytes) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

func (v *SigBytes) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

func (p *SigBytes) SetBytes(b []byte) {
	copy(p[:], b)
}

func (p *SigBytes) Set(sig *SigValue) {
	copy(p[:], sig.Encode())
}

//公钥HASH160
type HASH160 [20]byte

func NewHASH160(b []byte) HASH160 {
	id := HASH160{}
	copy(id[:], b)
	return id
}

func (v *HASH160) SetPK(pk *PublicKey) {
	*v = pk.Hash()
}

func (v *HASH160) Set(b []byte) {
	copy(v[:], b)
}

func (v HASH160) Cmp(b HASH160) int {
	u1 := NewUINT256(v[:])
	u2 := NewUINT256(b[:])
	return u1.Cmp(u2)
}

func (v HASH160) Equal(b HASH160) bool {
	return bytes.Equal(v[:], b[:])
}

func (v HASH160) Encode(w IWriter) error {
	_, err := w.Write(v[:])
	return err
}

func (v *HASH160) Decode(r IReader) error {
	_, err := r.Read(v[:])
	return err
}

type HashCacher struct {
	hash HASH256
	set  bool
}

func (h *HashCacher) Reset() {
	h.set = false
}

func (h HashCacher) IsSet() (HASH256, bool) {
	return h.hash, h.set
}

func (h *HashCacher) SetHash(hv HASH256) {
	h.hash = hv
	h.set = true
}

func (h *HashCacher) Hash(b []byte) HASH256 {
	if h.set {
		return h.hash
	}
	copy(h.hash[:], Hash256(b))
	h.set = true
	return h.hash
}

type TagUID [7]byte

func (v TagUID) Equal(b TagUID) bool {
	return bytes.Equal(v[:], b[:])
}

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
	TASV  Alloc    //积分分配比例,由标签持有者确定，写入后不可修改
	TPKH  HASH160  //标签所有者公钥的HASH160，标记标签所有者
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
	tag.TASV = S631
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
	TAG_PATH_FIX = "/sign/"
)

func (t *TagInfo) DecodeURL() error {
	if len(t.url) > 512 {
		return errors.New("url too long")
	}
	if !strings.HasPrefix(t.url, conf.HttpScheme) {
		return errors.New("url scheme error")
	}
	uv, err := url.Parse(t.url)
	if err != nil {
		return err
	}
	if strings.ToLower(uv.Scheme) != conf.HttpScheme {
		return errors.New("must use " + conf.HttpScheme)
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

func (t *TagInfo) Valid(cli *CliPart) error {
	//定位信息不能为空
	if cli.CLoc.IsZero() {
		return errors.New("cloc error")
	}
	//时间相差不能太大
	if cli.TimeErr() > conf.TimeErr {
		return errors.New("client time error")
	}
	if t.input == "" {
		return errors.New("input miss")
	}
	//获取标签信息
	itag, err := store.LoadTagInfo(t.TUID)
	if err != nil {
		return fmt.Errorf("get tag info error %w", err)
	}
	//校验mac
	if !aescmac.Vaild(itag.Mackey(), t.TUID[:], t.TCTR[:], t.TMAC[:], t.input) {
		return errors.New("cmac valid error")
	}
	//更新数据库标签计数器
	if err := store.SetTagCtr(t.TUID, t.TCTR.ToUInt()); err != nil {
		return err
	}
	//校验用户签名
	sig, err := NewSigValue(cli.CSig[:])
	if err != nil {
		return fmt.Errorf("sig error %w", err)
	}
	pub, err := NewPublicKey(cli.CPks[:])
	if err != nil {
		return fmt.Errorf("pub error %w", err)
	}
	data, err := t.ToSigBytes()
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(data)
	if err := cli.EncodeWriter(buf); err != nil {
		return err
	}
	//校验客户签名
	hv := Hash256(buf.Bytes())
	if !pub.Verify(hv, sig) {
		return fmt.Errorf("client sig verify error")
	}
	return nil
}

func (t *TagInfo) Decode(hr io.Reader) error {
	return t.DecodeReader(hr)
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
	if err := binary.Read(hr, Endian, &t.TASV); err != nil {
		return err
	}
	if err := binary.Read(hr, Endian, &t.TPKH); err != nil {
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
	if err := binary.Write(hw, Endian, t.TASV); err != nil {
		return err
	}
	if err := binary.Write(hw, Endian, t.TPKH); err != nil {
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
func (t *TagInfo) ToSigBytes() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := t.Encode(buf)
	return buf.Bytes(), err
}

//编码成url一部分写入标签
func (t *TagInfo) EncodeHex() ([]byte, error) {
	if err := t.TASV.Check(); err != nil {
		return nil, err
	}
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
	if err := binary.Write(hw, Endian, t.TASV); err != nil {
		return nil, err
	}
	if err := binary.Write(hw, Endian, t.TPKH); err != nil {
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

func (s *VarStr) Decode(r IReader) error {
	l, err := r.ReadByte()
	if err != nil {
		return err
	}
	if l == 0 {
		*s = ""
		return nil
	}
	sb := make([]byte, l)
	_, err = r.Read(sb)
	if err != nil {
		return err
	}
	*s = VarStr(sb)
	return nil
}

func (s VarStr) Encode(w IWriter) error {
	if len(s) > VAR_STR_MAX {
		return errors.New("var too big long")
	}
	if err := w.WriteByte(byte(len(s))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(s)); err != nil {
		return err
	}
	return nil
}

//client信息
//POST 编码数据到服务器
type CliPart struct {
	CLoc  Location //用户定位信息user location
	Prev  HASH256  //上个hash
	CTime int64    //客户端时间，不能和服务器相差太大 单位：纳秒
	CPks  PKBytes  //用户公钥
	CSig  SigBytes //用户签名
}

//是否是第一个记录
func (c CliPart) IsFirst() bool {
	return c.Prev.IsZero()
}

//时间差
func (c CliPart) TimeErr() float64 {
	v := float64(time.Now().UnixNano()) - float64(c.CTime)
	return math.Abs(v) / float64(time.Second)
}

//b=待签名数据，tag数据+client数据
func (c *CliPart) Verify(b []byte) error {
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
	//csig不包含在cli签名数据中
	hash := Hash256(b[:len(b)-len(c.CSig)])
	if !pub.Verify(hash, sig) {
		return errors.New("sig verify error")
	}
	return nil
}

//cpks HASH160
func (c *CliPart) ClientID() HASH160 {
	return Hash160To(c.CPks[:])
}

func (c *CliPart) Decode(r io.Reader) error {
	return c.DecodeReader(r)
}

func (c *CliPart) DecodeReader(r io.Reader) error {
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

func (c *CliPart) Encode(w io.Writer) error {
	if err := c.EncodeWriter(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, c.CSig); err != nil {
		return err
	}
	return nil
}

func (c *CliPart) EncodeWriter(w io.Writer) error {
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

func (c *CliPart) ToSigBytes() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := c.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *CliPart) Sign(pv *PrivateKey, tag []byte) ([]byte, error) {
	buf := bytes.NewBuffer(tag)
	//设置公钥
	c.CPks.Set(pv.PublicKey())
	if err := c.EncodeWriter(buf); err != nil {
		return nil, err
	}
	//签名
	hv := Hash256(buf.Bytes())
	sig, err := pv.Sign(hv)
	if err != nil {
		return nil, err
	}
	c.CSig.Set(sig)
	buf.Reset()
	if err := c.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//块信息
type SerPart struct {
	Nonce int64  //随机值 server full
	STime int64  //服务器时间 单位：纳秒
	Host  VarStr //验证主机地址地址 https://ip:port/verify,验证时拼接hex(hash)
}

func NewSerPart(url string) *SerPart {
	tb := &SerPart{}
	tb.SetRand()
	tb.SetTime()
	tb.SetHost(url)
	return tb
}

func (c *SerPart) SetRand() {
	SetRandInt(&c.Nonce)
}

func (c *SerPart) SetTime() {
	c.STime = time.Now().UnixNano()
}

func (c *SerPart) SetHost(url string) {
	c.Host = VarStr(url)
}

func (c *SerPart) Dump(tag *TagInfo, cli *CliPart) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := tag.Encode(buf); err != nil {
		return nil, err
	}
	if err := cli.Encode(buf); err != nil {
		return nil, err
	}
	if err := cli.Verify(buf.Bytes()); err != nil {
		return nil, err
	}
	if err := c.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c SerPart) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, &c.Nonce); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, &c.STime); err != nil {
		return err
	}
	if err := c.Host.Encode(w); err != nil {
		return err
	}
	return nil
}

func (c *SerPart) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &c.Nonce); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &c.STime); err != nil {
		return err
	}
	if err := c.Host.Decode(r); err != nil {
		return err
	}
	return nil
}

type Unit struct {
	TagInfo
	CliPart
	SerPart
	//hash use
	hasher HashCacher
	//是否已经验证成功
	verifyok bool
}

func (pv Unit) Equal(cv Unit) bool {
	return pv.Hash().Equal(cv.Hash())
}

func (pv *Unit) Check() error {
	//是否已经验证过
	if _, err := pv.Verify(); err != nil {
		return fmt.Errorf("verify unit error %w", err)
	}
	return nil
}

func (pv Unit) TTLocDis(cv *Unit) float64 {
	return cv.TLoc.Distance(pv.TLoc)
}

//获取两点之间的速度
//km/h
func (pv Unit) TTSpeed(cv *Unit) float64 {
	dis := cv.TLoc.Distance(pv.TLoc) / 1000
	tv := float64(cv.STime-pv.STime) / float64(time.Hour)
	return dis / tv
}

func (pv Unit) STimeSub(cv *Unit) float64 {
	return float64(cv.STime-pv.STime) / float64(time.Second)
}

//获取记录点与服务器时间差
func (b Unit) TimeSub() float64 {
	return math.Abs(float64(b.CTime-b.STime)) / float64(time.Second)
}

func (b Unit) CTLocDis() float64 {
	return b.CLoc.Distance(b.TLoc)
}

//获取定位不准产生的惩罚比例
func (b Unit) CTLocDisRate() float64 {
	return GetDisRate(b.CTLocDis())
}

func (b Unit) Encode(w IWriter) error {
	if err := b.TagInfo.Encode(w); err != nil {
		return err
	}
	if err := b.CliPart.Encode(w); err != nil {
		return err
	}
	if err := b.SerPart.Encode(w); err != nil {
		return err
	}
	return nil
}

func (b *Unit) Decode(r IReader) error {
	if err := b.TagInfo.Decode(r); err != nil {
		return err
	}
	if err := b.CliPart.Decode(r); err != nil {
		return err
	}
	if err := b.SerPart.Decode(r); err != nil {
		return err
	}
	return nil
}

//通过url访问校验数据,成功返回用户公钥hash160
func (pv *Unit) Verify() (HASH160, error) {
	cid := pv.CPks.Hash()
	if pv.verifyok {
		return cid, nil
	}
	//将tuid和hash连接生成hex格式连接在验证地址
	//访问这个验证地址访问
	vsb := append([]byte{}, pv.TUID[:]...)
	vsb = append(vsb, pv.Hash().Bytes()...)
	sh := hex.EncodeToString(vsb)
	surl := string(pv.Host) + "/" + sh
	res, err := http.Get(surl)
	if err != nil {
		return cid, err
	}
	if res.StatusCode != 200 {
		return cid, errors.New("status code error")
	}
	dat, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return cid, err
	}
	if len(dat) != 20 {
		return cid, errors.New("pkh length error")
	}
	uhash := NewHASH160(dat)
	if !cid.Equal(uhash) {
		return cid, errors.New("pkh bytes error")
	}
	pv.verifyok = true
	return cid, nil
}

func (b *Unit) Hash() HASH256 {
	if hash, ok := b.hasher.IsSet(); ok {
		return hash
	}
	buf := &bytes.Buffer{}
	_ = b.Encode(buf)
	return b.hasher.Hash(buf.Bytes())
}
