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

	"github.com/cxuhua/lzma"
)

//NTType 消息类型
type NTType uint8

func (t NTType) String() string {
	switch t {
	case NtVersion:
		return "NT_VERSION"
	case NtPing:
		return "NT_PING"
	case NtPong:
		return "NT_PONG"
	case NtGetAddrs:
		return "NT_GET_ADDRS"
	case NtAddrs:
		return "NT_ADDRS"
	case NtInv:
		return "NT_INV"
	case NtGetInv:
		return "NT_GET_INV"
	case NtTx:
		return "NT_TX"
	case NtBlock:
		return "NT_BLOCK"
	case NtGetBlock:
		return "NT_GET_BLOCK"
	case NtError:
		return "NT_ERROR"
	case NtAlert:
		return "NT_ALERT"
	case NtFilterLoad:
		return "NT_FILTER_LOAD"
	case NtFilterAdd:
		return "NT_FILTER_ADD"
	case NtFilterClear:
		return "NT_FILTER_CLEAR"
	case NtGetMerkle:
		return "NT_GET_MERKLE"
	case NtTxMerkle:
		return "NT_TX_MERKLE"
	case NtGetTxPool:
		return "NT_GET_TXPOOL"
	case NtTxPool:
		return "NT_TXPOOL"
	case NtBroadPkg:
		return "NT_BROAD_PKG"
	case NtBroadAck:
		return "NT_BROAD_ACK"
	case NtHeaders:
		return "NT_HEADERS"
	default:
		return "NT_UNKNOW"
	}
}

//协议包标识
const (
	NtVersion = NTType(1)
	//ping/pong
	NtPing = NTType(2)
	NtPong = NTType(3)
	//获取节点连接的其他地址
	NtGetAddrs = NTType(4)
	NtAddrs    = NTType(5)
	//inv 交易或者区块通报
	//当有新的交易或者区块生成通报给周边的节点
	NtInv = NTType(6)
	//获取交易或者区块
	NtGetInv = NTType(7)
	//获取交易的返回
	NtTx = NTType(8)
	//获取区块的返回
	NtBlock = NTType(9)
	//获取区块按高度
	NtGetBlock = NTType(10)
	//返回区块头列表
	NtHeaders = NTType(11)
	//返回一个错误信息
	NtError = NTType(12)
	//消息通知
	NtAlert = NTType(13)
	//过滤器 加载 添加 清除
	NtFilterLoad  = NTType(14)
	NtFilterAdd   = NTType(15)
	NtFilterClear = NTType(16)
	//交易merkle树
	NtGetMerkle = NTType(17)
	NtTxMerkle  = NTType(18)
	//获取内存交易池
	NtGetTxPool = NTType(19)
	NtTxPool    = NTType(20)
	//广播定制消息
	NtBroadInfo = NTType(21)
	//广播包头和响应,当广播消息时只发送广播包头，收到包头如果确定无需要收取数据再请求包数据
	NtBroadPkg = NTType(0xf0)
	NtBroadAck = NTType(0xf1)
)

//MsgBroadAck 广播应答
type MsgBroadAck struct {
	MsgID MsgID
}

//Type 消息类型
func (m MsgBroadAck) Type() NTType {
	return NtBroadAck
}

//ID 消息ID
func (m MsgBroadAck) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Encode 编码消息
func (m MsgBroadAck) Encode(w IWriter) error {
	return w.WriteFull(m.MsgID[:])
}

//Decode 解码消息
func (m *MsgBroadAck) Decode(r IReader) error {
	return r.ReadFull(m.MsgID[:])
}

//MsgBroadPkg 广播头
type MsgBroadPkg struct {
	Meta  []byte //自定义的meta数据
	MsgID MsgID  //md5
}

//ID 消息id
func (m MsgBroadPkg) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Type 消息类型
func (m MsgBroadPkg) Type() NTType {
	return NtBroadPkg
}

//Encode 编码消息
func (m MsgBroadPkg) Encode(w IWriter) error {
	err := w.WriteFull(m.Meta)
	if err != nil {
		return err
	}
	err = w.WriteFull(m.MsgID[:])
	if err != nil {
		return err
	}
	return nil
}

//Decode 解码消息
func (m *MsgBroadPkg) Decode(r IReader) error {
	err := r.ReadFull(m.Meta)
	if err != nil {
		return err
	}
	err = r.ReadFull(m.MsgID[:])
	if err != nil {
		return err
	}
	return nil
}

//错误定义
var (
	ErrNotID = errors.New("msg not id,can't broad")
	ErrMsgID = MsgID{}
)

//MsgID 消息ID定义 使用md5
type MsgID [md5.Size]byte

//SendKey 用于发送的key
func (m MsgID) SendKey() string {
	return "S" + string(m[:])
}

//RecvKey 用于接收的key
func (m MsgID) RecvKey() string {
	return "R" + string(m[:])
}

//IMsgMeta 创建一个meta信息
type IMsgMeta interface {
	NewMeta() []byte
}

//MsgIO 协议消息
type MsgIO interface {
	ID() (MsgID, error) //广播id获取
	Type() NTType
	Encode(w IWriter) error
	Decode(r IReader) error
}

//GetDefautMsgID 获取默认msgid
func GetDefautMsgID(m MsgIO) (MsgID, error) {
	w := NewWriter()
	err := m.Encode(w)
	if err != nil {
		return ErrMsgID, err
	}
	return md5.Sum(w.Bytes()), nil
}

//消息错误代码
const (
	ErrCodeRecvBlock  = 100001
	ErrCodeRecvTx     = 100002
	ErrCodeFilterMiss = 100003
	ErrCodeFilterLoad = 100004
	ErrCodeTxMerkle   = 100005
	ErrCodeBlockMiss  = 100006
	ErrCodeHeaders    = 100007
)

//MsgError 错误消息
type MsgError struct {
	Code  int32    //错误代码
	Error VarBytes //错误信息
	Ext   VarBytes //扩展信息
}

//NewMsgError 创建错误消息
func NewMsgError(code int, err error) *MsgError {
	return &MsgError{
		Code:  int32(code),
		Error: []byte(err.Error()),
	}
}

//ID 消息ID
func (m MsgError) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Type 消息类型
func (m MsgError) Type() NTType {
	return NtError
}

//Encode 编码数据
func (m MsgError) Encode(w IWriter) error {
	if err := w.TWrite(m.Code); err != nil {
		return err
	}
	if err := m.Error.Encode(w); err != nil {
		return err
	}
	if err := m.Ext.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码数据
func (m *MsgError) Decode(r IReader) error {
	if err := r.TRead(&m.Code); err != nil {
		return err
	}
	if err := m.Error.Decode(r); err != nil {
		return err
	}
	if err := m.Ext.Decode(r); err != nil {
		return err
	}
	return nil
}

//NetAddr 地址定义
type NetAddr struct {
	ip   net.IP
	port uint16
}

//IP 获取ip
func (c NetAddr) IP() []byte {
	return c.ip.To16()
}

//NetAddrForm 解析地址
func NetAddrForm(s string) NetAddr {
	n := NetAddr{}
	_ = n.From(s)
	return n
}

//From 解析地址
func (c *NetAddr) From(s string) error {
	h, p, err := net.SplitHostPort(s)
	if err != nil {
		return err
	}
	c.ip = net.ParseIP(h)
	i, err := strconv.ParseInt(p, 10, 32)
	if err != nil {
		c.port = DefaultPort
	} else {
		c.port = uint16(i)
	}
	return nil
}

//IsGlobalUnicast 是否是有效的可链接的地址
func (c NetAddr) IsGlobalUnicast() bool {
	return c.ip.IsGlobalUnicast()
}

//Network 网络类型
func (c NetAddr) Network() string {
	return c.ToTCPAddr().Network()
}

//ToTCPAddr 转换为Tcp结构
func (c NetAddr) ToTCPAddr() *net.TCPAddr {
	return &net.TCPAddr{
		IP:   c.ip,
		Port: int(c.port),
	}
}

//Equal ==
func (c NetAddr) Equal(d NetAddr) bool {
	return c.ip.Equal(d.ip) && c.port == d.port
}

func (c NetAddr) String() string {
	return c.Addr()
}

//Addr 获取地址
func (c NetAddr) Addr(h ...string) string {
	if len(h) > 0 {
		return net.JoinHostPort(h[0], fmt.Sprintf("%d", c.port))
	}
	return net.JoinHostPort(c.ip.String(), fmt.Sprintf("%d", c.port))
}

//Encode 编码网络地址
func (c NetAddr) Encode(w IWriter) error {
	b := c.ip.To16()
	if _, err := w.Write(b); err != nil {
		return err
	}
	if err := w.TWrite(c.port); err != nil {
		return err
	}
	return nil
}

//Decode 解码网络地址
func (c *NetAddr) Decode(r IReader) error {
	ip6 := make([]byte, net.IPv6len)
	if _, err := r.Read(ip6); err != nil {
		return err
	}
	c.ip = ip6
	if err := r.TRead(&c.port); err != nil {
		return err
	}
	return nil
}

//MsgPing ping消息
type MsgPing struct {
	Time   int64
	Height uint32 //发送我的最新高度
}

//ID 消息ID
func (v MsgPing) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Type 消息类型
func (v MsgPing) Type() NTType {
	return NtPing
}

//NewPong 收到ping时创建pong消息返回
func (v MsgPing) NewPong(h uint32) *MsgPong {
	msg := &MsgPong{Time: v.Time}
	msg.Height = h
	return msg
}

//Encode 编码ping消息
func (v MsgPing) Encode(w IWriter) error {
	if err := w.TWrite(v.Time); err != nil {
		return err
	}
	if err := w.TWrite(v.Height); err != nil {
		return err
	}
	return nil
}

//Decode 解码ping消息
func (v *MsgPing) Decode(r IReader) error {
	if err := r.TRead(&v.Time); err != nil {
		return err
	}
	if err := r.TRead(&v.Height); err != nil {
		return err
	}
	return nil
}

//NewMsgPing 创建ping消息
func NewMsgPing(h uint32) *MsgPing {
	msg := &MsgPing{Time: time.Now().UnixNano()}
	msg.Height = h
	return msg
}

//MsgPong 收到ping消息返回
type MsgPong struct {
	Time   int64
	Height uint32 //获取对方的高度
}

//Type 消息类型
func (v MsgPong) Type() NTType {
	return NtPong
}

//ID 消息id
func (v MsgPong) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Ping 获取ping值
func (v MsgPong) Ping() int {
	return int((time.Now().UnixNano() - v.Time) / 1000000)
}

//Encode 编码
func (v MsgPong) Encode(w IWriter) error {
	if err := w.TWrite(v.Time); err != nil {
		return err
	}
	if err := w.TWrite(v.Height); err != nil {
		return err
	}
	return nil
}

//Decode 解码
func (v *MsgPong) Decode(r IReader) error {
	if err := r.TRead(&v.Time); err != nil {
		return err
	}
	if err := r.TRead(&v.Height); err != nil {
		return err
	}
	return nil
}

//节点类型定义
const (
	//全节点
	FullNodeFlag = 1 << 0
)

//MsgVersion 版本消息包
type MsgVersion struct {
	Ver     uint32  //版本
	Service uint32  //服务
	Addr    NetAddr //节点外网地址
	Height  uint32  //节点区块高度
	NodeID  uint64  //节点id
	Tps     VarUInt //交易池数量
}

//NewMsgVersion 在链上生成一个版本数据包
func (bi *BlockIndex) NewMsgVersion() *MsgVersion {
	m := &MsgVersion{}
	m.Ver = conf.Ver
	m.Addr = conf.GetNetAddr()
	m.Height = bi.BestHeight()
	m.Service = FullNodeFlag
	m.NodeID = conf.nodeid
	m.Tps = VarUInt(bi.txp.Len())
	return m
}

//ID 消息ID
func (v MsgVersion) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Type 消息类型
func (v MsgVersion) Type() NTType {
	return NtVersion
}

//Encode 编码
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

//Decode 解码数据
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

//INetStream 网络流接口
type INetStream interface {
	ReadMsg() (MsgIO, error)
	WriteMsg(m MsgIO) error
	IReadWriter
	io.Closer
}

//NetStream 网络读写流
type NetStream struct {
	len   int    //收发到的数据总数
	bytes []byte //最后收到的数据包
	net.Conn
}

//NewNetStream 从网络连接创建网络流
func NewNetStream(conn net.Conn) *NetStream {
	return &NetStream{Conn: conn}
}

//Bytes 最后一个数据包数据
func (s *NetStream) Bytes() []byte {
	return s.bytes
}

//Reset 重置数据流状态
func (s *NetStream) Reset() {
	s.len = 0
	s.bytes = nil
}

//Len 读写的数据长度
func (s *NetStream) Len() int {
	return s.len
}

//WriteFull 完整写入
func (s *NetStream) WriteFull(dp []byte) error {
	return WriteFull(s, dp)
}

//ReadFull 完整读取
func (s *NetStream) ReadFull(dp []byte) error {
	return ReadFull(s, dp)
}

//TRead 按类型读
func (s *NetStream) TRead(data interface{}) error {
	return binary.Read(s, Endian, data)
}

//TWrite 按类型写入
func (s *NetStream) TWrite(data interface{}) error {
	return binary.Write(s, Endian, data)
}

//ReadMsg 从网络流读取数据包
func (s *NetStream) ReadMsg(attr ...*uint8) (MsgIO, error) {
	pd := &NetPackage{}
	err := pd.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("type=%d err=%w", pd.Type, err)
	}
	s.bytes = pd.Bytes
	s.len += pd.Bytes.Len()
	//读取数据包时可返回属性
	if len(attr) > 0 && attr[0] != nil {
		*attr[0] = pd.Attr
	}
	return pd.ToMsgIO()
}

//WriteMsg 写消息到网路
func (s *NetStream) WriteMsg(m MsgIO, attrs ...uint8) error {
	buf := NewWriter()
	if err := m.Encode(buf); err != nil {
		return err
	}
	attr := uint8(0)
	for _, av := range attrs {
		attr |= av
	}
	// >1k 的数据启用压缩
	if buf.Len() >= 1024 {
		attr |= PackageAttrZip
	}
	pd := &NetPackage{
		Flags: conf.flags,
		Type:  m.Type(),
		Attr:  attr,
		Bytes: buf.Bytes(),
	}
	err := pd.Encode(s)
	if err == nil {
		s.len += buf.Len()
		s.bytes = buf.Bytes()
	}
	return err
}

//ReadByte 读取一个字节
func (s *NetStream) ReadByte() (byte, error) {
	b0 := []byte{0}
	_, err := s.Read(b0)
	return b0[0], err
}

//WriteByte 写入一个字节
func (s *NetStream) WriteByte(b byte) error {
	b0 := []byte{b}
	_, err := s.Write(b0)
	return err
}

//数据包属性
const (
	//PackageAttrZip 数据是否启用压缩
	PackageAttrZip = uint8(1 << 0)
)

//NetPackage 网络数据包定义
type NetPackage struct {
	Flags [4]byte  //标识
	Type  NTType   //包类型
	Bytes VarBytes //数据长度
	Attr  uint8    //数据特性
	Sum   uint32   //校验和
}

//IsZip 是否启用压缩, Attr 设置后可用
func (v NetPackage) IsZip() bool {
	return v.Attr&PackageAttrZip != 0
}

//Encode 编码网络数据包
func (v NetPackage) Encode(w IWriter) error {
	//flags
	if err := w.WriteFull(v.Flags[:]); err != nil {
		return err
	}
	//type
	if err := w.WriteByte(uint8(v.Type)); err != nil {
		return err
	}
	//attr
	if err := w.WriteByte(v.Attr); err != nil {
		return err
	}
	//bytes
	var err error
	if v.IsZip() {
		err = v.Bytes.Compress(w)
	} else {
		err = v.Bytes.Encode(w)
	}
	if err != nil {
		return err
	}
	//sum
	if err := w.TWrite(v.Sum32()); err != nil {
		return err
	}
	return nil
}

//Sum32 计算校验和
func (v *NetPackage) Sum32() uint32 {
	crc := crc32.New(crc32.IEEETable)
	n, err := crc.Write(v.Flags[:])
	if err != nil || n != 4 {
		panic(err)
	}
	n, err = crc.Write([]byte{uint8(v.Type), v.Attr})
	if err != nil || n != 2 {
		panic(err)
	}
	n, err = crc.Write(v.Bytes)
	if err != nil || n != len(v.Bytes) {
		panic(err)
	}
	return crc.Sum32()
}

//Decode 解码网络数据包
func (v *NetPackage) Decode(r IReader) error {
	var err error
	//flags
	if err = r.ReadFull(v.Flags[:]); err != nil {
		return err
	}
	if !bytes.Equal(v.Flags[:], conf.flags[:]) {
		return errors.New("flags error")
	}
	//type
	typ, err := r.ReadByte()
	if err != nil {
		return err
	}
	v.Type = NTType(typ)
	//attr
	attr, err := r.ReadByte()
	if err != nil {
		return err
	}
	//bytes
	v.Attr = attr
	if v.IsZip() {
		err = v.Bytes.Uncompress(r)
	} else {
		err = v.Bytes.Decode(r)
	}
	if err != nil {
		return err
	}
	//sum
	if err = r.TRead(&v.Sum); err != nil {
		return err
	}
	if v.Sum32() != v.Sum {
		return errors.New("check sum error")
	}
	return nil
}

//WriteFull 写入完整数据
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

//ReadFull 从流读取完整数据
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

//NewReader 从二进制创建读取流
func NewReader(b []byte) IReader {
	return &reader{
		Reader: bytes.NewReader(b),
	}
}

//NewWriter 创建写入流
func NewWriter() IWriter {
	return &writer{
		Buffer: &bytes.Buffer{},
	}
}

//
type readwriter struct {
	*bytes.Buffer
}

//TWrite 按类型写
func (rw *readwriter) TWrite(data interface{}) error {
	return binary.Write(rw.Buffer, Endian, data)
}

//TRead 按类型读
func (rw *readwriter) TRead(data interface{}) error {
	return binary.Read(rw.Buffer, Endian, data)
}

//WriteFull 写入长度为len(dp)的数据
func (rw *readwriter) WriteFull(dp []byte) error {
	return WriteFull(rw.Buffer, dp)
}

//ReadFull 读取长度位dp的数据
func (rw *readwriter) ReadFull(dp []byte) error {
	return ReadFull(rw.Buffer, dp)
}

//NewReadWriter 使用二进制buf创建读写流
func NewReadWriter() IReadWriter {
	return &readwriter{
		Buffer: &bytes.Buffer{},
	}
}

//IReader 数据流读接口
type IReader interface {
	io.Reader
	io.ByteReader
	TRead(data interface{}) error
	ReadFull(dp []byte) error
}

//IWriter 数据流写接口
type IWriter interface {
	io.Writer
	io.ByteWriter
	TWrite(data interface{}) error
	Len() int
	Bytes() []byte
	Reset()
	WriteFull(dp []byte) error
}

//IReadWriter 数据流读写接口
type IReadWriter interface {
	IReader
	IWriter
}

//VarUInt 可变整形（无符号)
type VarUInt uint64

//ToAmount 强转为金额类型
func (v VarUInt) ToAmount() Amount {
	return Amount(v)
}

//ToInt 强转为Int类型
func (v VarUInt) ToInt() int {
	return int(v)
}

//Bytes 获取二进制数据
func (v VarUInt) Bytes() []byte {
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutUvarint(lb, uint64(v))
	return lb[:l]
}

//ToUInt32 强转为无符号32位整形
func (v VarUInt) ToUInt32() uint32 {
	return uint32(v)
}

//SetUInt32 使用uint32初始化
func (v *VarUInt) SetUInt32(uv uint32) {
	*v = VarUInt(uv)
}

//SetInt 使用int初始化
func (v *VarUInt) SetInt(uv int) {
	*v = VarUInt(uv)
}

//Encode 编码
func (v VarUInt) Encode(w IWriter) error {
	_, err := w.Write(v.Bytes())
	return err
}

//From 从二进制初始化
func (v *VarUInt) From(b []byte) int {
	vv, l := binary.Uvarint(b)
	*v = VarUInt(vv)
	return l
}

//Decode 解码
func (v *VarUInt) Decode(r IReader) error {
	vv, err := binary.ReadUvarint(r)
	*v = VarUInt(vv)
	return err
}

//VarInt 可变整形
type VarInt int64

//ToInt 强转位Int类型
func (v VarInt) ToInt() int {
	return int(v)
}

//Encode 编码可变整形
func (v VarInt) Encode(w IWriter) error {
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutVarint(lb, int64(v))
	_, err := w.Write(lb[:l])
	return err
}

//Decode 解码可变整形
func (v *VarInt) Decode(r IReader) error {
	vv, err := binary.ReadVarint(r)
	*v = VarInt(vv)
	return err
}

//VarBytes 可变数据定义
type VarBytes []byte

//Len 获取数据长度
func (v VarBytes) Len() int {
	return len(v)
}

//String 转为字符串
func (v VarBytes) String() string {
	return string(v[:])
}

//Equal 数据是否相同
func (v VarBytes) Equal(b VarBytes) bool {
	return bytes.Equal(v, b)
}

//Compress 压缩编码数据
func (v VarBytes) Compress(w IWriter) error {
	zv, err := lzma.Compress(v)
	if err != nil {
		return err
	}
	return VarBytes(zv).Encode(w)
}

//Uncompress 解压解码解码可变数据
func (v *VarBytes) Uncompress(r IReader) error {
	zv := VarBytes{}
	err := zv.Decode(r)
	if err != nil {
		return err
	}
	vv, err := lzma.Uncompress(zv)
	if err != nil {
		return err
	}
	*v = vv
	return nil
}

//Encode 编码可变数据
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

//Decode 解码可变数据
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
