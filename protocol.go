package xginx

import (
	"bytes"
	sha2562 "crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

//协议包标识
const (
	NT_VERSION   = uint8(1)
	NT_PING      = uint8(2)
	NT_PONG      = uint8(3)
	NT_GET_ADDRS = uint8(4)
	NT_ADDRS     = uint8(5)
)

//协议消息
type MsgIO interface {
	Type() uint8
	Encode(w IWriter) error
	Decode(r IReader) error
}

type EmptyMsg struct {
}

func (e EmptyMsg) Type() uint8 {
	return 0
}

func (e EmptyMsg) Encode(w IWriter) error {
	return nil
}

func (e EmptyMsg) Decode(r IReader) error {
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

func (c NetAddr) ToTcpAddr() *net.TCPAddr {
	return &net.TCPAddr{
		IP:   c.ip,
		Port: int(c.port),
	}
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
	if err := binary.Write(w, Endian, v.port); err != nil {
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
	if err := binary.Read(r, Endian, &v.port); err != nil {
		return err
	}
	return nil
}

type MsgPing struct {
	Time int64
}

func (v MsgPing) Type() uint8 {
	return NT_PING
}

func (v MsgPing) NewPong() *MsgPong {
	return &MsgPong{Time: v.Time}
}

func (v MsgPing) Encode(w IWriter) error {
	return binary.Write(w, Endian, v.Time)
}

func (v *MsgPing) Decode(r IReader) error {
	return binary.Read(r, Endian, &v.Time)
}

func NewMsgPing() *MsgPing {
	return &MsgPing{Time: time.Now().UnixNano()}
}

type MsgPong struct {
	Time int64
}

func (v MsgPong) Type() uint8 {
	return NT_PONG
}

func (v MsgPong) Ping() int {
	return int((time.Now().UnixNano() - v.Time) / 1000000)
}

func (v MsgPong) Encode(w IWriter) error {
	return binary.Write(w, Endian, v.Time)
}

func (v *MsgPong) Decode(r IReader) error {
	return binary.Read(r, Endian, &v.Time)
}

const (
	SERVICE_NODE = 1 << 0
)

//版本消息包
type MsgVersion struct {
	MsgIO
	Ver     uint32  //版本
	NodeID  HASH160 //节点id
	Service uint32  //服务
	Addr    NetAddr //节点地址
}

func NewMsgVersion() *MsgVersion {
	m := &MsgVersion{}
	m.NodeID = conf.NodeID
	m.Ver = conf.Ver
	m.Addr = conf.GetNetAddr()
	return m
}

func (v MsgVersion) Type() uint8 {
	return NT_VERSION
}

func (v MsgVersion) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, v.Ver); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, v.NodeID); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, v.Service); err != nil {
		return err
	}
	if err := v.Addr.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *MsgVersion) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &v.Ver); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.NodeID); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.Service); err != nil {
		return err
	}
	if err := v.Addr.Decode(r); err != nil {
		return err
	}
	return nil
}

type NetStream struct {
	net.Conn
}

func (c *NetStream) ReadMsg() (MsgIO, error) {
	pd := &NetPackage{}
	err := pd.Decode(c)
	if err != nil {
		return nil, err
	}
	return pd.ToMsgIO()
}

func (c *NetStream) WriteMsg(m MsgIO) error {
	buf := &bytes.Buffer{}
	if err := m.Encode(buf); err != nil {
		return err
	}
	flags := [4]byte{}
	copy(flags[:], conf.Flags)
	pd := &NetPackage{
		Flags: flags,
		Type:  m.Type(),
		Ver:   conf.Ver,
		Bytes: buf.Bytes(),
	}
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
	Ver   uint32   //版本
	Bytes VarBytes //数据长度
	Sum   [4]byte  //校验和HASH256 前4字节
}

func (v NetPackage) Encode(w IWriter) error {
	if _, err := w.Write(v.Flags[:]); err != nil {
		return err
	}
	if err := w.WriteByte(v.Type); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, v.Ver); err != nil {
		return err
	}
	if err := v.Bytes.Encode(w); err != nil {
		return err
	}
	if _, err := w.Write(v.Hash()); err != nil {
		return err
	}
	return nil
}

func (v *NetPackage) Hash() []byte {
	hasher := sha2562.New()
	hasher.Write(v.Flags[:])
	hasher.Write([]byte{v.Type})
	_ = binary.Write(hasher, Endian, v.Ver)
	hasher.Write(v.Bytes)
	sum := hasher.Sum(nil)
	return sum[:4]
}

func (v *NetPackage) Decode(r IReader) error {
	if _, err := r.Read(v.Flags[:]); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.Type); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.Ver); err != nil {
		return err
	}
	if err := v.Bytes.Decode(r); err != nil {
		return err
	}
	if _, err := r.Read(v.Sum[:]); err != nil {
		return err
	}
	if !bytes.Equal(v.Sum[:], v.Hash()) {
		return errors.New("check sum error")
	}
	return nil
}

type IReader interface {
	io.Reader
	io.ByteReader
}

type IWriter interface {
	io.Writer
	io.ByteWriter
}

type Stream interface {
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
	return binary.Write(w, Endian, v.Bytes())
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
	return binary.Write(w, Endian, lb[:l])
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

func (v VarBytes) Equal(b VarBytes) bool {
	return bytes.Equal(v, b)
}

func (v VarBytes) Encode(w IWriter) error {
	l := len(v)
	lb := make([]byte, binary.MaxVarintLen32)
	l = binary.PutUvarint(lb, uint64(l))
	if err := binary.Write(w, Endian, lb[:l]); err != nil {
		return err
	}
	if len(v) == 0 {
		return nil
	}
	if err := binary.Write(w, Endian, v); err != nil {
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
	if l > 1024*1024*4 {
		return errors.New("bytes length too long")
	}
	*v = make([]byte, l)
	if _, err := r.Read(*v); err != nil {
		return err
	}
	return nil
}
