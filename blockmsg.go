package xginx

import (
	"errors"
)

const (
	//每次请求的最大区块头数量
	REQ_MAX_HEADERS_SIZE = 2000
)

// NT_GET_HEADERS
type MsgGetHeaders struct {
	Skip  VarInt //跳过
	Start HASH256
	Limit VarInt
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

//创建区块头消息
func (bi *BlockIndex) NewMsgHeaders(hs ...BlockHeader) *MsgHeaders {
	hm := &MsgHeaders{}
	hm.Height = bi.GetNodeHeight()
	for _, hv := range hs {
		hm.Add(hv)
	}
	return hm
}

//根据高度定位返回
func (bi *BlockIndex) GetMsgHeadersUseHeight(h uint32) *MsgHeaders {
	iter := bi.NewIter()
	hm := bi.NewMsgHeaders()
	if h == InvalidHeight {
		h = 0
	} else {
		h++
	}
	if !iter.SeekHeight(h) {
		return hm
	}
	for i := 0; iter.Next() && i < REQ_MAX_HEADERS_SIZE; i++ {
		bh := iter.Curr().BlockHeader
		hm.Add(bh)
	}
	return hm
}

//请求最后消息头
func (bi *BlockIndex) ReqMsgHeaders() *MsgGetHeaders {
	msg := &MsgGetHeaders{}
	last := bi.Last()
	if last == nil {
		msg.Start = conf.genesis //从头开始获取
		msg.Skip = 0
	} else if id, err := last.ID(); err != nil {
		panic(err)
	} else {
		msg.Start = id
		msg.Skip = 1 //跳过一个，不包含id
	}
	return msg
}

//获取链上的区块头返回
func (bi *BlockIndex) GetMsgHeaders(msg *MsgGetHeaders) *MsgHeaders {
	hm := bi.NewMsgHeaders()
	iter := bi.NewIter()
	if !iter.SeekID(msg.Start) {
		LogInfof("get msg headers seek id %v failed", msg.Start)
		return hm
	}
	var ifn func() bool = nil
	bfw := msg.Limit < 0
	if bfw {
		ifn = func() bool {
			return iter.Prev()
		}
	} else {
		ifn = func() bool {
			return iter.Next()
		}
	}
	skip := msg.Skip
	for skip > 0 && ifn() {
		skip--
	}
	limit := REQ_MAX_HEADERS_SIZE
	if bfw {
		msg.Limit = -msg.Limit
	}
	if msg.Limit > 0 {
		limit = msg.Limit.ToInt()
	}
	hvs := []BlockHeader{}
	for i := 0; ifn() && i < limit; i++ {
		bh := iter.Curr().BlockHeader
		hvs = append(hvs, bh)
	}
	//如果是反向获取的需要倒转数组
	if bfw {
		for i := len(hvs) - 1; i >= 0; i-- {
			hm.Add(hvs[i])
		}
	} else {
		hm.Headers = hvs
	}
	return hm
}

// NT_HEADERS

type MsgHeaders struct {
	Height  BHeight //并及时通告发送方的最新高度
	Headers []BlockHeader
}

func (m MsgHeaders) Type() uint8 {
	return NT_HEADERS
}

func (m *MsgHeaders) Add(h BlockHeader) {
	m.Headers = append(m.Headers, h)
}

//最后一个
func (m MsgHeaders) LastID() HASH256 {
	lh := m.Headers[len(m.Headers)-1]
	id, err := lh.ID()
	if err != nil {
		panic(err)
	}
	return id
}

//检测连续性
func (m MsgHeaders) Check() error {
	if len(m.Headers) == 0 {
		return nil
	}
	ph := m.Headers[0]
	if err := ph.Check(); err != nil {
		return err
	}
	for i := 1; i < len(m.Headers); i++ {
		cv := m.Headers[i]
		if err := cv.Check(); err != nil {
			return err
		}
		if !cv.Prev.Equal(ph.MustID()) {
			return errors.New("headers not continue")
		}
		ph = cv
	}
	return nil
}

func (m MsgHeaders) Encode(w IWriter) error {
	if err := w.TWrite(m.Height); err != nil {
		return err
	}
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
	if err := r.TRead(&m.Height); err != nil {
		return err
	}
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
