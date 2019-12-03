package xginx

import (
	"crypto/md5"
	"errors"
	"fmt"
)

func (v NetPackage) ToMsgIO() (MsgIO, error) {
	var m MsgIO = nil
	buf := NewReader(v.Bytes)
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
	case NT_INV:
		m = &MsgInv{}
	case NT_TX:
		m = &MsgTx{}
	case NT_BLOCK:
		m = &MsgBlock{}
	case NT_GET_INV:
		m = &MsgGetInv{}
	case NT_GET_BLOCK:
		m = &MsgGetBlock{}
	case NT_HEADERS:
		m = &MsgHeaders{}
	case NT_ERROR:
		m = &MsgError{}
	case NT_ALERT:
		m = &MsgAlert{}
	case NT_FILTER_CLEAR:
		m = &MsgFilterClear{}
	case NT_FILTER_ADD:
		m = &MsgFilterAdd{}
	case NT_FILTER_LOAD:
		m = &MsgFilterLoad{}
	case NT_GET_MERKLE:
		m = &MsgGetMerkle{}
	case NT_TX_MERKLE:
		m = &MsgTxMerkle{}
	case NT_GET_TXPOOL:
		m = &MsgGetTxPool{}
	case NT_TXPOOL:
		m = &MsgTxPool{}
	case NT_BROAD_ACK:
		m = &MsgBroadAck{}
	case NT_BROAD_PKG:
		m = &MsgBroadPkg{}
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
}

func (e MsgGetAddrs) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (e MsgGetAddrs) Encode(w IWriter) error {
	return nil
}

func (e *MsgGetAddrs) Decode(r IReader) error {
	return nil
}

func (m MsgGetAddrs) Type() NTType {
	return NT_GET_ADDRS
}

//消息广播
type MsgAlert struct {
	Msg VarBytes //消息内容
	Sig VarBytes //消息签名可验证消息来自哪里
}

func NewMsgAlert(msg string, sig *SigValue) *MsgAlert {
	m := &MsgAlert{}
	m.Msg = []byte(msg)
	if sig != nil {
		sigs := sig.GetSigs()
		m.Sig = sigs[:]
	}
	return m
}

func (m MsgAlert) Id() (MsgId, error) {
	return md5.Sum(m.Msg), nil
}

//验证消息来源
func (m MsgAlert) Verify(pub *PublicKey) error {
	hv := Hash256(m.Msg)
	if m.Sig.Len() == 0 {
		return errors.New("miss sig")
	}
	sig, err := NewSigValue(m.Sig[:])
	if err != nil {
		return err
	}
	if !pub.Verify(hv, sig) {
		return errors.New("verify sig error")
	}
	return nil
}

func (m MsgAlert) Type() NTType {
	return NT_ALERT
}

func (m MsgAlert) Encode(w IWriter) error {
	if err := m.Msg.Encode(w); err != nil {
		return err
	}
	if err := m.Sig.Encode(w); err != nil {
		return err
	}
	return nil
}

func (m *MsgAlert) Decode(r IReader) error {
	if err := m.Msg.Decode(r); err != nil {
		return err
	}
	if err := m.Sig.Decode(r); err != nil {
		return err
	}
	return nil
}

//
type MsgAddrs struct {
	Addrs []NetAddr
}

func (m MsgAddrs) Type() NTType {
	return NT_ADDRS
}

func (m MsgAddrs) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
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
func (m *MsgAddrs) Add(a NetAddr) bool {
	if !a.IsGlobalUnicast() {
		return false
	}
	if len(m.Addrs) > 2000 {
		return true
	}
	m.Addrs = append(m.Addrs, a)
	return false
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
