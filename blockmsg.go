package xginx

import (
	"crypto/md5"
	"errors"
)

//MsgGetBlock 获取区块消息
type MsgGetBlock struct {
	Last  HASH256
	Next  uint32
	Count uint32 //获取数量
}

//MsgHeaders 获取区块头网络结构
type MsgHeaders struct {
	Headers Headers
	//上次请求参数
	Info MsgGetBlock
}

//ID 获取消息ID
func (m MsgHeaders) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Type 消息类型
func (m MsgHeaders) Type() NTType {
	return NtHeaders
}

//Encode 编码
func (m MsgHeaders) Encode(w IWriter) error {
	if err := m.Headers.Encode(w); err != nil {
		return err
	}
	if err := m.Info.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码
func (m *MsgHeaders) Decode(r IReader) error {
	if err := m.Headers.Decode(r); err != nil {
		return err
	}
	if err := m.Info.Decode(r); err != nil {
		return err
	}
	return nil
}

//ID 消息ID
func (m MsgGetBlock) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Type 消息类型
func (m MsgGetBlock) Type() NTType {
	return NtGetBlock
}

//Encode 编码消息
func (m MsgGetBlock) Encode(w IWriter) error {
	if err := m.Last.Encode(w); err != nil {
		return err
	}
	if err := w.TWrite(m.Next); err != nil {
		return err
	}
	if err := w.TWrite(m.Count); err != nil {
		return err
	}
	return nil
}

//Decode 解码消息
func (m *MsgGetBlock) Decode(r IReader) error {
	if err := m.Last.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&m.Next); err != nil {
		return err
	}
	if err := r.TRead(&m.Count); err != nil {
		return err
	}
	return nil
}

//GetMsgBlock 获取区块数据返回
func (bi *BlockIndex) GetMsgBlock(id HASH256) (*MsgBlock, error) {
	blk, err := bi.LoadBlock(id)
	if err != nil {
		return nil, err
	}
	return &MsgBlock{Blk: blk}, nil
}

//区块消息标记
const (
	//如果是新出的区块设置此标记并广播
	MsgBlockNewFlags = 1 << 0
	//使用Bytes原始字节打包传输
	MsgBlockUseBytes = 1 << 1
	//使用Blk对象打包传输
	MsgBlockUseBlk = 1 << 2
)

//MsgBlock 区块消息结构
type MsgBlock struct {
	Flags uint8
	Blk   *BlockInfo
	Bytes VarBytes
}

//NewMsgBlock 从区块信息创建消息
func NewMsgBlock(blk *BlockInfo) *MsgBlock {
	m := &MsgBlock{Blk: blk}
	m.AddFlags(MsgBlockUseBlk)
	return m
}

//NewMsgBlockBytes 从区块数据创建消息
func NewMsgBlockBytes(b []byte) *MsgBlock {
	m := &MsgBlock{Bytes: b}
	m.AddFlags(MsgBlockUseBytes)
	return m
}

//ID 消息ID
func (m MsgBlock) ID() (MsgID, error) {
	bid, err := m.Blk.ID()
	if err != nil {
		return ErrMsgID, err
	}
	return md5.Sum(bid[:]), nil
}

//AddFlags 添加消息标记
func (m *MsgBlock) AddFlags(f uint8) {
	m.Flags |= f
}

//Type 获取消息类型
func (m MsgBlock) Type() NTType {
	return NtBlock
}

//IsUseBytes 是否存储的是原始数据
func (m MsgBlock) IsUseBytes() bool {
	return m.Flags&MsgBlockUseBytes != 0
}

//IsUseBlk 是否存储的是消息结构数据
func (m MsgBlock) IsUseBlk() bool {
	return m.Flags&MsgBlockUseBlk != 0
}

//IsNewBlock  是否是新的区块
func (m MsgBlock) IsNewBlock() bool {
	return m.Flags&MsgBlockNewFlags != 0
}

//Encode 编码
func (m MsgBlock) Encode(w IWriter) error {
	if err := w.TWrite(m.Flags); err != nil {
		return err
	}
	if m.IsUseBlk() {
		if m.Blk == nil {
			return errors.New("blk nil")
		}
		if err := m.Blk.Encode(w); err != nil {
			return err
		}
	} else if m.IsUseBytes() {
		if err := m.Bytes.Encode(w); err != nil {
			return err
		}
	} else {
		return errors.New("miss data")
	}
	return nil
}

//Decode 解码
func (m *MsgBlock) Decode(r IReader) error {
	if err := r.TRead(&m.Flags); err != nil {
		return err
	}
	blk := &BlockInfo{}
	if m.IsUseBytes() {
		if err := m.Bytes.Decode(r); err != nil {
			return err
		}
		br := NewReader(m.Bytes)
		if err := blk.Decode(br); err != nil {
			return err
		}
	} else if m.IsUseBlk() {
		if err := blk.Decode(r); err != nil {
			return err
		}
	} else {
		return errors.New("miss data")
	}
	m.Blk = blk
	return nil
}
