package xginx

import (
	"bytes"
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
	NT_VERSION = uint8(1)
	NT_PING    = uint8(2)
	NT_PONG    = uint8(3)
)

//协议消息
type MsgIO interface {
	Type() uint8
	Encode(w IWriter) error
	Decode(r IReader) error
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

//版本消息包
type MsgVersion struct {
	MsgIO
	Ver   uint32   //版本
	Addr  NetAddr  //节点地址
	Certs VarBytes //节点证书
	Hash  HashID   //节点版本hash
}

func NewMsgVersion() *MsgVersion {
	certs, err := conf.EncodeCerts()
	if err != nil {
		panic(err)
	}
	m := &MsgVersion{}
	m.Ver = conf.Ver
	m.Certs = certs
	m.Addr = conf.GetNetAddr()
	m.Hash = conf.VerHash()
	return m
}

func (v MsgVersion) Type() uint8 {
	return NT_VERSION
}

func (v MsgVersion) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, v.Ver); err != nil {
		return err
	}
	if err := v.Addr.Encode(w); err != nil {
		return err
	}
	if err := v.Certs.Encode(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, v.Hash); err != nil {
		return err
	}
	return nil
}

func (v *MsgVersion) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &v.Ver); err != nil {
		return err
	}
	if err := v.Addr.Decode(r); err != nil {
		return err
	}
	if err := v.Certs.Decode(r); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.Hash); err != nil {
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
	pd := &NetPackage{
		Type:  m.Type(),
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
	Type  uint8    //包类型
	Bytes VarBytes //数据长度
	Sum   [4]byte  //校验和hash256 前4字节
}

func (v NetPackage) ToMsgIO() (MsgIO, error) {
	var m MsgIO = nil
	buf := bytes.NewReader(v.Bytes)
	switch v.Type {
	case NT_VERSION:
		m = &MsgVersion{}
	case NT_PING:
		m = &MsgPing{}
	case NT_PONG:
		m = &MsgPong{}
	}
	if m == nil {
		return nil, fmt.Errorf("message not create instance type=%d", v.Type)
	}
	if err := m.Decode(buf); err != nil {
		return nil, fmt.Errorf("message type=%d decode error %w", v.Type, err)
	}
	return m, nil
}

func (v NetPackage) Encode(w IWriter) error {
	if err := w.WriteByte(v.Type); err != nil {
		return err
	}
	if err := v.Bytes.Encode(w); err != nil {
		return err
	}
	b := append([]byte{v.Type}, v.Bytes...)
	if _, err := w.Write(HASH256P4(b)); err != nil {
		return err
	}
	return nil
}

func (v *NetPackage) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &v.Type); err != nil {
		return err
	}
	if err := v.Bytes.Decode(r); err != nil {
		return err
	}
	if _, err := r.Read(v.Sum[:]); err != nil {
		return err
	}
	b := append([]byte{v.Type}, v.Bytes...)
	if !bytes.Equal(v.Sum[:], HASH256P4(b)) {
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

type VarBytes []byte

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
	if l > 1024*1024*4 {
		return errors.New("bytes length too long")
	}
	*v = make([]byte, l)
	if _, err := r.Read(*v); err != nil {
		return err
	}
	return nil
}
