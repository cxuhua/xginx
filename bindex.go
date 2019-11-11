package xginx

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
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
func (ele *TBEle) LoadMeta(id HASH256) error {
	if ele.flags&TBELoadedMeta != 0 {
		return nil
	}
	hb, err := ele.idx.db.Index().Get(BLOCK_PREFIX, id[:])
	if err != nil {
		return err
	}
	buf := bytes.NewReader(hb)
	if err := ele.TBMeta.Decode(buf); err != nil {
		return err
	}
	if !id.Equal(ele.TBMeta.ID()) {
		return errors.New("hash error")
	}
	ele.flags |= TBELoadedMeta
	return nil
}

func EmptyTBEle(h uint32, i *BlockIndex) *TBEle {
	return &TBEle{
		Height: h,
		idx:    i,
	}
}

func NewTBEle(meta *TBMeta, idx *BlockIndex) *TBEle {
	return &TBEle{
		flags:  TBELoadedMeta,
		TBMeta: *meta,
		Height: InvalidHeight,
		idx:    idx,
	}
}

type IListener interface {
	//当块创建完毕
	OnNewBlock(bi *BlockIndex, blk *BlockInfo) error
	//完成区块
	OnFinished(bi *BlockIndex, blk *BlockInfo) error
	//获取签名私钥
	OnPrivateKey(bi *BlockIndex, blk *BlockInfo, out *TxOut) (*PrivateKey, error)
}

//区块链索引
type BlockIndex struct {
	//链监听器
	lptr IListener
	//
	mu sync.RWMutex
	//区块头列表
	lis *list.List
	//当前光标
	cur *list.Element
	//按高度缓存
	hmap map[uint32]*list.Element
	//按id缓存
	imap map[HASH256]*list.Element
	//lru缓存
	lru *Cache
	//存储
	db IBlkStore
}

//获取当前监听器
func (bi BlockIndex) GetListener() IListener {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.lptr
}

func (bi *BlockIndex) GetHEle(h uint32) *TBEle {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	ele, has := bi.hmap[h]
	if !has {
		return nil
	}
	return ele.Value.(*TBEle)
}

//计算当前难度
func (bi *BlockIndex) CalcBits(height uint32) uint32 {
	last := bi.Last()
	if last == nil || height == 0 {
		return GetMinPowBits()
	}
	if height%conf.PowSpan != 0 {
		return last.Bits
	}
	ct := last.Time
	ph := height - conf.PowSpan
	ele := bi.GetHEle(ph)
	if ele == nil {
		panic(errors.New("prev height height miss"))
	}
	pt := ele.Time
	return CalculateWorkRequired(ct, pt, last.Bits)
}

//创建下一个高度基本数据
func (bi *BlockIndex) NewBlock(ver uint32) (*BlockInfo, error) {
	if bi.lptr == nil {
		return nil, errors.New("block index listener null")
	}
	blk := &BlockInfo{}
	blk.Header.Ver = ver
	blk.Header.Time = uint32(time.Now().Unix())
	nexth := InvalidHeight
	//设置当前难度
	if last := bi.Last(); last == nil {
		blk.Header.Bits = GetMinPowBits()
		nexth = 0
	} else {
		nexth = last.Height + 1
		blk.Header.Prev = last.ID()
		blk.Header.Bits = bi.CalcBits(nexth)
	}
	SetRandInt(&blk.Header.Nonce)
	meta := EmptyTBEle(nexth, bi)
	meta.TBMeta.BlockHeader = blk.Header
	blk.Meta = meta
	if err := bi.lptr.OnNewBlock(bi, blk); err != nil {
		return nil, err
	}
	return blk, nil
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
	hptr := bi.lru.Get(id, func() (size int, value Value) {
		ele, has := bi.imap[id]
		if !has {
			return 0, nil
		}
		smeta := ele.Value.(*TBEle)
		bptr := &BlockInfo{}
		lmeta, err := bi.LoadTo(id, bptr)
		if err != nil {
			return 0, nil
		}
		if !lmeta.Hash().Equal(smeta.Hash()) {
			return 0, nil
		}
		bptr.Meta = smeta
		return smeta.Blk.Len.ToInt(), bptr
	})
	if hptr == nil {
		return nil, errors.New("load block failed")
	}
	return hptr.Value().(*BlockInfo), nil
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

//加载所有链meta
func (bi *BlockIndex) LoadAll() error {
	log.Println("start load main chain block header")
	hh := InvalidHeight
	vv := uint(0)
	for i := 0; ; i++ {
		ele, err := bi.LoadPrev()
		if err == FirstBlockErr {
			break
		}
		if err != nil {
			return err
		}
		if hh == InvalidHeight {
			hh = ele.Height + 1
		}
		p := 1 - (float32(ele.Height) / float32(hh))
		cv := uint((p * 10))
		if cv != vv {
			log.Println("load main chain progress ", cv*10, "%", bi.Len())
			vv = cv
		}
	}
	log.Println("load finished", bi.Len())
	return nil
}

func (bi *BlockIndex) LoadTBEle(id HASH256) (*TBEle, error) {
	ele := &TBEle{idx: bi}
	err := ele.LoadMeta(id)
	return ele, err
}

var (
	FirstBlockErr = errors.New("arrive first block")
)

//向前加载一个区块数据头
func (bi *BlockIndex) LoadPrev() (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	fe := bi.lis.Front()
	id := HASH256{}
	ih := uint32(0)
	if fe != nil {
		id = fe.Value.(*TBEle).Prev
	} else if bv := bi.GetBestValue(); !bv.IsValid() {
		return nil, errors.New("get best error")
	} else {
		id = bv.Id
		ih = bv.Height
	}
	meta, err := bi.LoadTBEle(id)
	if err != nil {
		return nil, err
	}
	if meta.Prev.IsZero() {
		//到达第一个
		meta.Height = 0
	} else if fe != nil {
		meta.Height = fe.Value.(*TBEle).Height - 1
	} else {
		//最后一个
		meta.Height = ih
	}
	if _, err := bi.pushfront(meta); err != nil {
		return nil, err
	}
	if meta.Prev.IsZero() {
		return meta, FirstBlockErr
	} else {
		return meta, nil
	}
}

//检测是否可以链入尾部,并返回当前高度和当前id
func (bi *BlockIndex) IsLinkBack(meta *TBMeta) (uint32, HASH256, bool) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	last := bi.lis.Back()
	if last == nil {
		return 0, HASH256{}, true
	}
	lv := last.Value.(*TBEle)
	if !meta.Prev.Equal(lv.ID()) {
		return 0, HASH256{}, false
	}
	return lv.Height, lv.ID(), true
}

//加入一个队列尾并设置高度
func (bi *BlockIndex) LinkBack(meta *TBMeta) (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	ele := NewTBEle(meta, bi)
	last := bi.lis.Back()
	if last == nil {
		return bi.pushback(ele)
	}
	lv := last.Value.(*TBEle)
	if !ele.Prev.Equal(lv.ID()) {
		return nil, errors.New("prev hash error")
	}
	ele.Height = lv.Height + 1
	return bi.pushback(ele)
}

func (bi *BlockIndex) LoadTX(id HASH256) (*TX, error) {
	hptr := bi.lru.Get(id, func() (size int, value Value) {
		txv, err := bi.LoadTxValue(id)
		if err != nil {
			return 0, nil
		}
		tx, err := txv.GetTX(bi)
		if err != nil {
			return 0, nil
		}
		buf := &bytes.Buffer{}
		if err := tx.Encode(buf); err != nil {
			return 0, nil
		}
		return buf.Len(), tx
	})
	if hptr == nil {
		return nil, errors.New("not found")
	}
	return hptr.Value().(*TX), nil
}

func (bi *BlockIndex) LoadTxValue(id HASH256) (*TxValue, error) {
	vk := GetDBKey(TXS_PREFIX, id[:])
	vb, err := bi.db.Index().Get(vk)
	if err != nil {
		return nil, err
	}
	vv := &TxValue{}
	err = vv.Decode(bytes.NewReader(vb))
	return vv, err
}

//加载块数据
func (bi *BlockIndex) LoadTo(id HASH256, block *BlockInfo) (*TBMeta, error) {
	bk := GetDBKey(BLOCK_PREFIX, id[:])
	meta := &TBMeta{}
	hb, err := bi.db.Index().Get(bk)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewReader(hb)
	if err := meta.Decode(buf); err != nil {
		return nil, err
	}
	bb, err := bi.db.Blk().Read(meta.Blk)
	if err != nil {
		return nil, err
	}
	err = block.Decode(bytes.NewReader(bb))
	return meta, err
}

//清除区块相关的缓存
func (bi *BlockIndex) cleanBlockCache(b *BlockInfo) {
	for _, tv := range b.Txs {
		bi.lru.Delete(tv.Hash())
	}
	bi.lru.Delete(b.ID())
}

//断开最后一个
func (bi *BlockIndex) UnlinkLast() error {
	last := bi.Last()
	if last == nil {
		return errors.New("last block miss")
	}
	b, err := bi.LoadBlock(last.ID())
	if err != nil {
		return err
	}
	err = bi.Unlink(b)
	if err == nil {
		bi.cleanBlockCache(b)
	}
	return err
}

//断开最后一个，必须是最后一个才能断开
func (bi *BlockIndex) Unlink(bp *BlockInfo) error {
	if bi.Len() == 0 {
		return nil
	}
	if bp.Meta == nil {
		return errors.New("block meta miss")
	}
	id := bp.ID()
	if !bi.Last().ID().Equal(id) {
		return errors.New("only unlink last block")
	}
	rb, err := bi.db.Rev().Read(bp.Meta.Rev)
	if err != nil {
		return fmt.Errorf("read block rev data error %w", err)
	}
	bt, err := LoadBatch(rb)
	if err != nil {
		return fmt.Errorf("load rev batch error %w", err)
	}
	if err := bi.db.Index().Write(bt); err != nil {
		return err
	}
	//断开链接
	return bi.UnlinkBack()
}

//获取最高块信息
func (bi *BlockIndex) GetBestValue() BestValue {
	bv := BestValue{}
	b, err := bi.db.Index().Get(BestBlockKey)
	if err != nil {
		return InvalidBest
	}
	if err := bv.From(b); err != nil {
		return InvalidBest
	}
	return bv
}

//获取某个id的所有积分
func (bi *BlockIndex) ListTokens(id HASH160) ([]*CoinKeyValue, error) {
	prefix := getDBKey(COIN_PREFIX, id[:])
	kvs := []*CoinKeyValue{}
	iter := bi.db.Index().Iterator(NewPrefix(prefix))
	defer iter.Close()
	for iter.Next() {
		kv := &CoinKeyValue{}
		err := kv.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, kv)
	}
	return kvs, nil
}

//暂时缓存交易
func (bi *BlockIndex) SetTx(tx *TX) error {
	bi.lru.Get(tx.Hash(), func() (size int, value Value) {
		buf := &bytes.Buffer{}
		if err := tx.Encode(buf); err != nil {
			return 0, nil
		}
		return buf.Len(), tx
	})
	return nil
}

//链接一个区块
func (bi *BlockIndex) LinkTo(bp *BlockInfo) (*TBEle, error) {
	cid, meta, bb, err := bp.ToTBMeta()
	if err != nil {
		return nil, err
	}
	//是否能连接到主链后
	phv, pid, isok := bi.IsLinkBack(meta)
	if !isok {
		return nil, fmt.Errorf("can't link to main chain hash=%v", cid)
	}
	//区块状态写入
	bt := NewBatch()
	//设置事物回退
	rt := bt.SetRev(NewBatch())
	//第一个
	if pid.IsZero() {
		bv := BestValue{Id: cid, Height: phv}
		bt.Put(BestBlockKey, bv.Bytes())
	} else {
		bv := BestValue{Id: cid, Height: phv + 1}
		bt.Put(BestBlockKey, bv.Bytes())
		//写回退
		cv := BestValue{Id: pid, Height: phv}
		rt.Put(BestBlockKey, cv.Bytes())
	}
	if err := bp.WriteTxsIdx(bi, bt); err != nil {
		return nil, err
	}
	//检测日志文件
	if bt.Len() > MAX_BLOCK_SIZE || rt.Len() > MAX_BLOCK_SIZE {
		return nil, errors.New("opts state logs too big > MAX_BLOCK_SIZE")
	}
	//保存回退日志
	meta.Rev, err = bi.db.Rev().Write(rt.Dump())
	if err != nil {
		return nil, err
	}
	//保存区块数据
	meta.Blk, err = bi.db.Blk().Write(bb)
	if err != nil {
		return nil, err
	}
	//保存区块头数据
	hbs, err := meta.Bytes()
	if err != nil {
		return nil, err
	}
	bt.Put(BLOCK_PREFIX, cid[:], hbs)
	//更新区块状态
	if err := bi.db.Index().Write(bt); err != nil {
		return nil, err
	}
	//连接区块
	return bi.LinkBack(meta)
}

func NewBlockIndex(lptr IListener) *BlockIndex {
	bi := &BlockIndex{
		lptr: lptr,
		lis:  list.New(),
		hmap: map[uint32]*list.Element{},
		imap: map[HASH256]*list.Element{},
		db:   NewLevelDBStore(conf.DataDir),
		lru:  NewCache(1024 * 1024 * 256),
	}
	return bi
}
