package xginx

import (
	"bytes"
	"container/list"
	"errors"
	"sync"
)

const (
	TBELoadedHead = 1 << 0 //如果已经加载了块头
)

var (
	//主链
	MainChain = NewBlockIndex()
)

//索引头
type TBEle struct {
	flags uint8
	TBMeta
	Height uint32
}

//从磁盘加载块头
func (e *TBEle) LoadMeta(id HASH256) error {
	if e.flags&TBELoadedHead != 0 {
		return nil
	}
	bk := GetDBKey(BLOCK_PREFIX, id[:])
	hb, err := IndexDB().Get(bk)
	if err != nil {
		return err
	}
	buf := bytes.NewReader(hb)
	if err := e.TBMeta.Decode(buf); err != nil {
		return err
	}
	if !id.Equal(e.TBMeta.Hash()) {
		return errors.New("hash error")
	}
	e.flags |= TBELoadedHead
	return nil
}

//从磁盘加载块头
func (e *TBEle) LoadBlock() (*BlockInfo, error) {
	if e.flags&TBELoadedHead == 0 {
		return nil, errors.New("meta not laod")
	} else {
		id := e.TBMeta.Hash()
		return LoadBlock(id)
	}
}

func NewTBEle(meta *TBMeta) *TBEle {
	return &TBEle{
		flags:  TBELoadedHead,
		TBMeta: *meta,
		Height: 0,
	}
}

func LoadTBEle(id HASH256) (*TBEle, error) {
	ele := &TBEle{}
	err := ele.LoadMeta(id)
	return ele, err
}

type BlockIndex struct {
	mu   sync.RWMutex
	lis  *list.List
	cur  *list.Element
	hmap map[uint32]*list.Element
}

//最低块
func (bi *BlockIndex) Lowest() *TBEle {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	le := bi.lis.Front()
	if le == nil {
		return nil
	}
	return le.Value.(*TBEle)
}

//最高块
func (bi *BlockIndex) Highest() *TBEle {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	le := bi.lis.Back()
	if le == nil {
		return nil
	}
	return le.Value.(*TBEle)
}

//链长度
func (bi *BlockIndex) Len() int {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.lis.Len()
}

//当前 当 Prev Next ToBack ToFront 成功时可调用
func (bi *BlockIndex) Current() *TBEle {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.cur.Value.(*TBEle)
}

//上一个
func (bi *BlockIndex) Prev() bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	if bi.cur == nil {
		bi.cur = bi.lis.Back()
	}
	bi.cur = bi.cur.Prev()
	return bi.cur != nil
}

//下一个
func (bi *BlockIndex) Next() bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	if bi.cur == nil {
		bi.cur = bi.lis.Front()
	}
	bi.cur = bi.cur.Next()
	return bi.cur != nil
}

//断开最前一个
func (bi *BlockIndex) UnlinkFront() {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	fe := bi.lis.Front()
	if fe == nil {
		return
	}
	fv := fe.Value.(*TBEle)
	bi.lis.Remove(fe)
	delete(bi.hmap, fv.Height)
}

//断开最后一个
func (bi *BlockIndex) UnlinkBack() {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	le := bi.lis.Back()
	if le == nil {
		return
	}
	tv := le.Value.(*TBEle)
	bi.lis.Remove(le)
	delete(bi.hmap, tv.Height)
}

//移动光标到最后一个
func (bi *BlockIndex) ToBack() bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	bi.cur = bi.lis.Back()
	return bi.cur != nil
}

//移动光标到第一个
func (bi *BlockIndex) ToFront() bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	bi.cur = bi.lis.Front()
	return bi.cur != nil
}

func (bi *BlockIndex) pushback(e *TBEle) (*TBEle, error) {
	if _, has := bi.hmap[e.Height]; has {
		return nil, errors.New("height ele exists")
	} else {
		ele := bi.lis.PushBack(e)
		bi.hmap[e.Height] = ele
		return e, nil
	}
}

//加入一个队头并设置高度
func (bi *BlockIndex) LinkFront(e *TBEle) (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	first := bi.lis.Front()
	if first == nil {
		return bi.pushfront(e)
	}
	lv := first.Value.(*TBEle)
	if !lv.Header.Prev.Equal(e.Hash()) {
		return nil, errors.New("prev hash error")
	}
	e.Height = lv.Height - 1
	return bi.pushfront(e)
}

//根据高度获取块
func (bi *BlockIndex) GetTBEle(h uint32) *TBEle {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	v, ok := bi.hmap[h]
	if !ok {
		return nil
	}
	return v.Value.(*TBEle)
}

func (bi *BlockIndex) pushfront(e *TBEle) (*TBEle, error) {
	if _, has := bi.hmap[e.Height]; has {
		return nil, errors.New("height ele exists")
	} else {
		ele := bi.lis.PushFront(e)
		bi.hmap[e.Height] = ele
		return e, nil
	}
}

//向前加载
func (bi *BlockIndex) LoadPrev() (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	fe := bi.lis.Front()
	id := HASH256{}
	if fe != nil && conf.genesisId.Equal(fe.Value.(*TBEle).Hash()) {
		return nil, errors.New("curr is genesis block")
	} else if fe != nil {
		id = fe.Value.(*TBEle).Header.Prev
	} else {
		id = GetBestBlock()
	}
	//已经是第一个
	if id.IsZero() {
		return nil, errors.New("curr is genesis block")
	}
	meta, err := LoadTBEle(id)
	if err != nil {
		return nil, err
	}
	if fe != nil {
		meta.Height = fe.Value.(*TBEle).Height - 1
	}
	if _, err := bi.pushfront(meta); err != nil {
		return nil, err
	}
	return meta, nil
}

//检测是否可以链入尾部
func (bi *BlockIndex) IsLinkBack(meta *TBMeta) bool {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	last := bi.lis.Back()
	if last == nil {
		//不存在时第一个必须是genesis block
		return meta.Header.IsGenesis()
	}
	lv := last.Value.(*TBEle)
	if !meta.Header.Prev.Equal(lv.Hash()) {
		return false
	}
	_, has := bi.hmap[lv.Height+1]
	return !has
}

//加入一个队列尾并设置高度
func (bi *BlockIndex) LinkBack(meta *TBMeta) (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	ele := NewTBEle(meta)
	last := bi.lis.Back()
	if last == nil && meta.Header.IsGenesis() {
		return bi.pushback(ele)
	} else if last == nil {
		return nil, errors.New("link back error,last miss")
	}
	lv := last.Value.(*TBEle)
	if !ele.Header.Prev.Equal(lv.Hash()) {
		return nil, errors.New("prev hash error")
	}
	ele.Height = lv.Height + 1
	return bi.pushback(ele)
}

func NewBlockIndex() *BlockIndex {
	bi := &BlockIndex{
		lis:  list.New(),
		hmap: map[uint32]*list.Element{},
	}
	return bi
}
