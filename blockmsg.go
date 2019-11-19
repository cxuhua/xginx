package xginx

import "errors"

const (
	//每次请求的最大区块头数量
	REQ_MAX_HEADERS_SIZE = 200
)

// NT_GET_HEADERS
type MsgGetHeaders struct {
	Skip  VarUInt //跳过
	Start HASH256
	Limit HASH256
}

func (m MsgGetHeaders) Type() uint8 {
	return NT_GET_HEADERS
}

func (m MsgGetHeaders) Encode(w IWriter) error {
	if err := m.Skip.Encode(w); err != nil {
		return err
	}
	if err := m.Start.Encode(w); err != nil {
		return err
	}
	if err := m.Limit.Encode(w); err != nil {
		return err
	}
	return nil
}

func (m *MsgGetHeaders) Decode(r IReader) error {
	if err := m.Skip.Decode(r); err != nil {
		return err
	}
	if err := m.Start.Decode(r); err != nil {
		return err
	}
	if err := m.Limit.Decode(r); err != nil {
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

//根据高度定位返回
func (bi *BlockIndex) GetMsgHeadersUseHeight(h uint32) (*MsgHeaders, error) {
	iter := bi.NewIter()
	if h == InvalidHeight {
		h = 0
	} else {
		h++
	}
	if !iter.SeekHeight(h) {
		return nil, errors.New("not found start height")
	}
	hm := &MsgHeaders{}
	for i := 0; iter.Next() && i < REQ_MAX_HEADERS_SIZE; i++ {
		bh := iter.Curr().BlockHeader
		hm.Add(bh)
	}
	if len(hm.Headers) == 0 {
		return nil, errors.New("not more block header")
	}
	return hm, nil
}

//请求最后消息头
func (bi *BlockIndex) ReqMsgHeaders() *MsgGetHeaders {
	msg := &MsgGetHeaders{}
	last := bi.Last()
	if last == nil {
		msg.Start = ZERO //从头开始获取
		msg.Skip = 0
	} else if id, err := last.ID(); err != nil {
		LogError("last id error", err)
	} else {
		msg.Start = id
		msg.Skip = 1 //跳过一个，不包含id
	}
	return msg
}

//获取链上的区块头返回
func (bi *BlockIndex) GetMsgHeaders(msg *MsgGetHeaders) (*MsgHeaders, error) {
	iter := bi.NewIter()
	if msg.Start.Equal(ZERO) {
		if !iter.First() {
			return nil, errors.New("not found first")
		}
	} else {
		if !iter.SeekID(msg.Start) {
			return nil, errors.New("not found start id")
		}
	}
	skip := msg.Skip
	for skip > 0 && iter.Next() {
		skip--
	}
	hm := &MsgHeaders{}
	for i := 0; iter.Next() && i < REQ_MAX_HEADERS_SIZE; i++ {
		bh := iter.Curr().BlockHeader
		hm.Add(bh)
		id, err := bh.ID()
		if err != nil {
			return nil, err
		}
		if id.Equal(msg.Limit) {
			break
		}
	}
	if len(hm.Headers) == 0 {
		return nil, errors.New("not more block header")
	}
	return hm, nil
}

// NT_HEADERS

type MsgHeaders struct {
	Headers []BlockHeader
}

func (m MsgHeaders) Type() uint8 {
	return NT_HEADERS
}

func (m *MsgHeaders) Add(h BlockHeader) {
	m.Headers = append(m.Headers, h)
}

func (m MsgHeaders) Encode(w IWriter) error {
	err := VarInt(len(m.Headers)).Encode(w)
	if err != nil {
		return err
	}
	for _, v := range m.Headers {
		if err := v.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

func (m *MsgHeaders) Decode(r IReader) error {
	num := VarInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Headers = make([]BlockHeader, num.ToInt())
	for i, _ := range m.Headers {
		v := BlockHeader{}
		err := v.Decode(r)
		if err != nil {
			return err
		}
		m.Headers[i] = v
	}
	return nil
}

//NT_BLOCK

type MsgBlock struct {
	Blk *BlockInfo
}

func NewMsgBlock(blk *BlockInfo) *MsgBlock {
	return &MsgBlock{Blk: blk}
}

func (m MsgBlock) Type() uint8 {
	return NT_BLOCK
}

func (m MsgBlock) Encode(w IWriter) error {
	return m.Blk.Encode(w)
}

func (m *MsgBlock) Decode(r IReader) error {
	m.Blk = &BlockInfo{}
	return m.Blk.Decode(r)
}
