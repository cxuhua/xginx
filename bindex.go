package xginx

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	TBELoadedMeta  = 1 << 0     //如果已经加载了块头
	TBELoadedBlock = 1 << 1     //如果加载了块数据
	InvalidHeight  = ^uint32(0) //无效的块高度
)

var (
	//到达第一个
	ArriveFirstBlock = errors.New("arrive first block")
	//空链
	EmptyBlockChain = errors.New("this is empty chain")
	//Block数据未下载
	BlockDataEmpty = errors.New("block data empty,not download")
)

//索引头
type TBEle struct {
	flags uint8
	TBMeta
	Height uint32
	bi     *BlockIndex
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
	hb, err := ele.bi.db.Index().Get(BLOCK_PREFIX, id[:])
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

func EmptyTBEle(h uint32, bi *BlockIndex) *TBEle {
	return &TBEle{
		Height: h,
		bi:     bi,
	}
}

func NewTBEle(meta *TBMeta, height uint32, bi *BlockIndex) *TBEle {
	return &TBEle{
		flags:  TBELoadedMeta,
		TBMeta: *meta,
		Height: height,
		bi:     bi,
	}
}

//区块链迭代器
type BIndexIter struct {
	bi  *BlockIndex
	cur *list.Element
	ele *list.Element
}

func (it *BIndexIter) SeekHeight(h uint32, skip ...int) bool {
	it.bi.mu.Lock()
	defer it.bi.mu.Unlock()
	it.cur = nil
	it.ele = it.bi.hmap[h]
	return it.skipEle(skip...)
}

//当前区块高度
func (it *BIndexIter) Height() uint32 {
	ele := it.Curr()
	return ele.Height
}

//当前区块id
func (it *BIndexIter) ID() HASH256 {
	ele := it.Curr()
	id, err := ele.ID()
	if err != nil {
		LogError("get block index iter id error", err)
		return ZERO
	}
	return id
}

//获取当前
func (it *BIndexIter) Curr() *TBEle {
	it.bi.mu.RLock()
	defer it.bi.mu.RUnlock()
	if it.ele != nil {
		return it.ele.Value.(*TBEle)
	}
	if it.cur == nil {
		panic(errors.New("first use next prev seek"))
	}
	return it.cur.Value.(*TBEle)
}

//skip >0  向前跳过
//skip <0 向后跳过
func (it *BIndexIter) skipEle(skip ...int) bool {
	if it.ele == nil || len(skip) == 0 || skip[0] == 0 {
		return it.ele != nil
	}
	skipv, rev := skip[0], false
	if skipv < 0 {
		skipv = -skipv
		rev = true
	}
	for skipv > 0 && it.ele != nil {
		if rev {
			it.ele = it.ele.Prev()
		} else {
			it.ele = it.ele.Next()
		}
		skipv--
	}
	return it.ele != nil
}

func (it *BIndexIter) SeekID(id HASH256, skip ...int) bool {
	it.bi.mu.Lock()
	defer it.bi.mu.Unlock()
	it.cur = nil
	it.ele = it.bi.imap[id]
	return it.skipEle(skip...)
}

func (it *BIndexIter) Prev() bool {
	it.bi.mu.Lock()
	defer it.bi.mu.Unlock()
	if it.ele != nil {
		it.cur = it.ele
		it.ele = nil
	} else if it.cur != nil {
		it.cur = it.cur.Prev()
	} else {
		it.cur = it.bi.lis.Back()
	}
	return it.cur != nil
}

func (it *BIndexIter) Next() bool {
	it.bi.mu.Lock()
	defer it.bi.mu.Unlock()
	if it.ele != nil {
		it.cur = it.ele
		it.ele = nil
	} else if it.cur != nil {
		it.cur = it.cur.Next()
	} else {
		it.cur = it.bi.lis.Front()
	}
	return it.cur != nil
}

func (it *BIndexIter) First(skip ...int) bool {
	it.bi.mu.Lock()
	defer it.bi.mu.Unlock()
	it.cur = nil
	it.ele = it.bi.lis.Front()
	return it.skipEle(skip...)
}

func (it *BIndexIter) Last(skip ...int) bool {
	it.bi.mu.Lock()
	defer it.bi.mu.Unlock()
	it.cur = nil
	it.ele = it.bi.lis.Back()
	return it.skipEle(skip...)
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
			LogInfof("load block main chian progress = %d%%", pv)
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
	txp  *TxPool                   //交易池
	lptr IListener                 //链监听器
	mu   sync.RWMutex              //
	lis  *list.List                //区块头列表
	hmap map[uint32]*list.Element  //按高度缓存
	imap map[HASH256]*list.Element //按id缓存
	lru  *IndexCacher              //lru缓存
	db   IBlkStore                 //存储和索引
}

func (bi *BlockIndex) CacheSize() int {
	return bi.lru.Size()
}

//获取当前监听器
func (bi *BlockIndex) GetListener() IListener {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.lptr
}

func (bi *BlockIndex) NewIter() *BIndexIter {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	iter := &BIndexIter{bi: bi}
	iter.ele = bi.lis.Back()
	return iter
}

func (bi *BlockIndex) gethele(h uint32) *TBEle {
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
	ele := bi.gethele(ph)
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
	if !CheckProofOfWorkBits(blk.Header.Bits) {
		return nil, errors.New("block bits check error")
	}
	SetRandInt(&blk.Header.Nonce)
	ele := EmptyTBEle(nexth, bi)
	ele.TBMeta.BlockHeader = blk.Header
	blk.Meta = ele
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

func (bi *BlockIndex) BestHeight() uint32 {
	bv := bi.GetBestValue()
	if !bv.IsValid() {
		return InvalidHeight
	}
	bi.mu.RLock()
	ele := bi.imap[bv.Id]
	bi.mu.RUnlock()
	if ele != nil {
		return ele.Value.(*TBEle).Height
	}
	return InvalidHeight
}

func (bi *BlockIndex) GetNodeHeight() BHeight {
	nh := BHeight{}
	nh.BH = bi.BestHeight()
	nh.HH = bi.LastHeight()
	return nh
}

func (bi *BlockIndex) LastHeight() uint32 {
	last := bi.Last()
	if last == nil {
		return InvalidHeight
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
	txv, err := bi.loadTxValue(id)
	if err != nil {
		return 0
	}
	return bi.GetBlockConfirm(txv.BlkId)
}

//加载区块
func (bi *BlockIndex) LoadBlock(id HASH256) (*BlockInfo, error) {
	var rerr error = nil
	hptr := bi.lru.Get(id, func() (size int, value Value) {
		ele, has := bi.imap[id]
		if !has {
			return 0, nil
		}
		smeta := ele.Value.(*TBEle)
		bptr := &BlockInfo{}
		lmeta, err := bi.loadTo(id, bptr)
		if err != nil {
			rerr = err
			return 0, nil
		}
		if !lmeta.Blk.HasData() {
			rerr = BlockDataEmpty
			return 0, nil
		}
		if !lmeta.Hash().Equal(smeta.Hash()) {
			return 0, nil
		}
		bptr.Meta = smeta
		return smeta.Blk.Len.ToInt(), bptr
	})
	if rerr != nil {
		return nil, rerr
	}
	if hptr == nil {
		return nil, errors.New("load block failed")
	}
	return hptr.Value().(*BlockInfo), nil
}

//断开最后一个
func (bi *BlockIndex) unlinkback() error {
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

func (bi *BlockIndex) pushback(e *TBEle) error {
	ele := bi.lis.PushBack(e)
	bi.hmap[e.Height] = ele
	if id, err := e.ID(); err != nil {
		return err
	} else {
		bi.imap[id] = ele
	}
	return nil
}

func (bi *BlockIndex) pushfront(e *TBEle) (*TBEle, error) {
	id, err := e.ID()
	if err != nil {
		return nil, err
	}
	fh := bi.lis.Front()
	if fh != nil && !fh.Value.(*TBEle).Prev.Equal(id) {
		return nil, errors.New("push error id to front")
	}
	ele := bi.lis.PushFront(e)
	bi.hmap[e.Height] = ele
	bi.imap[id] = ele
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
		ele, err := bi.loadPrev()
		if err == ArriveFirstBlock {
			break
		}
		if err == EmptyBlockChain {
			LogInfo("load finished, empty block chain")
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
	if bi.Len() == 0 {
		return nil
	}
	lnum := 50
	if lnum > bi.Len() {
		lnum = bi.Len()
	}
	//验证最后6个块
	LogInfof("verify last %d block start", lnum)
	for iter, i := bi.NewIter(), 0; iter.Prev() && i < lnum; i++ {
		ele := iter.Curr()
		//没数据不验证
		if !ele.HasBlk() {
			break
		}
		eid, err := ele.ID()
		if err != nil {
			return err
		}
		bp, err := bi.LoadBlock(eid)
		if err != nil {
			return fmt.Errorf("verify block %v error %w", ele, err)
		}
		err = bp.Check(bi, true)
		if err != nil {
			return fmt.Errorf("verify block %v error %w", ele, err)
		}
		LogInfof("verify block success id = %v height = %d #%d", iter.ID(), iter.Height(), lnum-i)
	}
	bv := bi.GetBestValue()
	LogInfo("load finished block count = ", bi.Len(), ",last height =", bi.LastHeight(), ",best height =", bv.Height)
	return nil
}

//是否有需要下载的区块
func (bi *BlockIndex) HasSync() bool {
	last := bi.Last()
	return last != nil && !last.HasBlk()
}

//获取回退代价，也就是回退多少个
func (bi *BlockIndex) UnlinkCount(id HASH256) (uint32, error) {
	if id.IsZero() {
		return bi.LastHeight() + 1, nil
	}
	ele, err := bi.getEle(id)
	if err != nil {
		return 0, errors.New("not found id")
	}
	return bi.LastHeight() - ele.Height, nil
}

//回退到指定id
func (bi *BlockIndex) UnlinkTo(id HASH256) error {
	count, err := bi.UnlinkCount(id)
	if err != nil {
		return err
	}
	for ; count > 0; count-- {
		err := bi.UnlinkLast()
		if err != nil {
			return err
		}
	}
	return nil
}

//转账交易
//从acc账号转向addr地址 金额:amt，交易费:fee
func (bi *BlockIndex) Transfer(src Address, addr Address, amt Amount, fee Amount) (*TX, error) {
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
		ctv, err := bi.txp.FindCoin(cv)
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
	if err := bi.txp.PushBack(tx); err != nil {
		return nil, err
	}
	return tx, nil
}

//获取区块头
func (bi *BlockIndex) loadtbele(id HASH256) (*TBEle, error) {
	ele := &TBEle{bi: bi}
	err := ele.LoadMeta(id)
	return ele, err
}

//向前加载一个区块数据头
func (bi *BlockIndex) loadPrev() (*TBEle, error) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	fe := bi.lis.Front()
	id := HASH256{}
	ih := uint32(0)
	if fe != nil {
		id = fe.Value.(*TBEle).Prev
	} else if bv := bi.GetLastValue(); !bv.IsValid() {
		return nil, EmptyBlockChain
	} else {
		id = bv.Id
		ih = bv.Height
	}
	ele, err := bi.loadtbele(id)
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
func (bi *BlockIndex) islinkback(meta *TBMeta) (uint32, HASH256, bool) {
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
func (bi *BlockIndex) linkback(ele *TBEle) error {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	last := bi.lis.Back()
	if last == nil {
		return bi.pushback(ele)
	}
	lv := last.Value.(*TBEle)
	id, err := lv.ID()
	if err != nil {
		return err
	}
	if !ele.Prev.Equal(id) {
		return errors.New("ele prev hash error")
	}
	if lv.Height+1 != ele.Height {
		return errors.New("ele height error")
	}
	return bi.pushback(ele)
}

func (bi *BlockIndex) LoadTX(id HASH256) (*TX, error) {
	//从缓存和区块获取
	hptr := bi.lru.Get(id, func() (size int, value Value) {
		txv, err := bi.loadTxValue(id)
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

func (bi *BlockIndex) loadTxValue(id HASH256) (*TxValue, error) {
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
func (bi *BlockIndex) loadTo(id HASH256, blk *BlockInfo) (*TBMeta, error) {
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
	if !meta.HasBlk() {
		return nil, BlockDataEmpty
	}
	bb, err := bi.db.Blk().Read(meta.Blk)
	if err != nil {
		return nil, err
	}
	err = blk.Decode(NewReader(bb))
	return meta, err
}

//清除区块相关的缓存
func (bi *BlockIndex) cleancache(b *BlockInfo) {
	for _, tv := range b.Txs {
		id, err := tv.ID()
		if err == nil {
			bi.lru.Delete(id)
		}
	}
	if id, err := b.ID(); err == nil {
		bi.lru.Delete(id)
	}
}

//设置最后的头信息
func (bi *BlockIndex) setLastHeader(bt *Batch) error {
	if iter := bi.NewIter(); iter.Prev() && iter.Prev() {
		ele := iter.Curr()
		pid, err := ele.ID()
		if err != nil {
			return err
		}
		bv := BestValue{Id: pid, Height: ele.Height}
		bt.Put(LastHeaderKey, bv.Bytes())
	} else {
		bt.Del(LastHeaderKey)
	}
	return nil
}

//只有头断开头
func (bi *BlockIndex) unlinkLastEle(ele *TBEle) error {
	id, err := ele.ID()
	if err != nil {
		return err
	}
	bt := bi.db.Index().NewBatch()
	bt.Del(BLOCK_PREFIX, id[:])
	err = bi.setLastHeader(bt)
	if err != nil {
		return err
	}
	err = bi.db.Index().Write(bt)
	if err != nil {
		return err
	}
	return bi.unlinkback()
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
	//如果没有下载区块数据直接断开区块头
	if !last.Blk.HasData() {
		return bi.unlinkLastEle(last)
	}
	blk, err := bi.LoadBlock(id)
	if err != nil {
		return err
	}
	err = bi.unlink(blk)
	if err == nil {
		bi.cleancache(blk)
	}
	LogInfo("unlink block", id, "success")
	return err
}

//断开一个区块
func (bi *BlockIndex) unlink(bp *BlockInfo) error {
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
	//len !=0 肯定存在last
	lid, err := bi.Last().ID()
	if err != nil {
		return err
	}
	//是否是最后一个
	if !lid.Equal(id) {
		return errors.New("only unlink last block")
	}
	//读取回退数据
	rb, err := bi.db.Rev().Read(bp.Meta.Rev)
	if err != nil {
		return fmt.Errorf("read block rev data error %w", err)
	}
	bt, err := bi.db.Index().LoadBatch(rb)
	if err != nil {
		return fmt.Errorf("load rev batch error %w", err)
	}
	//回退头
	err = bi.setLastHeader(bt)
	if err != nil {
		return err
	}
	//回退后会由回退数据设置bestvalue
	//删除区块头
	bt.Del(BLOCK_PREFIX, id[:])
	if err := bi.db.Index().Write(bt); err != nil {
		return err
	}
	//断开链接
	return bi.unlinkback()
}

//获取最高块信息
func (bi *BlockIndex) GetLastValue() BestValue {
	bv := BestValue{}
	b, err := bi.db.Index().Get(LastHeaderKey)
	if err != nil {
		return InvalidBest
	}
	if err := bv.From(b); err != nil {
		return InvalidBest
	}
	return bv
}

//获取下个需要同步的区块
func (bi *BlockIndex) GetNextSync() (*TBEle, error) {
	bv := bi.GetBestValue()
	if !bv.IsValid() {
		return bi.First(), nil
	}
	bi.mu.RLock()
	ele := bi.imap[bv.Id]
	if ele != nil {
		ele = ele.Next()
	}
	bi.mu.RUnlock()
	if ele == nil {
		return nil, errors.New("not found")
	}
	return ele.Value.(*TBEle), nil
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
	bi.mu.RLock()
	defer bi.mu.RUnlock()
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

//是否存在
func (bi *BlockIndex) HasBlock(id HASH256) bool {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	_, has := bi.imap[id]
	return has
}

func (bi *BlockIndex) getEle(id HASH256) (*TBEle, error) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	eptr, has := bi.imap[id]
	if !has {
		return nil, errors.New("not found")
	}
	return eptr.Value.(*TBEle), nil
}

//更新区块数据(需要区块头先链接好
func (bi *BlockIndex) UpdateBlk(blk *BlockInfo) error {
	cid, err := blk.ID()
	if err != nil {
		return err
	}
	buf := NewWriter()
	if err := blk.Encode(buf); err != nil {
		return err
	}
	if buf.Len() > MAX_BLOCK_SIZE {
		return errors.New("block size too big")
	}
	//获取区块头
	ele, err := bi.getEle(cid)
	if err != nil {
		return err
	}
	eid, err := ele.ID()
	if err != nil {
		return err
	}
	if !cid.Equal(eid) {
		return errors.New("blk id != header id")
	}
	blk.Meta = ele
	//设置交易数量
	blk.Meta.Txs = VarUInt(len(blk.Txs))
	//检测区块数据
	err = blk.Check(bi, true)
	if err != nil {
		return err
	}
	bt := bi.db.Index().NewBatch()
	rt := bt.NewRev()
	//是否能更新到best之后
	if bv := bi.GetBestValue(); !bv.IsValid() {
		blk.Meta.Height = 0
		bt.Put(BestBlockKey, BestValueBytes(cid, 0))
	} else if !blk.Meta.Prev.Equal(bv.Id) {
		return errors.New("prev hash id error,can't update blk")
	} else {
		bt.Put(BestBlockKey, BestValueBytes(cid, blk.Meta.Height))
		rt.Put(BestBlockKey, bv.Bytes())
	}
	if err := blk.CheckCoinbase(); err != nil {
		return err
	}
	if err := blk.WriteTxsIdx(bi, bt); err != nil {
		return err
	}
	//检测日志文件
	if bt.Len() > MAX_LOG_SIZE || rt.Len() > MAX_LOG_SIZE {
		return errors.New("opts state logs too big > MAX_LOG_SIZE")
	}
	//保存回退日志
	blk.Meta.Rev, err = bi.db.Rev().Write(rt.Dump())
	if err != nil {
		return err
	}
	//保存区块数据
	blk.Meta.Blk, err = bi.db.Blk().Write(buf.Bytes())
	if err != nil {
		return err
	}
	//保存区块头数据
	hbs, err := blk.Meta.Bytes()
	if err != nil {
		return err
	}
	bt.Put(BLOCK_PREFIX, cid[:], hbs)
	//写入索引数据
	err = bi.db.Index().Write(bt)
	if err != nil {
		return err
	}
	//删除交易池中存在这个区块中的交易
	for _, tx := range blk.Txs {
		id, err := tx.ID()
		if err == nil {
			bi.txp.Del(id)
		}
	}
	return nil
}

//检测如果是第一个，必须是genesis块,正式发布会生成genesis块
func (bi *BlockIndex) checkGenesis(header BlockHeader) error {
	id, err := header.ID()
	if err != nil {
		return err
	}
	if bi.Len() == 0 && !id.Equal(conf.genesis) {
		return errors.New("first not genesis id")
	}
	return nil
}

//连接区块头
func (bi *BlockIndex) LinkHeader(header BlockHeader) (*TBEle, error) {
	meta := &TBMeta{
		BlockHeader: header,
	}
	cid, err := meta.ID()
	if err != nil {
		return nil, err
	}
	if !CheckProofOfWork(cid, meta.Bits) {
		return nil, errors.New("block header bits check error")
	}
	nexth := InvalidHeight
	//是否能连接到主链后
	phv, pid, isok := bi.islinkback(meta)
	if !isok {
		return nil, fmt.Errorf("can't link to chain last, hash=%v", cid)
	}
	if pid.IsZero() {
		nexth = phv
	} else {
		nexth = phv + 1
	}
	bt := bi.db.Index().NewBatch()
	//保存区块头数据
	hbs, err := meta.Bytes()
	if err != nil {
		return nil, err
	}
	//保存区块头
	bt.Put(BLOCK_PREFIX, cid[:], hbs)
	//保存最后一个头
	bh := BestValue{Height: nexth, Id: cid}
	bt.Put(LastHeaderKey, bh.Bytes())
	//保存数据
	err = bi.db.Index().Write(bt)
	if err != nil {
		return nil, err
	}
	ele := NewTBEle(meta, nexth, bi)
	return ele, bi.linkback(ele)
}

func (bi *BlockIndex) GetTxPool() *TxPool {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.txp
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
		txp:  NewTxPool(),
		lptr: lptr,
		lis:  list.New(),
		hmap: map[uint32]*list.Element{},
		imap: map[HASH256]*list.Element{},
		db:   NewLevelDBStore(conf.DataDir),
		lru:  NewIndexCacher(64 * opt.MiB),
	}
	return bi
}
