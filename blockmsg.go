package xginx

import (
	"crypto/md5"
	"errors"
)

//NT_GET_BLOCK
type MsgGetBlock struct {
	Height uint32
}

func (m MsgGetBlock) Id() (MsgId, error) {
	return ErrMsgId, NotIdErr
}

func (m MsgGetBlock) Type() uint8 {
	return NT_GET_BLOCK
}

func (m MsgGetBlock) Encode(w IWriter) error {
	return w.TWrite(m.Height)
}

func (m *MsgGetBlock) Decode(r IReader) error {
	return r.TRead(&m.Height)
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
)

type MsgBlock struct {
	Flags uint8
	Blk   *BlockInfo
}

func NewMsgBlock(blk *BlockInfo) *MsgBlock {
	return &MsgBlock{Blk: blk}
}

func (m MsgBlock) Id() (MsgId, error) {
	bid, err := m.Blk.ID()
	if err != nil {
		return ErrMsgId, err
	}
	return md5.Sum(bid[:]), nil
}

func (m MsgBlock) Type() uint8 {
	return NT_BLOCK
}

func (m MsgBlock) IsBroad() bool {
	return m.Flags&MsgBlockNewFlags != 0
}

func (m MsgBlock) Encode(w IWriter) error {
	if m.Blk == nil {
		return errors.New("blk nil")
	}
	if err := w.TWrite(m.Flags); err != nil {
		return err
	}
	if err := m.Blk.Encode(w); err != nil {
		return err
	}
	return nil
}

func (m *MsgBlock) Decode(r IReader) error {
	if err := r.TRead(&m.Flags); err != nil {
		return err
	}
	blk := &BlockInfo{}
	if err := blk.Decode(r); err != nil {
		return err
	}
	m.Blk = blk
	return nil
}
