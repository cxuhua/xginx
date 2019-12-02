package xginx

import (
	"crypto/md5"
	"errors"
)

//NT_GET_BLOCK
type MsgGetBlock struct {
	BlkId  HASH256
	Height uint32
}

func (m MsgGetBlock) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (m MsgGetBlock) Type() uint8 {
	return NT_GET_BLOCK
}

func (m MsgGetBlock) Encode(w IWriter) error {
	if err := m.BlkId.Encode(w); err != nil {
		return err
	}
	if err := w.TWrite(m.Height); err != nil {
		return err
	}
	return nil
}

func (m *MsgGetBlock) Decode(r IReader) error {
	if err := m.BlkId.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&m.Height); err != nil {
		return err
	}
	return nil
}

//获取区块数据返回
func (bi *BlockIndex) GetMsgBlock(id HASH256) (*MsgBlock, error) {
	blk, err := bi.LoadBlock(id)
	if err != nil {
		return nil, err
	}
	return &MsgBlock{Blk: blk}, nil
}

const (
	//如果是新出的区块设置此标记并广播
	MsgBlockNewFlags = 1 << 0
	MsgBlockUseBytes = 1 << 1
	MsgBlockUseBlk   = 1 << 2
)

type MsgBlock struct {
	Flags uint8
	Blk   *BlockInfo
	Bytes VarBytes
}

func NewMsgBlock(blk *BlockInfo) *MsgBlock {
	m := &MsgBlock{Blk: blk}
	m.AddFlags(MsgBlockUseBlk)
	return m
}

func NewMsgBlockBytes(b []byte) *MsgBlock {
	m := &MsgBlock{Bytes: b}
	m.AddFlags(MsgBlockUseBytes)
	return m
}

func (m MsgBlock) Id() (MsgId, error) {
	bid, err := m.Blk.ID()
	if err != nil {
		return ErrMsgId, err
	}
	return md5.Sum(bid[:]), nil
}

func (m *MsgBlock) AddFlags(f uint8) {
	m.Flags |= f
}

func (m MsgBlock) Type() uint8 {
	return NT_BLOCK
}

func (m MsgBlock) IsUseBytes() bool {
	return m.Flags&MsgBlockUseBytes != 0
}

func (m MsgBlock) IsUseBlk() bool {
	return m.Flags&MsgBlockUseBlk != 0
}

func (m MsgBlock) IsBroad() bool {
	return m.Flags&MsgBlockNewFlags != 0
}

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
