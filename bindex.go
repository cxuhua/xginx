package xginx

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/willf/bitset"

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
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
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
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
	it.cur = nil
	it.ele = it.bi.imap[id]
	return it.skipEle(skip...)
}

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

func (it *BIndexIter) First(skip ...int) bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
	it.cur = nil
	it.ele = it.bi.lis.Front()
	return it.skipEle(skip...)
}

func (it *BIndexIter) Last(skip ...int) bool {
	it.bi.rwm.RLock()
	defer it.bi.rwm.RUnlock()
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
	rwm  sync.RWMutex              //
	lis  *list.List                //区块头列表
	hmap map[uint32]*list.Element  //按高度缓存
	imap map[HASH256]*list.Element //按id缓存
	lru  *IndexCacher              //lru缓存
	db   IBlkStore                 //存储和索引
}

//返回某个交易的merkle验证树
func (bi *BlockIndex) NewMsgTxMerkle(id HASH256) (*MsgTxMerkle, error) {
	txv, err := bi.LoadTxValue(id)
	if err != nil {
		return nil, err
	}
	blk, err := bi.LoadBlock(txv.BlkId)
	if err != nil {
		return nil, err
	}
	ids := []HASH256{}
	bs := bitset.New(uint(len(blk.Txs)))
	for i, tx := range blk.Txs {
		tid, err := tx.ID()
		if err != nil {
			return nil, err
		}
		if id.Equal(tid) {
			bs.Set(uint(i))
		}
		ids = append(ids, tid)
	}
	tree := NewMerkleTree(len(blk.Txs))
	tree = tree.Build(ids, bs)
	if tree.IsBad() {
		return nil, errors.New("merkle tree bad")
	}
	msg := &MsgTxMerkle{}
	msg.TxId = id
	msg.Hashs = tree.Hashs()
	msg.Trans = VarInt(tree.Trans())
	msg.Bits = FromBitSet(tree.Bits())
	return msg, nil
}

func (bi *BlockIndex) CacheSize() int {
	return bi.lru.Size()
}

//获取当前监听器
func (bi *BlockIndex) GetListener() IListener {
	return bi.lptr
}

func (bi *BlockIndex) NewIter() *BIndexIter {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	iter := &BIndexIter{bi: bi}
	iter.ele = bi.lis.Back()
	return iter
}

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

//计算当前区块高度对应的难度
func (bi *BlockIndex) CalcBits(height uint32) uint32 {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.calcBits(height)
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
func (bi *BlockIndex) first() *TBEle {
	le := bi.lis.Front()
	if le == nil {
		return nil
	}
	return le.Value.(*TBEle)
}

//最低块
func (bi *BlockIndex) First() *TBEle {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.first()
}

func (bi *BlockIndex) BestHeight() uint32 {
	bv := bi.GetBestValue()
	if !bv.IsValid() {
		return InvalidHeight
	}
	bi.rwm.RLock()
	ele := bi.imap[bv.Id]
	bi.rwm.RUnlock()
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

func (bi *BlockIndex) lastHeight() uint32 {
	last := bi.last()
	if last == nil {
		return InvalidHeight
	}
	return last.Height
}

//最高块
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

//链长度
func (bi *BlockIndex) Len() int {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.lis.Len()
}

//获取块头
func (bi *BlockIndex) GetBlockHeader(id HASH256) (*TBEle, error) {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	ele, has := bi.imap[id]
	if !has {
		return nil, errors.New("not found")
	}
	return ele.Value.(*TBEle), nil
}

//获取区块的确认数
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
func (bi *BlockIndex) unlinkCount(id HASH256) (uint32, error) {
	if id.IsZero() {
		return bi.lastHeight() + 1, nil
	}
	ele, err := bi.getEle(id)
	if err != nil {
		return 0, errors.New("not found id")
	}
	return bi.lastHeight() - ele.Height, nil
}

func (bi *BlockIndex) UnlinkCount(id HASH256) (uint32, error) {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	return bi.unlinkCount(id)
}

//回退到指定id
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

type MulTransInfo struct {
	bi   *BlockIndex
	Src  []Address //原地址
	Keep int       //找零到这个索引对应的src地址
	Dst  []Address //目标地址
	Amts []Amount  //目标金额
	Fee  Amount    //交易费
	Ext  []byte    //扩展信息
}

func (m *MulTransInfo) Check() error {
	if len(m.Src) == 0 || len(m.Dst) == 0 || len(m.Dst) != len(m.Amts) {
		return errors.New("src dst amts num error")
	}
	if m.Keep < 0 || m.Keep >= len(m.Src) {
		return errors.New("keep index out bound")
	}
	if !m.Fee.IsRange() {
		return errors.New("fee value error")
	}
	sum := Amount(0)
	for _, v := range m.Amts {
		sum += v
	}
	if !sum.IsRange() {
		return errors.New("amts value error")
	}
	return nil
}

//获取地址对应的账户和金额列表
func (m *MulTransInfo) getAddressInfo(addr Address) (*Account, Coins, error) {
	spkh, err := addr.GetPkh()
	if err != nil {
		return nil, nil, err
	}
	acc, err := m.bi.lptr.GetWallet().GetAccountWithPkh(spkh)
	if err != nil {
		return nil, nil, err
	}
	ds, err := m.bi.ListCoinsWithID(spkh)
	if err != nil {
		return nil, nil, err
	}
	return acc, ds, nil
}

//生成交易
//pri=true 只使用有私钥的账户
func (m *MulTransInfo) NewTx(pri bool) (*TX, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	tx := NewTx()
	//输出总计
	sum := m.Fee
	for _, v := range m.Amts {
		sum += v
	}
	//计算使用哪些输入
	for _, src := range m.Src {
		//获取转出账号信息
		acc, ds, err := m.getAddressInfo(src)
		if err != nil {
			return nil, err
		}
		//是否只使用有私钥的账户
		if pri && !acc.HasPrivate() {
			continue
		}
		//获取需要消耗的输出
		for _, cv := range ds {
			//如果来自内存池，保存引用到的交易，之后检测时，引用到的交易必须存在当前区块中
			if cv.pool {
				tx.Refs = append(tx.Refs, cv.TxId)
			}
			in, err := cv.NewTxIn(acc)
			if err != nil {
				return nil, err
			}
			tx.Ins = append(tx.Ins, in)
			sum -= cv.Value
			if sum <= 0 {
				break
			}
		}
	}
	//没有减完，余额不足
	if sum > 0 {
		return nil, errors.New("Insufficient balance")
	}
	//转出到其他账号的输出
	for i, v := range m.Amts {
		addr := m.Dst[i]
		//创建目标输出
		out, err := addr.NewTxOut(v, m.Ext)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//多减的就是找零钱给自己
	if amt := -sum; amt > 0 {
		out, err := m.Src[m.Keep].NewTxOut(amt)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	if err := tx.Sign(m.bi); err != nil {
		return nil, err
	}
	//回调处理错误不放入交易池
	if err := m.bi.lptr.OnNewTx(m.bi, tx); err != nil {
		return nil, err
	}
	//放入交易池
	if err := m.bi.txp.PushBack(tx); err != nil {
		return nil, err
	}
	return tx, nil
}

func (bi *BlockIndex) EmptyMulTransInfo() *MulTransInfo {
	return &MulTransInfo{
		bi:   bi,
		Src:  []Address{},
		Keep: 0,
		Dst:  []Address{},
		Amts: []Amount{},
		Fee:  0,
	}
}

//获取区块头
func (bi *BlockIndex) loadtbele(id HASH256) (*TBEle, error) {
	ele := &TBEle{bi: bi}
	err := ele.LoadMeta(id)
	return ele, err
}

//向前加载一个区块数据头
func (bi *BlockIndex) loadPrev() (*TBEle, error) {
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
	//获取最后一个区块头
	last := bi.lis.Back()
	if last == nil {
		return 0, ZERO, true
	}
	ele := last.Value.(*TBEle)
	id, err := ele.ID()
	if err != nil {
		return 0, id, false
	}
	//时间戳检测
	if meta.Time <= ele.Time {
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

//加入一个队列尾并设置高度
func (bi *BlockIndex) linkback(ele *TBEle) error {
	last := bi.lis.Back()
	if last == nil {
		return bi.pushback(ele)
	}
	lv := last.Value.(*TBEle)
	id, err := lv.ID()
	if err != nil {
		return err
	}
	if ele.Time <= lv.Time {
		return errors.New("check linkback  time < prev time error")
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
	last := bi.lis.Back()
	if last == nil {
		return nil
	}
	last = last.Prev()
	if last != nil {
		ele := last.Value.(*TBEle)
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
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	bv := bi.GetBestValue()
	if !bv.IsValid() {
		return bi.first(), nil
	}
	ele := bi.imap[bv.Id]
	if ele != nil {
		ele = ele.Next()
	}
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

func (bi *BlockIndex) ListCoins(addr Address, limit ...Amount) (Coins, error) {
	pkh, err := addr.GetPkh()
	if err != nil {
		return nil, err
	}
	return bi.ListCoinsWithID(pkh, limit...)
}

//获取某个id的所有余额
func (bi *BlockIndex) ListCoinsWithID(id HASH160, limit ...Amount) (Coins, error) {
	prefix := getDBKey(COIN_PREFIX, id[:])
	kvs := Coins{}
	//获取区块链中历史可用金额
	iter := bi.db.Index().Iterator(NewPrefix(prefix))
	defer iter.Close()
	sum := Amount(0)
	for iter.Next() {
		tk := &CoinKeyValue{}
		err := tk.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		//如果已经在内存中被消费了，不列出
		if bi.txp.IsSpentCoin(tk) {
			continue
		}
		sum += tk.Value
		kvs = append(kvs, tk)
		if len(limit) > 0 && sum >= limit[0] {
			return kvs, nil
		}
	}
	if len(limit) > 0 {
		limit[0] -= sum
	}
	//获取交易池中的用于id的金额
	cvs, err := bi.txp.ListCoins(id, limit...)
	if err != nil {
		return nil, err
	}
	for _, tk := range cvs {
		kvs = append(kvs, tk)
	}
	return kvs, nil
}

//是否存在
func (bi *BlockIndex) HasBlock(id HASH256) bool {
	bi.rwm.RLock()
	defer bi.rwm.RUnlock()
	_, has := bi.imap[id]
	return has
}

func (bi *BlockIndex) getEle(id HASH256) (*TBEle, error) {
	eptr, has := bi.imap[id]
	if !has {
		return nil, errors.New("not found")
	}
	return eptr.Value.(*TBEle), nil
}

func (bi *BlockIndex) updateblk(buf IWriter, blk *BlockInfo) error {
	bi.rwm.Lock()
	defer bi.rwm.Unlock()
	bv := bi.GetBestValue()
	bid, err := blk.ID()
	if err != nil {
		return err
	}
	//如果是第一区块必须是genesis区块
	if !bv.IsValid() && !conf.IsGenesisId(bid) {
		return errors.New("genesis blk error")
	}
	//不是肯定能连接到上一个
	if bv.IsValid() && !blk.Header.Prev.Equal(bv.Id) {
		return errors.New("prev hash id error,can't update blk")
	}
	nexth := InvalidHeight
	if !bv.IsValid() {
		nexth = 0
	} else {
		nexth = bv.Height + 1
	}
	if bi.calcBits(nexth) != blk.Header.Bits {
		return errors.New("blk bits error")
	}
	if !CheckProofOfWork(bid, blk.Header.Bits) {
		return errors.New("check pow blk bits error")
	}
	//强制检测区块大小
	if err := blk.Encode(buf); err != nil {
		return err
	}
	if buf.Len() > MAX_BLOCK_SIZE {
		return errors.New("block size too big")
	}
	//获取区块头
	ele, err := bi.getEle(bid)
	if err != nil {
		return err
	}
	eid, err := ele.ID()
	if err != nil {
		return err
	}
	if ele.Height != nexth {
		return errors.New("blk height error")
	}
	if !bid.Equal(eid) {
		return errors.New("blk id != header id")
	}
	blk.Meta = ele
	//设置交易数量
	blk.Meta.Txs = VarUInt(len(blk.Txs))
	return nil
}

//更新区块数据(需要区块头先链接好
func (bi *BlockIndex) UpdateBlk(blk *BlockInfo) error {
	buf := NewWriter()
	err := bi.updateblk(buf, blk)
	if err != nil {
		return err
	}
	//检测区块数据
	err = blk.Check(bi, true)
	if err != nil {
		return err
	}
	if err := blk.CheckCoinbase(); err != nil {
		return err
	}
	bid, err := blk.ID()
	if err != nil {
		return err
	}
	//写入索引数据
	bt := bi.db.Index().NewBatch()
	rt := bt.NewRev()
	bt.Put(BestBlockKey, BestValueBytes(bid, blk.Meta.Height))
	if bv := bi.GetBestValue(); bv.IsValid() {
		rt.Put(BestBlockKey, bv.Bytes())
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
	bt.Put(BLOCK_PREFIX, bid[:], hbs)
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
	bi.lptr.OnUpdateBlock(bi, blk)
	return nil
}

//连接区块头
func (bi *BlockIndex) LinkHeader(header BlockHeader) (*TBEle, error) {
	bi.rwm.Lock()
	defer bi.rwm.Unlock()
	meta := &TBMeta{
		BlockHeader: header,
	}
	bid, err := meta.ID()
	if err != nil {
		return nil, err
	}
	nexth := InvalidHeight
	//是否能连接到主链后
	phv, pid, isok := bi.islinkback(meta)
	if !isok {
		return nil, fmt.Errorf("can't link to chain last, hash=%v", bid)
	}
	if pid.IsZero() && !conf.IsGenesisId(bid) {
		return nil, errors.New("first blk must is genesis blk")
	}
	if pid.IsZero() {
		nexth = phv
	} else {
		nexth = phv + 1
	}
	//计算本高度下正确的难度
	bits := bi.calcBits(nexth)
	if bits != meta.Bits {
		return nil, errors.New("block header bits error")
	}
	if !CheckProofOfWork(bid, meta.Bits) {
		return nil, errors.New("block header bits check error")
	}
	bt := bi.db.Index().NewBatch()
	//保存区块头数据
	hbs, err := meta.Bytes()
	if err != nil {
		return nil, err
	}
	//保存区块头
	bt.Put(BLOCK_PREFIX, bid[:], hbs)
	//保存最后一个头
	bh := BestValue{Height: nexth, Id: bid}
	bt.Put(LastHeaderKey, bh.Bytes())
	//保存数据
	err = bi.db.Index().Write(bt)
	if err != nil {
		return nil, err
	}
	ele := NewTBEle(meta, nexth, bi)
	ele, err = ele, bi.linkback(ele)
	if err != nil {
		return nil, err
	}
	bi.lptr.OnUpdateHeader(bi, ele)
	return ele, nil
}

//获取索引存储db
func (bi *BlockIndex) GetStoreDB() IBlkStore {
	return bi.db
}

//获取内存交易池
func (bi *BlockIndex) GetTxPool() *TxPool {
	return bi.txp
}

//关闭链数据
func (bi *BlockIndex) Close() {
	bi.rwm.Lock()
	defer bi.rwm.Unlock()
	LogInfo("block index closing")
	bi.lptr.OnClose(bi)
	bi.db.Close()
	bi.lis.Init()
	_ = bi.lru.Close()
	bi.txp.Close()
	bi.hmap = nil
	bi.imap = nil
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
		lru:  NewIndexCacher(256 * opt.MiB),
	}
	return bi
}
