package xginx

import (
	"bytes"
	"fmt"
)

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
	case NT_GET_ADDRS:
		m = &MsgGetAddrs{}
	case NT_ADDRS:
		m = &MsgAddrs{}
	}
	if m == nil {
		return nil, fmt.Errorf("message not create instance type=%d", v.Type)
	}
	if err := m.Decode(buf); err != nil {
		return nil, fmt.Errorf("message type=%d decode error %w", v.Type, err)
	}
	return m, nil
}

type MsgGetAddrs struct {
	EmptyMsg
}

func (m MsgGetAddrs) Type() uint8 {
	return NT_GET_ADDRS
}

type MsgAddrs struct {
	Addrs []NetAddr
}

func (m MsgAddrs) Type() uint8 {
	return NT_ADDRS
}

func (m MsgAddrs) Encode(w IWriter) error {
	if err := VarInt(len(m.Addrs)).Encode(w); err != nil {
		return err
	}
	for _, v := range m.Addrs {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

//最多放2000个
func (m *MsgAddrs) Add(a NetAddr) {
	if len(m.Addrs) > 2000 {
		return
	}
	m.Addrs = append(m.Addrs, a)
}

func (m *MsgAddrs) Decode(r IReader) error {
	num := VarInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Addrs = make([]NetAddr, num)
	for i := 0; i < num.ToInt(); i++ {
		err := m.Addrs[i].Decode(r)
		if err != nil {
			return err
		}
	}
	return nil
}
