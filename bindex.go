package xginx

import (
	"container/list"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"sync"

	lru "github.com/hashicorp/golang-lru"

	"github.com/syndtr/goleveldb/leveldb/opt"
)

//错误定义
var (
	//ErrArriveFirstBlock 到达第一个
	ErrArriveFirstBlock = errors.New("arrive first block")
	//ErrEmptyBlockChain 空链
	ErrEmptyBlockChain = errors.New("this is empty chain")
	//ErrHeadersScope 当获取到的区块头在链中无法找到时
	ErrHeadersScope = errors.New("all hds not in scope")
	//ErrHeadersTooLow 证据区块头太少
	ErrHeadersTooLow = errors.New("headers too low")
)

//TBEle 索引头
type TBEle struct {
	TBMeta
	Height uint32
	bi     *BlockIndex
}

func (ele TBEle) String() string {
	id, err := ele.ID()
	if err != nil {
		panic(err)
	}
	return id.String()
}

//LoadMeta 从磁盘加载块头
func (ele *TBEle) LoadMeta(id HASH256) error {
	hb, err := ele.bi.blkdb.Index().Get(BlockPrefix, id[:])
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
	return nil
}

//EmptyTBEle 创建一个空的链表结构
func EmptyTBEle(h uint32, bh BlockHeader, bi *BlockIndex) *TBEle {
	return &TBEle{
		Height: h,
		bi:     bi,
		TBMeta: TBMeta{BlockHeader: bh},
	}
}

//NewTBEle 创建一个链表结构
func NewTBEle(meta *TBMeta, height uint32, bi *BlockIndex) *TBEle {
	return &TBEle{
		TBMeta: *meta,
		Height: height,
		bi:     bi,
	}
}

//BIndexIter 区块链迭代器
type BIndexIter struct {
	bi  *BlockIndex
	cur *list.Element
	ele *list.Element
}

//SeekHeight 定位到某个高度
func (it *BIndexIter) SeekHeight(h uint32, skip ...int) bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
	it.cur = nil
	it.ele = it.bi.hmap[h]
	return it.skipEle(skip...)
}

//Height 当前区块高度
func (it *BIndexIter) Height() uint32 {
	ele := it.Curr()
	return ele.Height
}

//ID 当前区块id
func (it *BIndexIter) ID() HASH256 {
	ele := it.Curr()
	id, err := ele.ID()
	if err != nil {
		LogError("get block index iter id error", err)
		return ZERO256
	}
	return id
}

//Curr 获取当前区块头
func (it *BIndexIter) Curr() *TBEle {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
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
	var ele *list.Element
	for skipv > 0 && it.ele != nil {
		if rev {
			ele = it.ele.Prev()
		} else {
			ele = it.ele.Next()
		}
		//如果移动到头或者尾
		if ele == nil {
			break
		}
		it.ele = ele
		skipv--
	}
	return it.ele != nil
}

//SeekID 定位到某个区块Id
func (it *BIndexIter) SeekID(id HASH256, skip ...int) bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
	it.cur = nil
	it.ele = it.bi.imap[id]
	return it.skipEle(skip...)
}

//Prev 上一个区块
func (it *BIndexIter) Prev() bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
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

//Next 下一个区块
func (it *BIndexIter) Next() bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
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

//First 第一个区块
func (it *BIndexIter) First(skip ...int) bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
	it.cur = nil
	it.ele = it.bi.lis.Front()
	return it.skipEle(skip...)
}

//Last 最后一个区块
func (it *BIndexIter) Last(skip ...int) bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
	it.cur = nil
	it.ele = it.bi.lis.Back()
	return it.skipEle(skip...)
}

//BlockEvent 区块发布交易参数
type BlockEvent struct {
	Idx *BlockIndex
	Blk *BlockInfo
}

//TxEvent 交易发布订阅参数
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

//GetPubSub 获取全局发布订阅
func GetPubSub() *PubSub {
	ponce.Do(func() {
		LogInfo("global pusbsub init cap=10")
		pubv = NewPubSub(10)
	})
	return pubv
}

//GetBlockIndex 获取全局主链
func GetBlockIndex() *BlockIndex {
	if midx == nil {
		panic(errors.New("block index not init"))
	}
	return midx
}

//链接加入创世块
func LinkGenesis(bi *BlockIndex) {
	dat, err := ioutil.ReadFile("genesis.blk")
	if err != nil {
		LogInfo("read genesis.blk error", err)
		return
	}
	buf := NewReader(dat)
	blk := &BlockInfo{}
	err = blk.Decode(buf)
	if err != nil {
		panic(err)
	}
	err = bi.LinkBlk(blk)
	if err != nil {
		panic(err)
	}
	if !blk.IsGenesis() {
		log.Panic("block not config genesis")
	}
	LogInfo("load link genesis block ", blk)
}

//InitBlockIndex 初始化主链
func InitBlockIndex(lis IListener) *BlockIndex {
	if conf == nil {
		panic(errors.New("config not init"))
	}
	monce.Do(func() {
		bi := NewBlockIndex(lis)
		err := lis.OnInit(bi)
		if err != nil {
			panic(err)
		}
		err = bi.LoadAll(-1, func(pv uint) {
			LogInfof("load block main chian progress = %d%%", pv)
		})
		if err == ErrEmptyBlockChain {
			LinkGenesis(bi)
		} else if err == ErrArriveFirstBlock {
			LogError(err)
		} else if err != nil {
			panic(err)
		}
		midx = bi
	})
	return midx
}

//BlockIndex 区块链结构
type BlockIndex struct {
	txp   *TxPool                   //交易池
	lptr  IListener                 //链监听器
	rwm   sync.RWMutex              //读写锁
	lis   *list.List                //区块头列表
	hmap  map[uint32]*list.Element  //按高度缓存
	imap  map[HASH256]*list.Element //按id缓存
	lru   *lru.Cache                //lru缓存
	blkdb IBlkStore                 //区块存储和索引
}

//NewMsgTxMerkle 返回某个交易的merkle验证树
func (bi *BlockIndex) NewMsgTxMerkle(id HASH256) (*MsgTxMerkle, error) {
	txv, err := bi.LoadTxValue(id)
	if err != nil {
		return nil, err
	}
	blk, err := bi.LoadBlock(txv.BlkID)
	if err != nil {
		return nil, err
	}
	ids := []HASH256{}
	bs := NewBitSet(len(blk.Txs))
	for i, tx := range blk.Txs {
		tid, err := tx.ID()
		if err != nil {
			return nil, err
		}
		if id.Equal(tid) {
			bs.Set(i)
		}
		ids = append(ids, tid)
	}
	tree := NewMerkleTree(len(blk.Txs))
	tree = tree.Build(ids, bs)
	if tree.IsBad() {
		return nil, errors.New("merkle tree bad")
	}
	msg := &MsgTxMerkle{}
	msg.TxID = id
	msg.Hashs = tree.Hashs()
	msg.Trans = VarInt(tree.Trans())
	msg.Bits = tree.Bits().Bytes()
	return msg, nil
}

//CacheSize 获取缓存大小
func (bi *BlockIndex) CacheSize() int {
	return bi.lru.Len()
}

//NewIter 创建一个区块链迭代器
func (bi *BlockIndex) NewIter() *BIndexIter {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	iter := &BIndexIter{bi: bi}
	iter.ele = bi.lis.Back()
	return iter
}

//按高度获取区块
func (bi *BlockIndex) gethele(h uint32) *TBEle {
	ele, has := bi.hmap[h]
	if !has {
		return nil
	}
	return ele.Value.(*TBEle)
}

//计算当前区块高度对应的难度
func (bi *BlockIndex) calcBits(height uint32) uint32 {
	last := bi.last()
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

//CalcBits 计算当前区块高度对应的难度
func (bi *BlockIndex) CalcBits(height uint32) uint32 {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.calcBits(height)
}

//NewBlock 创建下一个高度基本数据
func (bi *BlockIndex) NewBlock(ver uint32) (*BlockInfo, error) {
	blk := &BlockInfo{}
	blk.Header.Ver = ver
	blk.Header.Time = bi.lptr.TimeNow()
	blk.Header.Nonce = RandUInt32()
	bv := bi.GetBestValue()
	//设置当前难度
	if !bv.IsValid() {
		//创世区块
		blk.Header.Prev = ZERO256
		blk.Header.Bits = GetMinPowBits()
	} else {
		blk.Header.Prev = bv.ID
		blk.Header.Bits = bi.CalcBits(bv.Next())
	}
	//检测工作难度
	if !CheckProofOfWorkBits(blk.Header.Bits) {
		return nil, errors.New("block bits check error")
	}
	//创建数据
	blk.Meta = EmptyTBEle(bv.Next(), blk.Header, bi)
	//回调处理
	if err := bi.lptr.OnNewBlock(blk); err != nil {
		return nil, err
	}
	return blk, nil
}

//最低块
func (bi *BlockIndex) first() *TBEle {
	le := bi.lis.Front()
	if le == nil {
		return nil
	}
	return le.Value.(*TBEle)
}

//First 最低块
func (bi *BlockIndex) First() *TBEle {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.first()
}

//NextHeight 下一个块高度
func (bi *BlockIndex) NextHeight() uint32 {
	return bi.GetBestValue().Next()
}

//Height 获取当前链高度
func (bi *BlockIndex) Height() uint32 {
	last := bi.Last()
	if last == nil {
		return 0
	}
	return last.Height
}

//Time 获取当前链最高区块时间，空链获取当前时间
func (bi *BlockIndex) Time() uint32 {
	last := bi.Last()
	if last == nil {
		return bi.lptr.TimeNow()
	}
	return last.Time
}

//BestHeight 保存的最新区块高度
func (bi *BlockIndex) BestHeight() uint32 {
	return bi.GetBestValue().Height
}

func (bi *BlockIndex) lastHeight() uint32 {
	last := bi.last()
	if last == nil {
		return InvalidHeight
	}
	return last.Height
}

//Last 最高块
func (bi *BlockIndex) Last() *TBEle {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.last()
}

//最高块
func (bi *BlockIndex) last() *TBEle {
	le := bi.lis.Back()
	if le == nil {
		return nil
	}
	return le.Value.(*TBEle)
}

//Len 链长度
func (bi *BlockIndex) Len() int {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.lis.Len()
}

//GetBlockHeader 获取块头
func (bi *BlockIndex) GetBlockHeader(id HASH256) (*TBEle, error) {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	ele, has := bi.imap[id]
	if !has {
		return nil, errors.New("not found")
	}
	return ele.Value.(*TBEle), nil
}

//GetBlockConfirm 获取区块的确认数
func (bi *BlockIndex) GetBlockConfirm(id HASH256) int {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
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

//GetTxConfirm 获取交易确认数(所属区块的确认数)
func (bi *BlockIndex) GetTxConfirm(id HASH256) int {
	txv, err := bi.LoadTxValue(id)
	if err != nil {
		return 0
	}
	return bi.GetBlockConfirm(txv.BlkID)
}

//LoadWithHeight 按高度查询区块
func (bi *BlockIndex) LoadBlockWithH(h int) (*BlockInfo, error) {
	bi.rwm.RLock()
	ele := bi.gethele(uint32(h))
	bi.rwm.RUnlock()
	if ele == nil {
		return nil, fmt.Errorf("not found height %d", h)
	}
	id, err := ele.ID()
	if err != nil {
		return nil, err
	}
	return bi.LoadBlock(id)
}

//LoadBlock 加载区块
func (bi *BlockIndex) LoadBlock(id HASH256) (*BlockInfo, error) {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.loadblock(id)
}

//LoadBlock 加载区块
func (bi *BlockIndex) loadblock(id HASH256) (*BlockInfo, error) {
	hptr, ok := bi.lru.Get(id)
	if ok {
		return hptr.(*BlockInfo), nil
	}
	ele, has := bi.imap[id]
	if !has {
		return nil, fmt.Errorf("id %v miss", id)
	}
	smeta := ele.Value.(*TBEle)
	bptr := &BlockInfo{}
	lmeta, err := bi.loadTo(id, bptr)
	if err != nil {
		return nil, err
	}
	if !lmeta.Hash().Equal(smeta.Hash()) {
		return nil, fmt.Errorf("load blockinfo err hash error")
	}
	bptr.Meta = smeta
	bi.lru.Add(id, bptr)
	return bptr, nil
}

//断开最后一个内存中的头
func (bi *BlockIndex) unlinkback() {
	//获取最后一个对象
	le := bi.lis.Back()
	if le == nil {
		return
	}
	tv := le.Value.(*TBEle)
	//移除索引
	delete(bi.hmap, tv.Height)
	delete(bi.imap, tv.MustID())
	bi.lis.Remove(le)
}

//LinkBack 连接区块头
func (bi *BlockIndex) LinkBack(e *TBEle) {
	bi.rwm.Lock()
	defer bi.rwm.Unlock()
	ele := bi.lis.PushBack(e)
	bi.hmap[e.Height] = ele
	bi.imap[e.MustID()] = ele
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

//LoadAll 加载所有链meta
//f进度回调 0-100
func (bi *BlockIndex) LoadAll(limit int, fn func(pv uint)) error {
	LogInfo("start load main chain block header")
	hh := InvalidHeight
	vv := uint(0)
	//加载所有区块头
	for i := 0; limit < 0 || i < limit; i++ {
		ele, err := bi.LoadPrev()
		if err == ErrArriveFirstBlock {
			break
		}
		if err == ErrEmptyBlockChain {
			LogInfo("load finished, empty block chain")
			return err
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
	//验证最后6个块
	lnum := 6
	if lnum > bi.Len() {
		lnum = bi.Len()
	}
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
		err = bp.Verify(ele, bi)
		if err != nil {
			return fmt.Errorf("verify block %v error %w", ele, err)
		}
		LogInfof("verify block success id = %v height = %d #%d", iter.ID(), iter.Height(), lnum-i)
	}
	bv := bi.GetBestValue()
	LogInfof("load finished block , best height=%d,best id=%v", bv.Height, bv.ID)
	return bi.txp.Load(bi, TxPoolFile)
}

//HasSync 是否有需要下载的区块
func (bi *BlockIndex) HasSync() bool {
	last := bi.Last()
	return last != nil && !last.HasBlk()
}

//获取回退代价，也就是回退多少个
func (bi *BlockIndex) unlinkCount(id HASH256) (uint32, error) {
	ele, err := bi.getEle(id)
	if err != nil {
		return 0, errors.New("not found id")
	}
	return bi.lastHeight() - ele.Height, nil
}

//UnlinkCount 返回需要断开的区块数量
func (bi *BlockIndex) UnlinkCount(id HASH256) (uint32, error) {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.unlinkCount(id)
}

//检测连续的区块头列表是否有效
func (bi *BlockIndex) checkHeaders(hs []BlockHeader) error {
	if len(hs) == 0 {
		return nil
	}
	pv := hs[0]
	if err := pv.Check(); err != nil {
		return err
	}
	for i := 1; i < len(hs); i++ {
		cv := hs[i]
		if err := cv.Check(); err != nil {
			return err
		}
		//时间必须连续
		if cv.Time < pv.Time {
			return errors.New("time not continue")
		}
		//id必须能连接
		if !cv.Prev.Equal(pv.MustID()) {
			return errors.New("headers not continue")
		}
		pv = cv
	}
	return nil
}

//Unlink 根据证据区块链修正本地链,回退到一个指定id重新链接
func (bi *BlockIndex) Unlink(hds Headers) error {
	if len(hds) == 0 {
		return errors.New("empty headers")
	}
	iter := bi.NewIter()
	//最后匹配的高度和id
	lp := NewInvalidBest()
	//产生分叉后剩余的区块
	ls := Headers{}
	for i, v := range hds {
		id, err := v.ID()
		if err != nil {
			return err
		}
		//无法定位找到了分叉点
		if !iter.SeekID(id) {
			ls = hds[i:]
			break
		}
		lp.Height = iter.Curr().Height
		lp.ID = id
	}
	//所有的区块头都不在链中
	if !lp.IsValid() {
		return ErrHeadersScope
	}
	//所有区块头都在链中
	if len(ls) == 0 {
		return nil
	}
	//从分叉高度开始检测证据区块头是否合法
	err := ls.Check(lp.Height, bi)
	if err != nil {
		return err
	}
	//获取需要回退到id的数量
	num, err := bi.UnlinkCount(lp.ID)
	if err != nil {
		return err
	}
	//如果证据区块头不足
	if num >= uint32(len(ls)) {
		return ErrHeadersTooLow
	}
	//回退到指定的id
	return bi.UnlinkTo(lp.ID)
}

//UnlinkTo 必须从最后开始断开，回退到指定id,不包括id
func (bi *BlockIndex) UnlinkTo(id HASH256) error {
	bi.rwm.Lock()
	defer bi.rwm.Unlock()
	count, err := bi.unlinkCount(id)
	if err != nil {
		return err
	}
	for ; count > 0; count-- {
		err := bi.unlinkLast()
		if err != nil {
			return err
		}
	}
	return nil
}

//获取区块头
func (bi *BlockIndex) loadtbele(id HASH256) (*TBEle, error) {
	ele := &TBEle{bi: bi}
	err := ele.LoadMeta(id)
	return ele, err
}

//LoadPrev 向前加载一个区块数据头
func (bi *BlockIndex) LoadPrev() (*TBEle, error) {
	first := bi.lis.Front()
	var fele *TBEle = nil
	id := ZERO256
	ih := uint32(0)
	if first != nil {
		fele = first.Value.(*TBEle)
		id = fele.Prev
	} else if bv := bi.GetBestValue(); !bv.IsValid() {
		return nil, ErrEmptyBlockChain
	} else {
		id = bv.ID
		ih = bv.Height
	}
	cele, err := bi.loadtbele(id)
	if err != nil {
		return nil, err
	}
	//第一个必须是配置的创世区块
	if cele.Prev.IsZero() && !conf.IsGenesisID(id) {
		return nil, errors.New("genesis block miss")
	}
	if conf.IsGenesisID(id) {
		//到达第一个
		cele.Height = 0
	} else if fele != nil {
		cele.Height = fele.Height - 1
	} else {
		//最后一个
		cele.Height = ih
	}
	if _, err := bi.pushfront(cele); err != nil {
		return nil, err
	}
	if cele.Height == 0 {
		return cele, ErrArriveFirstBlock
	}
	return cele, nil
}

//检测是否可以链入尾部,并返回当前高度和当前id
func (bi *BlockIndex) islinkback(meta *TBMeta) (uint32, HASH256, bool) {
	//获取最后一个区块头
	last := bi.lis.Back()
	if last == nil {
		return 0, ZERO256, true
	}
	ele := last.Value.(*TBEle)
	id, err := ele.ID()
	if err != nil {
		return 0, id, false
	}
	//时间戳检测
	if meta.Time < ele.Time {
		LogError("check islink back time < prev time")
		return 0, id, false
	}
	//prev id检测
	if !meta.Prev.Equal(id) {
		LogError("check islink back previd != lastid")
		return 0, id, false
	}
	return ele.Height, id, true
}

//LoadTX 从链中获取一个交易
func (bi *BlockIndex) LoadTX(id HASH256) (*TX, error) {
	//从缓存和区块获取
	hptr, ok := bi.lru.Get(id)
	if ok {
		return hptr.(*TX), nil
	}
	txv, err := bi.LoadTxValue(id)
	if err != nil {
		return nil, err
	}
	tx, err := txv.GetTX(bi)
	if err != nil {
		return nil, err
	}
	bi.lru.Add(id, tx)
	return tx, nil
}

//HasTxValue 是否存在交易
func (bi *BlockIndex) HasTxValue(id HASH256) (bool, error) {
	return bi.blkdb.Index().Has(TxsPrefix, id[:])
}

//LoadTxValue 获取交易所在的区块和位置
func (bi *BlockIndex) LoadTxValue(id HASH256) (*TxValue, error) {
	vv := &TxValue{}
	vb, err := bi.blkdb.Index().Get(TxsPrefix, id[:])
	if err != nil {
		return nil, err
	}
	err = vv.Decode(NewReader(vb))
	if err != nil {
		return nil, err
	}
	return vv, err
}

//NewMsgGetBlock 创建区块网络消息
func (bi *BlockIndex) NewMsgGetBlock(id HASH256) MsgIO {
	//从磁盘读取区块数据
	bb, err := bi.ReadBlock(id)
	if err != nil {
		return NewMsgError(ErrCodeBlockMiss, err)
	}
	//发送区块数据过去
	return NewMsgBlockBytes(bb)
}

//ReadBlock 读取区块数据
func (bi *BlockIndex) ReadBlock(id HASH256) ([]byte, error) {
	bk := GetDBKey(BlockPrefix, id[:])
	meta := &TBMeta{}
	hb, err := bi.blkdb.Index().Get(bk)
	if err != nil {
		return nil, err
	}
	buf := NewReader(hb)
	if err := meta.Decode(buf); err != nil {
		return nil, err
	}
	if !meta.HasBlk() {
		return nil, errors.New("block data miss")
	}
	return bi.blkdb.Blk().Read(meta.Blk)
}

//加载块数据
func (bi *BlockIndex) loadTo(id HASH256, blk *BlockInfo) (*TBMeta, error) {
	bk := GetDBKey(BlockPrefix, id[:])
	meta := &TBMeta{}
	hb, err := bi.blkdb.Index().Get(bk)
	if err != nil {
		return nil, err
	}
	buf := NewReader(hb)
	if err := meta.Decode(buf); err != nil {
		return nil, err
	}
	if !meta.HasBlk() {
		return nil, errors.New("block data miss")
	}
	bb, err := bi.blkdb.Blk().Read(meta.Blk)
	if err != nil {
		return nil, err
	}
	err = blk.Decode(NewReader(bb))
	return meta, err
}

//清除区块相关的缓存
func (bi *BlockIndex) cleancache(blk *BlockInfo) {
	//清除交易
	for _, tv := range blk.Txs {
		id, err := tv.ID()
		if err != nil {
			continue
		}
		bi.lru.Remove(id)
	}
	//清除区块
	if id, err := blk.ID(); err == nil {
		bi.lru.Remove(id)
	}
}

//UnlinkLast 断开最后一个区块
func (bi *BlockIndex) UnlinkLast() error {
	bi.rwm.Lock()
	defer bi.rwm.Unlock()
	return bi.unlinkLast()
}

//断开最后一个
func (bi *BlockIndex) unlinkLast() error {
	last := bi.last()
	if last == nil {
		return errors.New("last block miss")
	}
	id, err := last.ID()
	if err != nil {
		return err
	}
	blk, err := bi.loadblock(id)
	if err != nil {
		return err
	}
	bi.lptr.OnUnlinkBlock(blk)
	err = bi.unlink(blk)
	if err == nil {
		bi.cleancache(blk)
	}
	return err
}

//断开一个区块
func (bi *BlockIndex) unlink(bp *BlockInfo) error {
	if bi.lis.Len() == 0 {
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
	lid, err := bi.last().ID()
	if err != nil {
		return err
	}
	//是否是最后一个
	if !lid.Equal(id) {
		return errors.New("only unlink last block")
	}
	//读取回退数据
	rb, err := bi.blkdb.Rev().Read(bp.Meta.Rev)
	if err != nil {
		return fmt.Errorf("read block rev data error %w", err)
	}
	bt, err := bi.blkdb.Index().LoadBatch(rb)
	if err != nil {
		return fmt.Errorf("load rev batch error %w", err)
	}
	//回退后会由回退数据设置bestvalue
	//删除区块头
	bt.Del(BlockPrefix, id[:])
	//回退数据
	err = bi.blkdb.Index().Write(bt)
	if err != nil {
		return err
	}
	//断开链接
	bi.unlinkback()
	return nil
}

//NewMsgHeaders 创建证据区块头信息
//默认获取30个区块头，如果分叉超过30个区块需要另外处理
func (bi *BlockIndex) NewMsgHeaders(msg *MsgGetBlock) *MsgHeaders {
	iter := bi.NewIter()
	//返回上次的参数
	rsg := &MsgHeaders{Info: *msg}
	numv := int(msg.Count)
	//向前移动count个
	if !iter.SeekHeight(msg.Next, -numv) {
		return rsg
	}
	//获取最多numv个返回
	for i := numv; iter.Next() && i > 0; i-- {
		rsg.Headers.Add(iter.Curr().BlockHeader)
	}
	return rsg
}

//LastHeaders 获取最后的多少个区块头
func (bi *BlockIndex) LastHeaders(limit int) Headers {
	iter := bi.NewIter()
	hs := Headers{}
	if !iter.Last() {
		return hs
	}
	for i := 0; i < limit && iter.Prev(); i++ {
		hs.Add(iter.Curr().BlockHeader)
	}
	hs.Reverse()
	return hs
}

//RemoveBestValue 移除数据库中的最新区块信息
func (bi *BlockIndex) RemoveBestValue() error {
	return bi.blkdb.Index().Del(BestBlockKey)
}

//GetBestValue 获取最高块信息
func (bi *BlockIndex) GetBestValue() BestValue {
	bv := BestValue{}
	b, err := bi.blkdb.Index().Get(BestBlockKey)
	if err != nil {
		return InvalidBest
	}
	if err := bv.From(b); err != nil {
		return InvalidBest
	}
	return bv
}

//GetCoinWithAddress 从指定地址交易idx和输出索引获取金额信息
func (bi *BlockIndex) GetCoinWithAddress(addr Address, txid HASH256, idx VarUInt) (*CoinKeyValue, error) {
	pkh, err := addr.GetPkh()
	if err != nil {
		return nil, err
	}
	return bi.GetCoin(pkh, txid, idx)
}

//GetCoin 获取一笔金额
func (bi *BlockIndex) GetCoin(pkh HASH160, txid HASH256, idx VarUInt) (*CoinKeyValue, error) {
	key := GetDBKey(CoinsPrefix, pkh[:], txid[:], idx.Bytes())
	coin := &CoinKeyValue{}
	val, err := bi.blkdb.Index().Get(key)
	if err != nil {
		coin, err = bi.txp.GetCoin(pkh, txid, idx)
	} else {
		err = coin.From(key, val)
	}
	return coin, err
}

//WriteGenesis 加载写入第一个区块
func (bi *BlockIndex) WriteGenesis() {
	dat, err := ioutil.ReadFile("genesis.blk")
	if err != nil {
		panic(fmt.Errorf("genesis.blk miss"))
	}
	buf := NewReader(dat)
	blk := &BlockInfo{}
	err = blk.Decode(buf)
	if err != nil {
		panic(err)
	}
	blk.Meta = EmptyTBEle(0, blk.Header, bi)
	err = blk.Write(bi)
	if err != nil {
		panic(err)
	}
	LogInfof("write genesis block %v success", blk)
}

//ListCoinsWithAccount 根据账号获取金额
func (bi *BlockIndex) ListCoinsWithAccount(acc *Account) (*CoinsState, error) {
	addr, err := acc.GetAddress()
	if err != nil {
		return nil, err
	}
	return bi.ListCoins(addr)
}

//ListCoins 获取某个地址账号的金额
func (bi *BlockIndex) ListCoins(addr Address) (*CoinsState, error) {
	pkh, err := addr.GetPkh()
	if err != nil {
		return nil, err
	}
	ds, err := bi.ListCoinsWithID(pkh)
	if err != nil {
		return nil, err
	}
	return ds.State(bi.NextHeight()), nil
}

//ListTxs 获取某个地址相关的交易
func (bi *BlockIndex) ListTxs(addr Address, limit ...int) (TxIndexs, error) {
	pkh, err := addr.GetPkh()
	if err != nil {
		return nil, err
	}
	return bi.ListTxsWithID(pkh, limit...)
}

//ListTxsWithID 获取交易
func (bi *BlockIndex) ListTxsWithID(id HASH160, limit ...int) (TxIndexs, error) {
	//和id相关的交易
	prefix := GetDBKey(TxpPrefix, id[:])
	idxs := TxIndexs{}
	//从交易池获取
	cvs, err := bi.txp.ListTxsWithID(bi, id, limit...)
	if err != nil {
		return nil, err
	}
	if idxs = append(idxs, cvs...); len(limit) > 0 {
		limit[0] -= len(idxs)
		if limit[0] <= 0 {
			return idxs, nil
		}
	}
	//获取区块链中可用的交易
	iter := bi.blkdb.Index().Iterator(NewPrefix(prefix))
	defer iter.Close()
	//根据区块高度倒序获取
	if iter.Last() {
		iv, err := NewTxIndex(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		idxs = append(idxs, iv)
		if len(limit) > 0 && len(idxs) >= limit[0] {
			return idxs, nil
		}
	}
	//倒序获取
	for iter.Prev() {
		iv, err := NewTxIndex(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		idxs = append(idxs, iv)
		if len(limit) > 0 && len(idxs) >= limit[0] {
			return idxs, nil
		}
	}
	return idxs, nil
}

//cb返回false,不再继续获取
func (bi *BlockIndex) ListCoinsWithCB(addr Address, cb func(ckv *CoinKeyValue) bool) error {
	tp := bi.GetTxPool()
	pkh, err := addr.GetPkh()
	if err != nil {
		return err
	}
	prefix := getDBKey(CoinsPrefix, pkh[:])
	iter := bi.blkdb.Index().Iterator(NewPrefix(prefix))
	defer iter.Close()
	for iter.Next() {
		ckv := &CoinKeyValue{}
		err := ckv.From(iter.Key(), iter.Value())
		if err != nil {
			return err
		}
		//如果在交易池消费了不显示
		if tp.IsSpentCoin(ckv) {
			continue
		}
		if !cb(ckv) {
			return nil
		}
	}
	return nil
}

//ListCoinsWithID 获取某个id的所有余额
//已经消费在内存中的不列出
func (bi *BlockIndex) ListCoinsWithID(pkh HASH160) (Coins, error) {
	tp := bi.GetTxPool()
	prefix := getDBKey(CoinsPrefix, pkh[:])
	kvs := Coins{}
	//获取区块链中历史可用金额
	iter := bi.blkdb.Index().Iterator(NewPrefix(prefix))
	defer iter.Close()
	for iter.Next() {
		ckv := &CoinKeyValue{}
		err := ckv.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		//如果在交易池消费了不显示
		if tp.IsSpentCoin(ckv) {
			continue
		}
		kvs = append(kvs, ckv)
	}
	//获取交易池中的用于id的金额
	cvs, err := tp.ListCoins(pkh)
	if err != nil {
		return nil, err
	}
	kvs = append(kvs, cvs...)
	return kvs, nil
}

//HasBlock 是否存在存在返回高度
func (bi *BlockIndex) HasBlock(id HASH256) (uint32, bool) {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	bh := InvalidHeight
	ele, has := bi.imap[id]
	if has {
		bh = ele.Value.(*TBEle).Height
	}
	return bh, has
}

//GetEle 获取区块头元素
func (bi *BlockIndex) GetEle(id HASH256) (*TBEle, error) {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.getEle(id)
}

func (bi *BlockIndex) getEle(id HASH256) (*TBEle, error) {
	eptr, has := bi.imap[id]
	if !has {
		return nil, errors.New("not found")
	}
	return eptr.Value.(*TBEle), nil
}

func (bi *BlockIndex) linkblk(blk *BlockInfo) error {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	//创建区块头
	meta := &TBMeta{
		BlockHeader: blk.Header,
	}
	bid, err := meta.ID()
	if err != nil {
		return err
	}
	nexth := InvalidHeight
	//是否能连接到主链后
	phv, pid, isok := bi.islinkback(meta)
	if !isok {
		return fmt.Errorf("can't link to chain last, hash=%v", bid)
	}
	//第一个必须是创世区块
	if pid.IsZero() && !conf.IsGenesisID(bid) {
		return errors.New("first blk must is genesis blk")
	}
	if pid.IsZero() {
		nexth = phv
	} else {
		nexth = phv + 1
	}
	//计算本高度下正确的难度
	bits := bi.calcBits(nexth)
	if bits != meta.Bits {
		return errors.New("block header bits error")
	}
	//检测id是否符合当前难度
	if !CheckProofOfWork(bid, meta.Bits) {
		return errors.New("block header bits check error")
	}
	ele := NewTBEle(meta, nexth, bi)
	blk.Meta = ele
	//设置交易数量
	blk.Meta.Txs = VarUInt(len(blk.Txs))
	return nil
}

//LinkBlk 更新区块数据(需要区块头先链接好
func (bi *BlockIndex) LinkBlk(blk *BlockInfo) error {
	err := bi.linkblk(blk)
	if err != nil {
		return err
	}
	//检测区块数据
	err = blk.Check(bi, true)
	if err != nil {
		return err
	}
	//执行交易脚本检测,返回错误不能打包
	err = blk.ExecScript(bi)
	if err != nil {
		return err
	}
	//写入数据库
	err = blk.Write(bi)
	if err != nil {
		return err
	}
	//连接必定不能出错
	bi.LinkBack(blk.Meta)
	//删除交易池中存在这个区块中的交易
	bi.txp.DelTxs(bi, blk.Txs)
	//事件通知
	bi.lptr.OnLinkBlock(blk)
	return nil
}

//GetTxPool 获取内存交易池
func (bi *BlockIndex) GetTxPool() *TxPool {
	return bi.txp
}

//Close 关闭链数据
func (bi *BlockIndex) Close() {
	bi.rwm.Lock()
	defer bi.rwm.Unlock()
	bi.lptr.OnClose()
	bi.blkdb.Close()
	bi.lis.Init()
	bi.txp.Close()
	bi.hmap = nil
	bi.imap = nil
}

//NewBlockIndex 创建区块链
func NewBlockIndex(lis IListener) *BlockIndex {
	blru, err := lru.New(256 * opt.MiB)
	if err != nil {
		panic(err)
	}
	return &BlockIndex{
		txp:   NewTxPool(),
		lptr:  lis,
		lis:   list.New(),
		hmap:  map[uint32]*list.Element{},
		imap:  map[HASH256]*list.Element{},
		blkdb: NewLevelDBStore(conf.DataDir + "/blks"),
		lru:   blru,
	}
}
