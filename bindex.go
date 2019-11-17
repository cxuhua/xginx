package xginx

import (
	"container/list"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb/opt"
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

func (ele TBEle) String() string {
	id, _ := ele.ID()
	return id.String()
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
	buf := NewReader(hb)
	if err := ele.TBMeta.Decode(buf); err != nil {
		return err
	}
	if eleid, err := ele.TBMeta.ID(); err != nil {
		return err
	} else if !id.Equal(eleid) {
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

func NewTBEle(meta *TBMeta, height uint32, idx *BlockIndex) *TBEle {
	return &TBEle{
		flags:  TBELoadedMeta,
		TBMeta: *meta,
		Height: height,
		idx:    idx,
	}
}

type IListener interface {
	//当块创建时，可以添加，修改块内信息
	OnNewBlock(bi *BlockIndex, blk *BlockInfo) error
	//完成区块，当检测完成调用,设置merkle之前
	OnFinished(bi *BlockIndex, blk *BlockInfo) error
	//获取签名账户
	GetAccount(bi *BlockIndex, pkh HASH160) (*Account, error)
	//链关闭时
	OnClose(bi *BlockIndex)
	//当一个块连接到链之前
	OnLinkBlock(bi *BlockIndex, blk *BlockInfo)
	//获取钱包
	GetWallet() IWallet
}

//区块发布交易参数
type BlockEvent struct {
	Idx *BlockIndex
	Blk *BlockInfo
}

//交易发布订阅参数
type TxEvent struct {
	Idx *BlockIndex
	Blk *BlockInfo //如果交易在块中blk != nil
	Tx  *TX
}

var (
	midx  *BlockIndex = nil
	monce sync.Once
	pubv  = NewPubSub(10)
	ponce sync.Once
)

//获取全局发布订阅
func GetPubSub() *PubSub {
	ponce.Do(func() {
		LogInfo("global pusbsub init cap=10")
		pubv = NewPubSub(10)
	})
	return pubv
}

//获取全局主链
func GetBlockIndex() *BlockIndex {
	if midx == nil {
		panic(errors.New("block index not init"))
	}
	return midx
}

//初始化主链
func InitBlockIndex(lis IListener) *BlockIndex {
	if conf == nil {
		panic(errors.New("config not init"))
	}
	monce.Do(func() {
		bi := NewBlockIndex(lis)
		err := bi.LoadAll(func(pv uint) {
			LogInfof("load block chian progress = %d%%", pv)
		})
		if err == EmptyBlockChain {
			LogError(err)
		} else if err == ArriveFirstBlock {
			LogError(err)
		} else if err != nil {
			panic(err)
		}
		midx = bi
	})
	return midx
}

//区块链索引
type BlockIndex struct {
	//交易池
	tp *TxPool
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
	lru *IndexCacher
	//存储和索引
	db IBlkStore
}

//获取当前监听器
func (bi *BlockIndex) GetListener() IListener {
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
	blk := &BlockInfo{}
	blk.Header.Ver = ver
	blk.Header.Time = uint32(time.Now().Unix())
	nexth := InvalidHeight
	//设置当前难度
	if last := bi.Last(); last == nil {
		blk.Header.Bits = GetMinPowBits()
		nexth = 0
	} else if lid, err := last.ID(); err != nil {
		return nil, err
	} else {
		nexth = last.Height + 1
		blk.Header.Prev = lid
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

func (bi *BlockIndex) LastHeight() uint32 {
	last := bi.Last()
	if last == nil {
		return 0
	}
	return last.Height
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
	if bi.cur == nil {
		panic(errors.New("cur nil"))
	}
	return bi.cur.Value.(*TBEle)
}

//获取区块的确认数
func (bi *BlockIndex) GetBlockConfirm(id HASH256) int {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	cele, has := bi.imap[id]
	if !has {
		return 0
	}
	cmeta := cele.Value.(*TBEle)
	lele := bi.lis.Back()
	if lele == nil {
		return 0
	}
	lmeta := lele.Value.(*TBEle)
	return int(lmeta.Height-cmeta.Height) + 1
}

//获取交易确认数(所属区块的确认数)
func (bi *BlockIndex) GetTxConfirm(id HASH256) int {
	txv, err := bi.LoadTxValue(id)
	if err != nil {
		return 0
	}
	return bi.GetBlockConfirm(txv.BlkId)
}

//加载区块
func (bi *BlockIndex) LoadBlock(id HASH256) (*BlockInfo, error) {
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

//重置光标到某个位置
func (bi *BlockIndex) SeekTo(v ...interface{}) bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	if len(v) == 0 {
		bi.cur = nil
		return false
	}
	switch v[0].(type) {
	case uint32:
		h := v[0].(uint32)
		bi.cur = bi.hmap[h]
	case int:
		h := uint32(v[0].(int))
		bi.cur = bi.hmap[h]
	case uint:
		h := uint32(v[0].(uint))
		bi.cur = bi.hmap[h]
	case HASH256:
		id := v[0].(HASH256)
		bi.cur = bi.imap[id]
	default:
		id := NewHASH256(v[0])
		bi.cur = bi.imap[id]
	}
	return bi.cur != nil
}

//上一个
func (bi *BlockIndex) Prev() bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	if bi.cur == bi.lis.Front() {
		return false
	}
	if bi.cur == nil {
		bi.cur = bi.lis.Back()
		return bi.cur != nil
	} else {
		bi.cur = bi.cur.Prev()
		return bi.cur != nil
	}
}

//下一个
func (bi *BlockIndex) Next() bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	if bi.cur == bi.lis.Back() {
		return false
	}
	if bi.cur == nil {
		bi.cur = bi.lis.Front()
		return bi.cur != nil
	} else {
		bi.cur = bi.cur.Next()
		return bi.cur != nil
	}
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
	id, err := tv.ID()
	if err != nil {
		return err
	}
	delete(bi.imap, id)
	bi.lis.Remove(le)
	return nil
}

func (bi *BlockIndex) pushback(e *TBEle) (*TBEle, error) {
	ele := bi.lis.PushBack(e)
	bi.hmap[e.Height] = ele
	if id, err := e.ID(); err != nil {
		return nil, err
	} else {
		bi.imap[id] = ele
	}
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
	if id, err := e.ID(); err != nil {
		return nil, err
	} else {
		bi.imap[id] = ele
	}
	return e, nil
}

//加载所有链meta
//f进度回调 0-100
func (bi *BlockIndex) LoadAll(fn func(pv uint)) error {
	LogInfo("start load main chain block header")
	hh := InvalidHeight
	vv := uint(0)
	//加载所有区块头
	for i := 0; ; i++ {
		ele, err := bi.LoadPrev()
		if err == ArriveFirstBlock {
			break
		}
		if err == EmptyBlockChain {
			break
		}
		if err != nil {
			return fmt.Errorf("load block header error %w", err)
		}
		if hh == InvalidHeight {
			hh = ele.Height + 1
		}
		p := 1 - (float32(ele.Height) / float32(hh))
		cv := uint((p * 100))
		if cv == vv {
			continue
		}
		if fn != nil && cv > 0 {
			fn(cv)
		}
		vv = cv
	}
	//重置光标
	bi.SeekTo()
	//验证最后6个块
	for i := 0; bi.Prev() && i < 6; i++ {
		ele := bi.Current()
		eleid, err := ele.ID()
		if err != nil {
			return err
		}
		bp, err := bi.LoadBlock(eleid)
		if err != nil {
			return fmt.Errorf("verify block %v error %w", ele, err)
		}
		err = bp.Check(bi, true)
		if err != nil {
			return fmt.Errorf("verify block %v error %w", ele, err)
		}
	}
	LogInfo("load finished block count = ", bi.Len())
	return nil
}

//转账交易
//从acc账号转向addr地址 金额:amt，交易费:fee
func (bi *BlockIndex) Transfer(src Address, addr Address, amt Amount, fee Amount) (*TX, error) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	if !fee.IsRange() || amt == 0 || !amt.IsRange() {
		return nil, errors.New("amount zero or fee error")
	}
	spkh, err := src.GetPkh()
	if err != nil {
		return nil, err
	}
	dpkh, err := addr.GetPkh()
	if err != nil {
		return nil, err
	}
	acc, err := bi.lptr.GetAccount(bi, spkh)
	if err != nil {
		return nil, err
	}
	ds, err := bi.ListCoinsWithID(spkh)
	if err != nil {
		return nil, err
	}
	balance := ds.Balance()
	if (amt + fee) > balance {
		return nil, errors.New("Insufficient balance")
	}
	tx := &TX{}
	tx.Ver = 1
	sum := Amount(0)
	tx.Outs = []*TxOut{}
	//获取需要的输入
	tx.Ins = []*TxIn{}
	for _, cv := range ds {
		//看是否在之前就已经消费
		ctv, err := bi.tp.FindCoin(cv)
		if err == nil {
			return nil, fmt.Errorf("coin cost at txpool id= %v", ctv)
		}
		in, err := cv.NewTxIn(acc)
		if err != nil {
			return nil, err
		}
		tx.Ins = append(tx.Ins, in)
		sum += cv.Value
		if sum >= amt+fee {
			break
		}
	}
	//创建目标输出
	out := &TxOut{}
	out.Value = amt
	if script, err := NewLockedScript(dpkh); err != nil {
		return nil, err
	} else {
		out.Script = script
	}
	tx.Outs = append(tx.Outs, out)
	//找零钱给自己
	if rv := sum - fee - amt; rv > 0 {
		mine := &TxOut{}
		script, err := acc.NewLockedScript()
		if err != nil {
			return nil, err
		}
		mine.Script = script
		mine.Value = rv
		tx.Outs = append(tx.Outs, mine)
	}
	if err := tx.Sign(bi); err != nil {
		return nil, err
	}
	//放入交易池
	if err := bi.tp.PushBack(tx); err != nil {
		return nil, err
	}
	return tx, nil
}

func (bi *BlockIndex) LoadTBEle(id HASH256) (*TBEle, error) {
	ele := &TBEle{idx: bi}
	err := ele.LoadMeta(id)
	return ele, err
}

var (
	//到达第一个
	ArriveFirstBlock = errors.New("arrive first block")
	//空链
	EmptyBlockChain = errors.New("this is empty chain")
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
		return nil, EmptyBlockChain
	} else {
		id = bv.Id
		ih = bv.Height
	}
	ele, err := bi.LoadTBEle(id)
	if err != nil {
		return nil, err
	}
	if ele.Prev.IsZero() {
		//到达第一个
		ele.Height = 0
	} else if fe != nil {
		ele.Height = fe.Value.(*TBEle).Height - 1
	} else {
		//最后一个
		ele.Height = ih
	}
	if _, err := bi.pushfront(ele); err != nil {
		return nil, err
	}
	if ele.Height == 0 {
		return ele, ArriveFirstBlock
	} else {
		return ele, nil
	}
}

//检测是否可以链入尾部,并返回当前高度和当前id
func (bi *BlockIndex) IsLinkBack(meta *TBMeta) (uint32, HASH256, bool) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	last := bi.lis.Back()
	if last == nil {
		return 0, ZERO, true
	}
	lv := last.Value.(*TBEle)
	id, err := lv.ID()
	if err != nil {
		return 0, id, false
	}
	if !meta.Prev.Equal(id) {
		return 0, id, false
	}
	return lv.Height, id, true
}

//加入一个队列尾并设置高度
func (bi *BlockIndex) LinkBack(ele *TBEle) (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	last := bi.lis.Back()
	if last == nil {
		return bi.pushback(ele)
	}
	lv := last.Value.(*TBEle)
	id, err := lv.ID()
	if err != nil {
		return nil, err
	}
	if !ele.Prev.Equal(id) {
		return nil, errors.New("ele prev hash error")
	}
	if lv.Height+1 != ele.Height {
		return nil, errors.New("ele height error")
	}
	return bi.pushback(ele)
}

func (bi *BlockIndex) LoadTX(id HASH256) (*TX, error) {
	//从缓存和区块获取
	hptr := bi.lru.Get(id, func() (size int, value Value) {
		txv, err := bi.LoadTxValue(id)
		if err != nil {
			return 0, nil
		}
		tx, err := txv.GetTX(bi)
		if err != nil {
			return 0, nil
		}
		buf := NewWriter()
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
	vv := &TxValue{}
	vb, err := bi.db.Index().Get(TXS_PREFIX, id[:])
	if err != nil {
		return nil, err
	}
	err = vv.Decode(NewReader(vb))
	if err != nil {
		return nil, err
	}
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
	buf := NewReader(hb)
	if err := meta.Decode(buf); err != nil {
		return nil, err
	}
	bb, err := bi.db.Blk().Read(meta.Blk)
	if err != nil {
		return nil, err
	}
	err = block.Decode(NewReader(bb))
	return meta, err
}

//清除区块相关的缓存
func (bi *BlockIndex) cleancache(b *BlockInfo) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	for _, tv := range b.Txs {
		id, err := tv.ID()
		if err == nil {
			bi.lru.Delete(id)
		}
	}
	if id, err := b.ID(); err != nil {
		log.Println("id error", err)
	} else {
		bi.lru.Delete(id)
	}
}

//断开最后一个
func (bi *BlockIndex) UnlinkLast() error {
	last := bi.Last()
	if last == nil {
		return errors.New("last block miss")
	}
	id, err := last.ID()
	if err != nil {
		return err
	}
	b, err := bi.LoadBlock(id)
	if err != nil {
		return err
	}
	err = bi.Unlink(b)
	if err == nil {
		bi.cleancache(b)
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
	id, err := bp.ID()
	if err != nil {
		return err
	}
	lid, err := bi.Last().ID()
	if err != nil {
		return err
	}
	if !lid.Equal(id) {
		return errors.New("only unlink last block")
	}
	rb, err := bi.db.Rev().Read(bp.Meta.Rev)
	if err != nil {
		return fmt.Errorf("read block rev data error %w", err)
	}
	bt, err := bi.db.Index().LoadBatch(rb)
	if err != nil {
		return fmt.Errorf("load rev batch error %w", err)
	}
	//删除区块头
	bt.Del(BLOCK_PREFIX, id[:])
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

func (bi *BlockIndex) ListCoins(addr Address) (Coins, error) {
	pkh, err := addr.GetPkh()
	if err != nil {
		return nil, err
	}
	return bi.ListCoinsWithID(pkh)
}

//获取某个id的所有积分
func (bi *BlockIndex) ListCoinsWithID(id HASH160) (Coins, error) {
	prefix := getDBKey(COIN_PREFIX, id[:])
	kvs := Coins{}
	iter := bi.db.Index().Iterator(NewPrefix(prefix))
	defer iter.Close()
	for iter.Next() {
		tk := &CoinKeyValue{}
		err := tk.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, tk)
	}
	return kvs, nil
}

//链接一个区块
func (bi *BlockIndex) LinkTo(blk *BlockInfo) error {
	//检测区块
	if err := blk.Check(bi, true); err != nil {
		return err
	}
	cid, meta, bb, err := blk.ToTBMeta()
	if err != nil {
		return err
	}
	nexth := InvalidHeight
	//是否能连接到主链后
	phv, pid, isok := bi.IsLinkBack(meta)
	if !isok {
		return fmt.Errorf("can't link to main chain hash=%v", cid)
	}
	//区块状态写入
	bt := bi.db.Index().NewBatch()
	//设置事物回退
	rt := bt.NewRev()
	//第一个
	if pid.IsZero() {
		nexth = phv
		bv := BestValue{Id: cid, Height: nexth}
		bt.Put(BestBlockKey, bv.Bytes())
	} else {
		nexth = phv + 1
		bv := BestValue{Id: cid, Height: nexth}
		bt.Put(BestBlockKey, bv.Bytes())
		//写回退
		cv := BestValue{Id: pid, Height: phv}
		rt.Put(BestBlockKey, cv.Bytes())
	}
	if err := blk.WriteTxsIdx(bi, bt); err != nil {
		return err
	}
	//检测日志文件
	if bt.Len() > MAX_LOG_SIZE || rt.Len() > MAX_LOG_SIZE {
		return errors.New("opts state logs too big > MAX_LOG_SIZE")
	}
	//保存回退日志
	meta.Rev, err = bi.db.Rev().Write(rt.Dump())
	if err != nil {
		return err
	}
	//保存区块数据
	meta.Blk, err = bi.db.Blk().Write(bb)
	if err != nil {
		return err
	}
	//保存区块头数据
	hbs, err := meta.Bytes()
	if err != nil {
		return err
	}
	bt.Put(BLOCK_PREFIX, cid[:], hbs)
	//连接区块
	ele := NewTBEle(meta, nexth, bi)
	ele, err = bi.LinkBack(ele)
	if err != nil {
		return err
	}
	blk.Meta = ele
	//写入索引数据
	err = bi.db.Index().Write(bt)
	if err == nil {
		bi.lptr.OnLinkBlock(bi, blk)
	} else {
		err = bi.UnlinkBack()
	}
	if err != nil {
		return err
	}
	//删除交易池中存在这个区块中的交易
	for _, tx := range blk.Txs {
		id, err := tx.ID()
		if err == nil {
			bi.tp.Del(id)
		}
	}
	return err
}

func (bi *BlockIndex) GetTxPool() *TxPool {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.tp
}

//关闭链数据
func (bi *BlockIndex) Close() {
	if bi.hmap == nil {
		return
	}
	bi.mu.Lock()
	defer bi.mu.Unlock()
	LogInfo("block index closing")
	bi.lptr.OnClose(bi)
	bi.db.Close()
	bi.lis.Init()
	bi.hmap = nil
	bi.imap = nil
	bi.lru.EvictAll()
	_ = bi.lru.Close()
	LogInfo("block index closed")
}

func NewBlockIndex(lptr IListener) *BlockIndex {
	bi := &BlockIndex{
		tp:   NewTxPool(),
		lptr: lptr,
		lis:  list.New(),
		hmap: map[uint32]*list.Element{},
		imap: map[HASH256]*list.Element{},
		db:   NewLevelDBStore(conf.DataDir),
		lru:  NewIndexCacher(64 * opt.MiB),
	}
	return bi
}
