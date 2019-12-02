package xginx

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"strconv"
	"time"
)

type NTType uint8

func (t NTType) String() string {
	switch t {
	case NT_VERSION:
		return "NT_VERSION"
	case NT_PING:
		return "NT_PING"
	case NT_PONG:
		return "NT_PONG"
	case NT_GET_ADDRS:
		return "NT_GET_ADDRS"
	case NT_ADDRS:
		return "NT_ADDRS"
	case NT_INV:
		return "NT_INV"
	case NT_GET_INV:
		return "NT_GET_INV"
	case NT_TX:
		return "NT_TX"
	case NT_BLOCK:
		return "NT_BLOCK"
	case NT_GET_BLOCK:
		return "NT_GET_BLOCK"
	case NT_ERROR:
		return "NT_ERROR"
	case NT_ALERT:
		return "NT_ALERT"
	case NT_FILTER_LOAD:
		return "NT_FILTER_LOAD"
	case NT_FILTER_ADD:
		return "NT_FILTER_ADD"
	case NT_FILTER_CLEAR:
		return "NT_FILTER_CLEAR"
	case NT_GET_MERKLE:
		return "NT_GET_MERKLE"
	case NT_TX_MERKLE:
		return "NT_TX_MERKLE"
	case NT_GET_TXPOOL:
		return "NT_GET_TXPOOL"
	case NT_TXPOOL:
		return "NT_TXPOOL"
	case NT_BROAD_PKG:
		return "NT_BROAD_PKG"
	case NT_BROAD_ACK:
		return "NT_BROAD_ACK"
	default:
		return "NT_UNKNOW"
	}
}

//协议包标识
const (
	NT_VERSION = NTType(1)
	//ping/pong
	NT_PING = NTType(2)
	NT_PONG = NTType(3)
	//获取节点连接的其他地址
	NT_GET_ADDRS = NTType(4)
	NT_ADDRS     = NTType(5)
	//inv 交易或者区块通报
	//当有新的交易或者区块生成通报给周边的节点
	NT_INV = NTType(6)
	//获取交易或者区块
	NT_GET_INV = NTType(7)
	//获取交易的返回
	NT_TX = NTType(8)
	//获取区块的返回
	NT_BLOCK = NTType(9)
	//获取区块按高度
	NT_GET_BLOCK = NTType(10)
	//返回一个错误信息
	NT_ERROR = NTType(12)
	//消息通知
	NT_ALERT = NTType(13)
	//过滤器 加载 添加 清除
	NT_FILTER_LOAD  = NTType(14)
	NT_FILTER_ADD   = NTType(15)
	NT_FILTER_CLEAR = NTType(16)
	//交易merkle树
	NT_GET_MERKLE = NTType(17)
	NT_TX_MERKLE  = NTType(18)
	//获取内存交易池
	NT_GET_TXPOOL = NTType(19)
	NT_TXPOOL     = NTType(20)
	//广播包头和响应
	NT_BROAD_PKG = NTType(0xf0)
	NT_BROAD_ACK = NTType(0xf1)
)

type MsgBroadAck struct {
	MsgId MsgId
}

func (e MsgBroadAck) Type() NTType {
	return NT_BROAD_ACK
}

func (m MsgBroadAck) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (e MsgBroadAck) Encode(w IWriter) error {
	return w.WriteFull(e.MsgId[:])
}

func (e *MsgBroadAck) Decode(r IReader) error {
	return r.ReadFull(e.MsgId[:])
}

type MsgBroadPkg struct {
	MsgId MsgId //md5
}

func (m MsgBroadPkg) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (e MsgBroadPkg) Type() NTType {
	return NT_BROAD_PKG
}

func (e MsgBroadPkg) Encode(w IWriter) error {
	return w.WriteFull(e.MsgId[:])
}

func (e *MsgBroadPkg) Decode(r IReader) error {
	return r.ReadFull(e.MsgId[:])
}

var (
	NotIdErr = errors.New("msg not id,can't broad")
	ErrMsgId = MsgId{}
)

//使用md5
type MsgId [md5.Size]byte

func (m MsgId) SendKey() string {
	return "S" + string(m[:])
}

func (m MsgId) RecvKey() string {
	return "R" + string(m[:])
}

//协议消息
type MsgIO interface {
	Id() (MsgId, error) //实现了此方法的包才能进行广播，负责返回 NotIdErr
	Type() NTType
	Encode(w IWriter) error
	Decode(r IReader) error
}

const (
	ErrCodeRecvBlock  = 100001
	ErrCodeRecvTx     = 100002
	ErrCodeFilterMiss = 100003
	ErrCodeFilterLoad = 100004
	ErrCodeTxMerkle   = 100005
	ErrCodeBlockMiss  = 100006
)

type MsgError struct {
	Code  int32    //错误代码
	Error VarBytes //错误信息
	Ext   VarBytes //扩展信息
}

func (m MsgError) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func NewMsgError(code int, err error) *MsgError {
	return &MsgError{
		Code:  int32(code),
		Error: []byte(err.Error()),
	}
}

func (e MsgError) Type() NTType {
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
	Height uint32 //发送我的最新高度
}

func (v MsgPing) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (v MsgPing) Type() NTType {
	return NT_PING
}

func (v MsgPing) NewPong(h uint32) *MsgPong {
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

func NewMsgPing(h uint32) *MsgPing {
	msg := &MsgPing{Time: time.Now().UnixNano()}
	msg.Height = h
	return msg
}

type MsgPong struct {
	Time   int64
	Height uint32 //获取对方的高度
}

func (v MsgPong) Type() NTType {
	return NT_PONG
}

func (v MsgPong) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
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

//版本消息包
type MsgVersion struct {
	Ver     uint32  //版本
	Service uint32  //服务
	Addr    NetAddr //节点外网地址
	Height  uint32  //节点区块高度
	NodeID  uint64  //节点随机id
	Tps     VarUInt //交易池数量
}

//在链上生成一个版本数据包
func (bi *BlockIndex) NewMsgVersion() *MsgVersion {
	m := &MsgVersion{}
	m.Ver = conf.Ver
	m.Addr = conf.GetNetAddr()
	m.Height = bi.BestHeight()
	m.Service = SERVICE_NODE
	m.NodeID = conf.nodeid
	m.Tps = VarUInt(bi.txp.Len())
	return m
}

func (v MsgVersion) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (v MsgVersion) Type() NTType {
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
	if err := w.TWrite(v.Height); err != nil {
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
	if err := r.TRead(&v.Height); err != nil {
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
	Type  NTType   //包类型
	Bytes VarBytes //数据长度
	Sum   uint32
}

func (v NetPackage) Encode(w IWriter) error {
	if err := w.WriteFull(v.Flags[:]); err != nil {
		return err
	}
	if err := w.WriteByte(uint8(v.Type)); err != nil {
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
	n, err = crc.Write([]byte{uint8(v.Type)})
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
	if typ, err := r.ReadByte(); err != nil {
		return err
	} else {
		v.Type = NTType(typ)
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
