package xginx

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

const (
	TBELoadedMeta  = 1 << 0     //如果已经加载了块头
	TBELoadedBlock = 1 << 1     //如果加载了块数据
	InvalidHeight  = ^uint32(0) //无效的块高度
)

//索引头
type TBEle struct {
	flags uint8
	TBMeta
	Height uint32
	idx    *BlockIndex
}

//从磁盘加载块头
func (e *TBEle) LoadMeta(id HASH256) error {
	if e.flags&TBELoadedMeta != 0 {
		return nil
	}
	hb, err := e.idx.store.Index().Get(BLOCK_PREFIX, id[:])
	if err != nil {
		return err
	}
	buf := bytes.NewReader(hb)
	if err := e.TBMeta.Decode(buf); err != nil {
		return err
	}
	if !id.Equal(e.TBMeta.ID()) {
		return errors.New("hash error")
	}
	e.flags |= TBELoadedMeta
	return nil
}

func NewTBEle(meta *TBMeta, idx *BlockIndex) *TBEle {
	return &TBEle{
		flags:  TBELoadedMeta,
		TBMeta: *meta,
		Height: 0,
		idx:    idx,
	}
}

type BlockIndex struct {
	mu sync.RWMutex
	//区块头列表
	lis *list.List
	//当前光标
	cur *list.Element
	//按高度缓存
	hmap map[uint32]*list.Element
	//按id缓存
	imap map[HASH256]*list.Element
	//块缓存
	cacher *cache.Cache
	//存储
	store IStore
}

//最低块
func (bi *BlockIndex) First() *TBEle {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	le := bi.lis.Front()
	if le == nil {
		return nil
	}
	return le.Value.(*TBEle)
}

//最高块
func (bi *BlockIndex) Last() *TBEle {
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

//加载区块
func (bi *BlockIndex) LoadBlock(id HASH256) (*BlockInfo, error) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	ckey := string(id[:])
	if v, has := bi.cacher.Get(ckey); has {
		return v.(*BlockInfo), nil
	}
	ele, has := bi.imap[id]
	if !has {
		return nil, fmt.Errorf("chain not load %v", id)
	}
	smeta := ele.Value.(*TBEle)
	bptr := &BlockInfo{}
	lmeta, err := bi.LoadTo(id, bptr)
	if err != nil {
		return nil, err
	}
	if !lmeta.Hash().Equal(smeta.Hash()) {
		return nil, fmt.Errorf("meta data error")
	}
	bptr.Meta = smeta
	bi.cacher.Set(ckey, bptr, time.Minute*30)
	return bptr, nil
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
	delete(bi.hmap, fv.Height)
	delete(bi.imap, fv.ID())
	bi.lis.Remove(fe)
}

//断开最后一个
func (bi *BlockIndex) UnlinkBack() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	le := bi.lis.Back()
	if le == nil {
		return nil
	}
	tv := le.Value.(*TBEle)
	delete(bi.hmap, tv.Height)
	delete(bi.imap, tv.ID())
	bi.lis.Remove(le)
	return nil
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
	ele := bi.lis.PushBack(e)
	bi.hmap[e.Height] = ele
	bi.imap[e.ID()] = ele
	return e, nil
}

//加入一个队头并设置高度
func (bi *BlockIndex) LinkFront(meta *TBMeta) (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	ele := NewTBEle(meta, bi)
	first := bi.lis.Front()
	if first == nil {
		return bi.pushfront(ele)
	}
	lv := first.Value.(*TBEle)
	if !lv.Prev.Equal(ele.ID()) {
		return nil, errors.New("prev hash error")
	}
	ele.Height = lv.Height - 1
	return bi.pushfront(ele)
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
	ele := bi.lis.PushFront(e)
	bi.hmap[e.Height] = ele
	bi.imap[e.ID()] = ele
	return e, nil
}

var (
	//向前加载完毕
	FirstIsGenesis = errors.New("first is genesis block")
	//没有best区块
	NotFoundBest = errors.New("not found best block")
)

//加载所有链meta
func (bi *BlockIndex) LoadAll() error {
	log.Println("start load main chain block header")
	hh := InvalidHeight
	vv := uint(0)
	for {
		ele, err := bi.LoadPrev()
		if err == FirstIsGenesis {
			log.Println("load main chain block header finish", bi.Len())
			return nil
		}
		if err == NotFoundBest {
			log.Println("block chain data empty")
			return nil
		}
		if err != nil {
			log.Println("load main chain block header error = ", err)
			return err
		}
		if hh == InvalidHeight {
			hh = ele.Height
		}
		p := 1 - (float32(ele.Height) / float32(hh))
		cv := uint((p * 10))
		if cv != vv {
			log.Println("load main chain progress ", cv*10, "%", bi.Len())
			vv = cv
		}
	}
}

func (bi *BlockIndex) LoadTBEle(id HASH256) (*TBEle, error) {
	ele := &TBEle{idx: bi}
	err := ele.LoadMeta(id)
	return ele, err
}

//向前加载一个区块数据头
func (bi *BlockIndex) LoadPrev() (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	fe := bi.lis.Front()
	id := HASH256{}
	ih := uint32(0)
	if fe != nil && conf.genesisId.Equal(fe.Value.(*TBEle).ID()) {
		return nil, FirstIsGenesis
	} else if fe != nil {
		id = fe.Value.(*TBEle).Prev
	} else if bv := bi.store.GetBestValue(); !bv.IsValid() {
		return nil, NotFoundBest
	} else {
		id = bv.Id
		ih = bv.Height
	}
	meta, err := bi.LoadTBEle(id)
	if err != nil {
		return nil, err
	}
	if fe != nil {
		meta.Height = fe.Value.(*TBEle).Height - 1
	} else {
		meta.Height = ih
	}
	if _, err := bi.pushfront(meta); err != nil {
		return nil, err
	}
	return meta, nil
}

//检测是否可以链入尾部,并返回下一个高度
func (bi *BlockIndex) IsLinkBack(meta *TBMeta) (uint32, bool) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	last := bi.lis.Back()
	if last == nil {
		//不存在时第一个必须是genesis block
		return 0, meta.IsGenesis()
	}
	lv := last.Value.(*TBEle)
	if !meta.Prev.Equal(lv.ID()) {
		return 0, false
	}
	return lv.Height + 1, true
}

//加入一个队列尾并设置高度
func (bi *BlockIndex) LinkBack(meta *TBMeta) (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	ele := NewTBEle(meta, bi)
	last := bi.lis.Back()
	if last == nil && meta.IsGenesis() {
		return bi.pushback(ele)
	} else if last == nil {
		return nil, errors.New("link back error,last miss")
	}
	lv := last.Value.(*TBEle)
	if !ele.Prev.Equal(lv.ID()) {
		return nil, errors.New("prev hash error")
	}
	ele.Height = lv.Height + 1
	return bi.pushback(ele)
}

func (chain *BlockIndex) LoadTxValue(id HASH256) (*TxValue, error) {
	vk := GetDBKey(TXS_PREFIX, id[:])
	vb, err := chain.store.State().Get(vk)
	if err != nil {
		return nil, err
	}
	vv := &TxValue{}
	err = vv.Decode(bytes.NewReader(vb))
	return vv, err
}

func (chain *BlockIndex) LoadUvValue(id HASH256) (*UvValue, error) {
	vk := GetDBKey(UXS_PREFIX, id[:])
	vb, err := chain.store.State().Get(vk)
	if err != nil {
		return nil, err
	}
	vv := &UvValue{}
	err = vv.Decode(bytes.NewReader(vb))
	return vv, err
}

func (chain *BlockIndex) LoadCliLastUnit(cli HASH160) (HASH256, error) {
	id := HASH256{}
	ckey := GetDBKey(CBI_PREFIX, cli[:])
	bb, err := chain.store.State().Get(ckey)
	if err != nil {
		return id, err
	}
	copy(id[:], bb)
	return id, nil
}

//加载块数据
func (chain *BlockIndex) LoadTo(id HASH256, block *BlockInfo) (*TBMeta, error) {
	bk := GetDBKey(BLOCK_PREFIX, id[:])
	meta := &TBMeta{}
	hb, err := chain.store.Index().Get(bk)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewReader(hb)
	if err := meta.Decode(buf); err != nil {
		return nil, err
	}
	bb, err := chain.store.Blk().Read(meta.Blk)
	if err != nil {
		return nil, err
	}
	err = block.Decode(bytes.NewReader(bb))
	return meta, err
}

//断开最后一个，必须是最后一个才能断开
func (chain *BlockIndex) Unlink(block *BlockInfo) error {
	if chain.Len() == 0 {
		return nil
	}
	if block.Meta == nil {
		return errors.New("block meta miss")
	}
	id := block.ID()
	bk := GetDBKey(BLOCK_PREFIX, id.Bytes())
	if !chain.Last().ID().Equal(id) {
		return errors.New("only unlink last block")
	}
	revbb, err := chain.store.Rev().Read(block.Meta.Rev)
	if err != nil {
		return fmt.Errorf("read block rev data error %w", err)
	}
	revb, err := LoadBatch(revbb)
	if err != nil {
		return fmt.Errorf("load rev batch error %w", err)
	}
	if err := chain.UnlinkBack(); err != nil {
		return err
	}
	//删除数据
	if err := chain.store.Index().Del(bk); err != nil {
		return err
	}
	if err := chain.store.State().Write(revb); err != nil {
		return err
	}
	return nil
}

//link
func (chain *BlockIndex) LinkTo(block *BlockInfo) (*TBEle, error) {
	id, meta, bb, err := block.ToTBMeta()
	if err != nil {
		return nil, err
	}
	//是否能连接到主链后
	nexth, blink := chain.IsLinkBack(meta)
	if !blink {
		return nil, fmt.Errorf("can't link to main chain hash=%v", id)
	}
	//区块状态写入
	batch := NewBatch()
	//设置事物回退
	revb := batch.SetRev(NewBatch())
	//更新bestBlockId
	bv := BestValue{Id: id, Height: nexth}
	batch.Put(BestBlockKey, bv.Bytes())
	if err := block.WriteUvsIdx(batch); err != nil {
		return nil, err
	}
	if err := block.WriteTxsIdx(batch); err != nil {
		return nil, err
	}
	if batch.Len() > MAX_BLOCK_SIZE || revb.Len() > MAX_BLOCK_SIZE {
		return nil, errors.New("opts state logs too big > MAX_BLOCK_SIZE")
	}
	//获取上一个区块
	if !block.IsGenesis() {
		last := chain.Last()
		pb, err := chain.LoadBlock(last.ID())
		if err != nil {
			return nil, err
		}
		bv := BestValue{Id: last.ID(), Height: last.Height}
		//保存上一个用于日志回退
		revb.Put(BestBlockKey, bv.Bytes())
		//保存cli的上一个块用于数据回退
		if err := pb.WriteCliBestId(revb); err != nil {
			return nil, err
		}
	}
	//区块索引key
	bk := GetDBKey(BLOCK_PREFIX, id.Bytes())
	//保存回退日志
	meta.Rev, err = chain.store.Rev().Write(revb.Dump())
	if err != nil {
		return nil, err
	}
	//保存区块数据
	meta.Blk, err = chain.store.Blk().Write(bb)
	if err != nil {
		return nil, err
	}
	//保存头
	hbs, err := meta.Bytes()
	if err != nil {
		return nil, err
	}
	//连接区块
	ele, err := chain.LinkBack(meta)
	if err != nil {
		return nil, err
	}
	//保存区块信息索引
	if err := chain.store.Index().Put(bk, hbs); err != nil {
		return nil, err
	}
	//更新状态
	if err := chain.store.State().Write(batch); err != nil {
		return nil, err
	}
	return ele, nil
}

func NewBlockIndex() *BlockIndex {
	bi := &BlockIndex{
		lis:    list.New(),
		hmap:   map[uint32]*list.Element{},
		imap:   map[HASH256]*list.Element{},
		cacher: cache.New(time.Minute*30, time.Hour),
		store:  store,
	}
	return bi
}
