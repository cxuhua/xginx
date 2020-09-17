package xginx

import (
	"crypto/md5"
	"errors"
	"fmt"
)

//ToMsgIO 转为类型
func (v NetPackage) ToMsgIO() (MsgIO, error) {
	var m MsgIO = nil
	buf := NewReader(v.Bytes)
	switch v.Type {
	case NtVersion:
		m = &MsgVersion{}
	case NtPing:
		m = &MsgPing{}
	case NtPong:
		m = &MsgPong{}
	case NtGetAddrs:
		m = &MsgGetAddrs{}
	case NtAddrs:
		m = &MsgAddrs{}
	case NtInv:
		m = &MsgInv{}
	case NtTx:
		m = &MsgTx{}
	case NtBlock:
		m = &MsgBlock{}
	case NtGetInv:
		m = &MsgGetInv{}
	case NtGetBlock:
		m = &MsgGetBlock{}
	case NtHeaders:
		m = &MsgHeaders{}
	case NtError:
		m = &MsgError{}
	case NtAlert:
		m = &MsgAlert{}
	case NtFilterClear:
		m = &MsgFilterClear{}
	case NtFilterAdd:
		m = &MsgFilterAdd{}
	case NtFilterLoad:
		m = &MsgFilterLoad{}
	case NtGetMerkle:
		m = &MsgGetMerkle{}
	case NtTxMerkle:
		m = &MsgTxMerkle{}
	case NtGetTxPool:
		m = &MsgGetTxPool{}
	case NtTxPool:
		m = &MsgTxPool{}
	case NtBroadAck:
		m = &MsgBroadAck{}
	case NtBroadPkg:
		m = &MsgBroadPkg{}
	}
	if m == nil {
		return nil, fmt.Errorf("message not create instance type=%v", v.Type)
	}
	if err := m.Decode(buf); err != nil {
		return nil, fmt.Errorf("message type=%v decode error %w", v.Type, err)
	}
	return m, nil
}

//MsgGetAddrs 获取节点记录的其他节点地址
type MsgGetAddrs struct {
}

//ID 返回消息id
func (m MsgGetAddrs) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Encode 编码消息
func (m MsgGetAddrs) Encode(w IWriter) error {
	return nil
}

//Decode 解码消息
func (m *MsgGetAddrs) Decode(r IReader) error {
	return nil
}

//Type 获取消息类型
func (m MsgGetAddrs) Type() NTType {
	return NtGetAddrs
}

//MsgAlert 消息广播
type MsgAlert struct {
	Msg VarBytes
	Sig VarBytes //消息签名可验证消息来自哪里
}

//NewMsgAlert 创建消息
func NewMsgAlert(msg string, sig *SigValue) *MsgAlert {
	m := &MsgAlert{}
	m.Msg = []byte(msg)
	if sig != nil {
		sigs := sig.GetSigs()
		m.Sig = sigs[:]
	}
	return m
}

//ID 消息ID
func (m MsgAlert) ID() (MsgID, error) {
	return md5.Sum(m.Msg), nil
}

//Verify 验证消息来源
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

//Type 消息类型
func (m MsgAlert) Type() NTType {
	return NtAlert
}

//Encode 编码消息
func (m MsgAlert) Encode(w IWriter) error {
	if err := m.Msg.Encode(w); err != nil {
		return err
	}
	if err := m.Sig.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码消息
func (m *MsgAlert) Decode(r IReader) error {
	if err := m.Msg.Decode(r); err != nil {
		return err
	}
	if err := m.Sig.Decode(r); err != nil {
		return err
	}
	return nil
}

//MsgAddrs 返回地址
type MsgAddrs struct {
	Addrs []NetAddr
}

//Type 消息类型
func (m MsgAddrs) Type() NTType {
	return NtAddrs
}

//ID 消息ID
func (m MsgAddrs) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Encode 编码消息
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

//Add 最多放2000个
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

//Decode 解码消息
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
