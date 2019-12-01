package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"strconv"
	"time"
)

//协议包标识
const (
	NT_VERSION = uint8(1)
	//ping/pong
	NT_PING = uint8(2)
	NT_PONG = uint8(3)
	//获取节点连接的其他地址
	NT_GET_ADDRS = uint8(4)
	NT_ADDRS     = uint8(5)
	//inv 交易或者区块通报
	//当有新的交易或者区块生成通报给周边的节点
	NT_INV = uint8(6)
	//获取交易或者区块
	NT_GET_INV = uint8(7)
	//获取交易的返回
	NT_TX = uint8(8)
	//获取区块的返回
	NT_BLOCK = uint8(9)
	//获取区块头
	NT_GET_HEADERS = uint8(10)
	//返回区块头，最多200个
	NT_HEADERS = uint8(11)
	//返回一个错误信息
	NT_ERROR = uint8(12)
	//消息通知
	NT_ALERT = uint8(13)
	//过滤器 加载 添加 清除
	NT_FILTER_LOAD  = uint8(14)
	NT_FILTER_ADD   = uint8(15)
	NT_FILTER_CLEAR = uint8(16)
	//交易merkle树
	NT_GET_MERKLE = uint8(17)
	NT_TX_MERKLE  = uint8(18)
	//获取内存交易池
	NT_GET_TXPOOL = uint8(19)
	NT_TXPOOL     = uint8(20)
	//取消交易池交易
	NT_CANCEL_TX = uint8(21)
	//广播包头和响应
	NT_BROAD_HEAD = uint8(0xf0)
	NT_BROAD_ACK  = uint8(0xf1)
)

type MsgBroadAck struct {
	Id MsgId
}

func (e MsgBroadAck) Type() uint8 {
	return NT_BROAD_ACK
}

func (e MsgBroadAck) Encode(w IWriter) error {
	return w.WriteFull(e.Id[:])
}

func (e *MsgBroadAck) Decode(r IReader) error {
	return r.ReadFull(e.Id[:])
}

type MsgBroadHead struct {
	Id MsgId //md5
}

func (e MsgBroadHead) Type() uint8 {
	return NT_BROAD_HEAD
}

func (e MsgBroadHead) Encode(w IWriter) error {
	return w.WriteFull(e.Id[:])
}

func (e *MsgBroadHead) Decode(r IReader) error {
	return r.ReadFull(e.Id[:])
}

var (
	NotIdErr = errors.New("msg not id,can't broad")
	ErrMsgId = MsgId{}
)

//使用md5
type MsgId [16]byte

//协议消息
type MsgIO interface {
	Id() (MsgId, error) //实现了此方法的包才能进行广播，负责返回 NotIdErr
	Type() uint8
	Encode(w IWriter) error
	Decode(r IReader) error
}

type MsgEmpty struct {
}

func (e MsgEmpty) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (e MsgEmpty) Type() uint8 {
	return 0
}

func (e MsgEmpty) Encode(w IWriter) error {
	return nil
}

func (e *MsgEmpty) Decode(r IReader) error {
	return nil
}

const (
	ErrCodeRecvBlock  = 100001
	ErrCodeRecvTx     = 100002
	ErrCodeFilterMiss = 100003
	ErrCodeFilterLoad = 100004
	ErrCodeTxMerkle   = 100005
	ErrCodeCancelTx   = 100006
)

type MsgError struct {
	Code  int32    //错误代码
	Error VarBytes //错误信息
	Ext   VarBytes //扩展信息
}

func NewMsgError(code int, err error) *MsgError {
	return &MsgError{
		Code:  int32(code),
		Error: []byte(err.Error()),
	}
}

func (e MsgError) Type() uint8 {
	return NT_ERROR
}

func (e MsgError) Encode(w IWriter) error {
	if err := w.TWrite(e.Code); err != nil {
		return err
	}
	if err := e.Error.Encode(w); err != nil {
		return err
	}
	if err := e.Ext.Encode(w); err != nil {
		return err
	}
	return nil
}

func (e *MsgError) Decode(r IReader) error {
	if err := r.TRead(&e.Code); err != nil {
		return err
	}
	if err := e.Error.Decode(r); err != nil {
		return err
	}
	if err := e.Ext.Decode(r); err != nil {
		return err
	}
	return nil
}

type NetAddr struct {
	ip   net.IP
	port uint16
}

func NetAddrForm(s string) NetAddr {
	n := NetAddr{}
	_ = n.From(s)
	return n
}

func (n *NetAddr) From(s string) error {
	h, p, err := net.SplitHostPort(s)
	if err != nil {
		return err
	}
	n.ip = net.ParseIP(h)
	i, err := strconv.ParseInt(p, 10, 32)
	if err != nil {
		return err
	}
	n.port = uint16(i)
	return nil
}

//是否是有效的可链接的地址
func (c NetAddr) IsGlobalUnicast() bool {
	return c.ip.IsGlobalUnicast()
}

func (c NetAddr) Network() string {
	return c.ToTcpAddr().Network()
}

func (c NetAddr) ToTcpAddr() *net.TCPAddr {
	return &net.TCPAddr{
		IP:   c.ip,
		Port: int(c.port),
	}
}

func (v NetAddr) Equal(d NetAddr) bool {
	return v.ip.Equal(d.ip) && v.port == d.port
}

func (v NetAddr) String() string {
	return v.Addr()
}

func (v NetAddr) Addr(h ...string) string {
	if len(h) > 0 {
		return net.JoinHostPort(h[0], fmt.Sprintf("%d", v.port))
	} else {
		return net.JoinHostPort(v.ip.String(), fmt.Sprintf("%d", v.port))
	}
}

func (v NetAddr) Encode(w IWriter) error {
	b := v.ip.To16()
	if _, err := w.Write(b); err != nil {
		return err
	}
	if err := w.TWrite(v.port); err != nil {
		return err
	}
	return nil
}

func (v *NetAddr) Decode(r IReader) error {
	ip6 := make([]byte, 16)
	if _, err := r.Read(ip6); err != nil {
		return err
	}
	v.ip = ip6
	if err := r.TRead(&v.port); err != nil {
		return err
	}
	return nil
}

type MsgPing struct {
	Time   int64
	Height BHeight //发送我的最新高度
}

func (v MsgPing) Type() uint8 {
	return NT_PING
}

func (v MsgPing) NewPong(h BHeight) *MsgPong {
	msg := &MsgPong{Time: v.Time}
	msg.Height = h
	return msg
}

func (v MsgPing) Encode(w IWriter) error {
	if err := w.TWrite(v.Time); err != nil {
		return err
	}
	if err := w.TWrite(v.Height); err != nil {
		return err
	}
	return nil
}

func (v *MsgPing) Decode(r IReader) error {
	if err := r.TRead(&v.Time); err != nil {
		return err
	}
	if err := r.TRead(&v.Height); err != nil {
		return err
	}
	return nil
}

func NewMsgPing(h BHeight) *MsgPing {
	msg := &MsgPing{Time: time.Now().UnixNano()}
	msg.Height = h
	return msg
}

type MsgPong struct {
	Time   int64
	Height BHeight //获取对方的高度
}

func (v MsgPong) Type() uint8 {
	return NT_PONG
}

func (v MsgPong) Ping() int {
	return int((time.Now().UnixNano() - v.Time) / 1000000)
}

func (v MsgPong) Encode(w IWriter) error {
	if err := w.TWrite(v.Time); err != nil {
		return err
	}
	if err := w.TWrite(v.Height); err != nil {
		return err
	}
	return nil
}

func (v *MsgPong) Decode(r IReader) error {
	if err := r.TRead(&v.Time); err != nil {
		return err
	}
	if err := r.TRead(&v.Height); err != nil {
		return err
	}
	return nil
}

const (
	//全节点
	SERVICE_NODE = 1 << 0
)

type BHeight struct {
	BH uint32 //数据高度
	HH uint32 //头高度
}

func (v BHeight) Encode(w IWriter) error {
	if err := w.TWrite(v.BH); err != nil {
		return err
	}
	if err := w.TWrite(v.HH); err != nil {
		return err
	}
	return nil
}

func (v *BHeight) Decode(r IReader) error {
	if err := r.TRead(&v.BH); err != nil {
		return err
	}
	if err := r.TRead(&v.HH); err != nil {
		return err
	}
	return nil
}

//版本消息包
type MsgVersion struct {
	Ver     uint32  //版本
	Service uint32  //服务
	Addr    NetAddr //节点外网地址
	Height  BHeight //节点区块高度
	NodeID  uint64  //节点随机id
	Tps     VarUInt //交易池数量
}

//在链上生成一个版本数据包
func (bi *BlockIndex) NewMsgVersion() *MsgVersion {
	m := &MsgVersion{}
	m.Ver = conf.Ver
	m.Addr = conf.GetNetAddr()
	m.Height = bi.GetNodeHeight()
	m.Service = SERVICE_NODE
	m.NodeID = conf.nodeid
	m.Tps = VarUInt(bi.txp.Len())
	return m
}

func (v MsgVersion) Type() uint8 {
	return NT_VERSION
}

func (v MsgVersion) Encode(w IWriter) error {
	if err := w.TWrite(v.Ver); err != nil {
		return err
	}
	if err := w.TWrite(v.Service); err != nil {
		return err
	}
	if err := v.Addr.Encode(w); err != nil {
		return err
	}
	if err := v.Height.Encode(w); err != nil {
		return err
	}
	if err := w.TWrite(v.NodeID); err != nil {
		return err
	}
	if err := v.Tps.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *MsgVersion) Decode(r IReader) error {
	if err := r.TRead(&v.Ver); err != nil {
		return err
	}
	if err := r.TRead(&v.Service); err != nil {
		return err
	}
	if err := v.Addr.Decode(r); err != nil {
		return err
	}
	if err := v.Height.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&v.NodeID); err != nil {
		return err
	}
	if err := v.Tps.Decode(r); err != nil {
		return err
	}
	return nil
}

type INetStream interface {
	ReadMsg() (MsgIO, error)
	WriteMsg(m MsgIO) error
	IReadWriter
	io.Closer
}

type NetStream struct {
	len   int    //收发到的数据总数
	bytes []byte //最后收到的数据包
	net.Conn
}

func NewNetStream(conn net.Conn) *NetStream {
	return &NetStream{Conn: conn}
}

func (c *NetStream) Bytes() []byte {
	return c.bytes
}

func (c *NetStream) Reset() {
	c.len = 0
	c.bytes = nil
}

func (c *NetStream) Len() int {
	return c.len
}

func (w *NetStream) WriteFull(dp []byte) error {
	return WriteFull(w, dp)
}

func (r *NetStream) ReadFull(dp []byte) error {
	return ReadFull(r, dp)
}

func (c *NetStream) TRead(data interface{}) error {
	return binary.Read(c, Endian, data)
}

func (c *NetStream) TWrite(data interface{}) error {
	return binary.Write(c, Endian, data)
}

func (c *NetStream) ReadMsg() (MsgIO, error) {
	pd := &NetPackage{}
	err := pd.Decode(c)
	if err != nil {
		return nil, fmt.Errorf("type=%d err=%w", pd.Type, err)
	}
	c.len += pd.Bytes.Len()
	return pd.ToMsgIO()
}

func (c *NetStream) WriteMsg(m MsgIO) error {
	buf := NewWriter()
	if err := m.Encode(buf); err != nil {
		return err
	}
	flags := [4]byte{}
	copy(flags[:], conf.Flags)
	pd := &NetPackage{
		Flags: flags,
		Type:  m.Type(),
		Bytes: buf.Bytes(),
	}
	c.len += buf.Len()
	return pd.Encode(c)
}

func (c *NetStream) ReadByte() (byte, error) {
	b0 := []byte{0}
	_, err := c.Read(b0)
	return b0[0], err
}

func (c *NetStream) WriteByte(b byte) error {
	b0 := []byte{b}
	_, err := c.Write(b0)
	return err
}

type NetPackage struct {
	Flags [4]byte  //标识
	Type  uint8    //包类型
	Bytes VarBytes //数据长度
	Sum   uint32
}

func (v NetPackage) Encode(w IWriter) error {
	if err := w.WriteFull(v.Flags[:]); err != nil {
		return err
	}
	if err := w.WriteByte(v.Type); err != nil {
		return err
	}
	if err := v.Bytes.Encode(w); err != nil {
		return err
	}
	if err := w.TWrite(v.Sum32()); err != nil {
		return err
	}
	return nil
}

func (v *NetPackage) Sum32() uint32 {
	crc := crc32.New(crc32.IEEETable)
	n, err := crc.Write(v.Flags[:])
	if err != nil || n != 4 {
		panic(err)
	}
	n, err = crc.Write([]byte{v.Type})
	if err != nil || n != 1 {
		panic(err)
	}
	n, err = crc.Write(v.Bytes)
	if err != nil || n != len(v.Bytes) {
		panic(err)
	}
	return crc.Sum32()
}

func (v *NetPackage) Decode(r IReader) error {
	var err error
	if err = r.ReadFull(v.Flags[:]); err != nil {
		return err
	}
	if v.Type, err = r.ReadByte(); err != nil {
		return err
	}
	if err = v.Bytes.Decode(r); err != nil {
		return err
	}
	if err = r.TRead(&v.Sum); err != nil {
		return err
	}
	if !bytes.Equal(v.Flags[:], []byte(conf.Flags)) {
		return errors.New("flags not same")
	}
	if v.Sum32() != v.Sum {
		return errors.New("check sum error")
	}
	return nil
}

func WriteFull(w io.Writer, dp []byte) error {
	l := len(dp)
	p := 0
	for l-p > 0 {
		b, err := w.Write(dp[p:])
		if err != nil {
			return err
		}
		p += b
	}
	return nil
}

func ReadFull(r io.Reader, dp []byte) error {
	l := len(dp)
	p := 0
	for l-p > 0 {
		b, err := r.Read(dp[p:])
		if err != nil {
			return err
		}
		p += b
	}
	return nil
}

type reader struct {
	*bytes.Reader
}

func (r *reader) ReadFull(dp []byte) error {
	return ReadFull(r, dp)
}

func (r *reader) TRead(data interface{}) error {
	return binary.Read(r.Reader, Endian, data)
}

type writer struct {
	*bytes.Buffer
}

//
func (w *writer) TWrite(data interface{}) error {
	return binary.Write(w.Buffer, Endian, data)
}

func (w *writer) Len() int {
	return w.Buffer.Len()
}

func (w *writer) Bytes() []byte {
	return w.Buffer.Bytes()
}

func (w *writer) WriteFull(dp []byte) error {
	return WriteFull(w.Buffer, dp)
}

func (w *writer) Reset() {
	w.Buffer.Reset()
}

//
func NewReader(b []byte) IReader {
	return &reader{
		Reader: bytes.NewReader(b),
	}
}

func NewWriter() IWriter {
	return &writer{
		Buffer: &bytes.Buffer{},
	}
}

//
type readwriter struct {
	*bytes.Buffer
}

func (w *readwriter) TWrite(data interface{}) error {
	return binary.Write(w.Buffer, Endian, data)
}

func (r *readwriter) TRead(data interface{}) error {
	return binary.Read(r.Buffer, Endian, data)
}

func (w *readwriter) WriteFull(dp []byte) error {
	return WriteFull(w.Buffer, dp)
}

func (r *readwriter) ReadFull(dp []byte) error {
	return ReadFull(r.Buffer, dp)
}

func NewReadWriter() IReadWriter {
	return &readwriter{
		Buffer: &bytes.Buffer{},
	}
}

type IReader interface {
	io.Reader
	io.ByteReader
	TRead(data interface{}) error
	ReadFull(dp []byte) error
}

type IWriter interface {
	io.Writer
	io.ByteWriter
	TWrite(data interface{}) error
	Len() int
	Bytes() []byte
	Reset()
	WriteFull(dp []byte) error
}

type IReadWriter interface {
	IReader
	IWriter
}

type VarUInt uint64

func (v VarUInt) ToAmount() Amount {
	return Amount(v)
}

func (v VarUInt) ToInt() int {
	return int(v)
}

func (v VarUInt) Bytes() []byte {
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutUvarint(lb, uint64(v))
	return lb[:l]
}

func (v VarUInt) ToUInt32() uint32 {
	return uint32(v)
}

func (v *VarUInt) SetUInt32(uv uint32) {
	*v = VarUInt(uv)
}

func (v *VarUInt) SetInt(uv int) {
	*v = VarUInt(uv)
}

func (v VarUInt) Encode(w IWriter) error {
	_, err := w.Write(v.Bytes())
	return err
}

func (v *VarUInt) From(b []byte) int {
	vv, l := binary.Uvarint(b)
	*v = VarUInt(vv)
	return l
}

func (v *VarUInt) Decode(r IReader) error {
	vv, err := binary.ReadUvarint(r)
	*v = VarUInt(vv)
	return err
}

type VarInt int64

func (v VarInt) ToInt() int {
	return int(v)
}

func (v VarInt) Encode(w IWriter) error {
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutVarint(lb, int64(v))
	_, err := w.Write(lb[:l])
	return err
}

func (v *VarInt) Decode(r IReader) error {
	vv, err := binary.ReadVarint(r)
	*v = VarInt(vv)
	return err
}

type VarBytes []byte

func (v VarBytes) Len() int {
	return len(v)
}

func (v VarBytes) String() string {
	return string(v[:])
}

func (v VarBytes) Equal(b VarBytes) bool {
	return bytes.Equal(v, b)
}

func (v VarBytes) Encode(w IWriter) error {
	l := len(v)
	lb := make([]byte, binary.MaxVarintLen32)
	l = binary.PutUvarint(lb, uint64(l))
	if err := w.TWrite(lb[:l]); err != nil {
		return err
	}
	if len(v) == 0 {
		return nil
	}
	if err := w.WriteFull(v); err != nil {
		return err
	}
	return nil
}

func (v *VarBytes) Decode(r IReader) error {
	l, err := binary.ReadUvarint(r)
	if err != nil {
		return err
	}
	if l == 0 {
		return nil
	}
	if l > 1024*1024*5 {
		return errors.New("bytes length too long")
	}
	*v = make([]byte, l)
	if err := r.ReadFull(*v); err != nil {
		return err
	}
	return nil
}
