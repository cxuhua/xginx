package xginx

import (
	"errors"
	"fmt"
)

const (
	//最大块大小
	MAX_BLOCK_SIZE = 1024 * 1024 * 4
	//最大日志大小
	MAX_LOG_SIZE = 1024 * 1024 * 2
	//最大扩展数据
	MAX_EXT_SIZE = 1024 * 4
	//锁定时间分界值
	LOCKTIME_THRESHOLD = uint32(500000000)
	//如果所有输入都是SEQUENCE_FINAL忽略locktime
	SEQUENCE_FINAL = uint32(0xffffffff)
	//是否禁止sequencel规则
	SEQUENCE_DISABLE_FLAG = uint32(1 << 31)
	//设置此位表示相对时间，时间粒度为512
	SEQUENCE_TYPE_FLAG = uint32(1 << 22)
	//相对时间mask
	SEQUENCE_MASK = uint32(0x0000ffff)
	//粒度 2^9=512
	SEQUENCE_GRANULARITY = 9
	//coinbase输出只能在当前区块高度100之后使用
	COINBASE_MATURITY = 100
)

//用户交易索引
type TxIndex struct {
	TxId  HASH256
	Value TxValue
}

func NewTxIndex(k []byte, v []byte) (*TxIndex, error) {
	iv := &TxIndex{}
	off := len(ZERO160) + len(TXP_PREFIX)
	copy(iv.TxId[:], k[off:off+len(ZERO256)])
	err := iv.Value.Decode(NewReader(v))
	return iv, err
}

//是否来自内存
func (ti TxIndex) IsPool() bool {
	return ti.Value.BlkId.IsZero()
}

type TxIndexs []*TxIndex

//存储交易索引值
type TxValue struct {
	BlkId HASH256 //块hash
	TxIdx VarUInt //txs 索引
}

func (v TxValue) GetTX(bi *BlockIndex) (*TX, error) {
	blk, err := bi.LoadBlock(v.BlkId)
	if err != nil {
		return nil, err
	}
	uidx := v.TxIdx.ToInt()
	if uidx < 0 || uidx >= len(blk.Txs) {
		return nil, errors.New("txsidx out of bound")
	}
	tx := blk.Txs[uidx]
	return tx, nil
}

func (v TxValue) Encode(w IWriter) error {
	if err := v.BlkId.Encode(w); err != nil {
		return err
	}
	if err := v.TxIdx.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *TxValue) Decode(r IReader) error {
	if err := v.BlkId.Decode(r); err != nil {
		return err
	}
	if err := v.TxIdx.Decode(r); err != nil {
		return err
	}
	return nil
}

func (v TxValue) Bytes() ([]byte, error) {
	buf := NewWriter()
	err := v.Encode(buf)
	return buf.Bytes(), err
}

//区块头数据
type HeaderBytes []byte

func (b HeaderBytes) Clone() HeaderBytes {
	v := make([]byte, len(b))
	copy(v, b)
	return v
}

func (b *HeaderBytes) SetNonce(v uint32) {
	l := len(*b)
	Endian.PutUint32((*b)[l-4:], v)
}

func (b *HeaderBytes) SetTime(v uint32) {
	l := len(*b)
	Endian.PutUint32((*b)[l-12:], v)
}

func (b *HeaderBytes) Hash() HASH256 {
	return Hash256From(*b)
}

func (b *HeaderBytes) Header() BlockHeader {
	buf := NewReader(*b)
	hptr := BlockHeader{}
	err := hptr.Decode(buf)
	if err != nil {
		panic(err)
	}
	return hptr
}

func getblockheadersize() int {
	buf := NewWriter()
	b := BlockHeader{}
	_ = b.Encode(buf)
	return buf.Len()
}

var (
	blockheadersize = getblockheadersize()
)

//区块头
type BlockHeader struct {
	Ver    uint32  //block ver
	Prev   HASH256 //pre block hash
	Merkle HASH256 //txs Merkle tree hash
	Time   uint32  //时间戳
	Bits   uint32  //难度
	Nonce  uint32  //随机值
	hasher HashCacher
}

func (v BlockHeader) Check() error {
	id, err := v.ID()
	if err != nil {
		return err
	}
	if v.Merkle.IsZero() {
		return errors.New("merkle id error")
	}
	if !CheckProofOfWork(id, v.Bits) {
		return errors.New("block header bits check error")
	}
	return nil
}

func (v BlockHeader) String() string {
	id, err := v.ID()
	if err != nil {
		panic(err)
	}
	return id.String()
}

func (v BlockHeader) Bytes() HeaderBytes {
	buf := NewWriter()
	err := v.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (v BlockHeader) IsGenesis() bool {
	id, err := v.ID()
	if err != nil {
		panic(err)
	}
	return v.Prev.IsZero() && conf.IsGenesisId(id)
}

func (v *BlockHeader) MustID() HASH256 {
	id, err := v.ID()
	if err != nil {
		panic(err)
	}
	return id
}

func (v *BlockHeader) ResetID() (HASH256, error) {
	v.hasher.Reset()
	return v.ID()
}

func (v *BlockHeader) ID() (HASH256, error) {
	if h, has := v.hasher.IsSet(); has {
		return h, nil
	}
	buf := NewWriter()
	err := v.Encode(buf)
	if err != nil {
		return ZERO256, err
	}
	return v.hasher.Hash(buf.Bytes()), nil
}

func (v *BlockHeader) Encode(w IWriter) error {
	if err := w.TWrite(v.Ver); err != nil {
		return err
	}
	if err := v.Prev.Encode(w); err != nil {
		return err
	}
	if err := v.Merkle.Encode(w); err != nil {
		return err
	}
	if err := w.TWrite(v.Time); err != nil {
		return err
	}
	if err := w.TWrite(v.Bits); err != nil {
		return err
	}
	if err := w.TWrite(v.Nonce); err != nil {
		return err
	}
	return nil
}

func (v *BlockHeader) Decode(r IReader) error {
	if err := r.TRead(&v.Ver); err != nil {
		return err
	}
	if err := v.Prev.Decode(r); err != nil {
		return err
	}
	if err := v.Merkle.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&v.Time); err != nil {
		return err
	}
	if err := r.TRead(&v.Bits); err != nil {
		return err
	}
	if err := r.TRead(&v.Nonce); err != nil {
		return err
	}
	return nil
}

type Headers []BlockHeader

func (hs Headers) Encode(w IWriter) error {
	if err := VarUInt(len(hs)).Encode(w); err != nil {
		return err
	}
	for _, v := range hs {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (hs *Headers) Decode(r IReader) error {
	num := VarUInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	vs := make([]BlockHeader, num)
	for i, _ := range vs {
		v := BlockHeader{}
		err := v.Decode(r)
		if err != nil {
			return err
		}
		vs[i] = v
	}
	*hs = vs
	return nil
}

func (hs *Headers) Add(h BlockHeader) {
	*hs = append(*hs, h)
}

func (hs *Headers) Reverse() {
	vs := Headers{}
	for i := len(*hs) - 1; i >= 0; i-- {
		vs.Add((*hs)[i])
	}
	*hs = vs
}

//检测区块头
//主要检测难度和merkle,id
func (hs Headers) check(h uint32, bh BlockHeader, bi *BlockIndex) error {
	if bh.Merkle.IsZero() {
		return errors.New("merkle id error")
	}
	if bh.Bits != bi.CalcBits(h) {
		return errors.New("height bits error")
	}
	//重新计算id，id必须符合当前难度
	if id, err := bh.ResetID(); err != nil {
		return err
	} else if !CheckProofOfWork(id, bh.Bits) {
		return errors.New("header bits error")
	}
	return nil
}

//检测区块头列表高度从height开始
func (hs Headers) Check(height uint32, bi *BlockIndex) error {
	if len(hs) == 0 {
		return errors.New("empty headers")
	}
	nexth := NextHeight(height)
	prev := hs[0]
	if err := hs.check(nexth, prev, bi); err != nil {
		return err
	}
	for i := 1; i < len(hs); i++ {
		curr := hs[i]
		if !curr.Prev.Equal(prev.MustID()) {
			return errors.New("prev hash != prev id")
		}
		if curr.Time <= prev.Time {
			return errors.New("time error")
		}
		if err := hs.check(nexth+uint32(i), curr, bi); err != nil {
			return err
		}
		prev = curr
	}
	return nil
}

//txs交易部分和比特币类似
//块大小限制为4M大小
type BlockInfo struct {
	Header BlockHeader //区块头
	Txs    []*TX       //交易记录，类似比特币
	Meta   *TBEle      //指向链节点
	merkel HashCacher  //merkel hash 缓存
}

func (blk *BlockInfo) Write(bi *BlockIndex) error {
	bid, err := blk.ID()
	if err != nil {
		return err
	}
	buf := NewWriter()
	if err := blk.Encode(buf); err != nil {
		return err
	}
	if buf.Len() > MAX_BLOCK_SIZE {
		return fmt.Errorf("block %v too big", bid)
	}
	//写入索引数据
	bt := bi.db.Index().NewBatch()
	rt := bt.NewRev()
	//写入最好区块数据信息
	bt.Put(BestBlockKey, BestValueBytes(bid, blk.Meta.Height))
	//还原写入
	if bv := bi.GetBestValue(); bv.IsValid() {
		rt.Put(BestBlockKey, bv.Bytes())
	}
	//写入交易信息
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
	return bi.db.Index().Write(bt)
}

func (blk BlockInfo) String() string {
	id, err := blk.ID()
	if err != nil {
		panic(err)
	}
	return id.String()
}

func (blk *BlockInfo) IsFinal(tx *TX) bool {
	return tx.IsFinal(blk.Meta.Height, blk.Meta.Time)
}

//创建Cosinbase 脚本
func (blk *BlockInfo) CoinbaseScript(ip []byte, bs ...[]byte) Script {
	return NewCoinbaseScript(blk.Meta.Height, ip, bs...)
}

//获取区块奖励
func (blk *BlockInfo) CoinbaseReward() Amount {
	return GetCoinbaseReward(blk.Meta.Height)
}

//检测coinbas
func (blk *BlockInfo) CheckCoinbase() error {
	if blk.Meta == nil {
		return errors.New("not set block meta,can't check coinbase")
	}
	if len(blk.Txs) < 1 {
		return errors.New("txs count == 0,coinbase miss")
	}
	if len(blk.Txs[0].Ins) != 1 {
		return errors.New("ins count == 0,coinbase miss")
	}
	script := blk.Txs[0].Ins[0].Script
	if !script.IsCoinBase() {
		return errors.New("ins script type error,coinbase miss")
	}
	if script.Height() != blk.Meta.Height {
		return errors.New("coinbase height != meta height")
	}
	return nil
}

//写入交易索引
func (blk *BlockInfo) WriteTxsIdx(bi *BlockIndex, bt *Batch) error {
	bid, err := blk.ID()
	if err != nil {
		return err
	}
	//交易所在的区块信息和金额信息索引
	for idx, tx := range blk.Txs {
		id, err := tx.ID()
		if err != nil {
			return err
		}
		vval := TxValue{
			TxIdx: VarUInt(idx),
			BlkId: bid,
		}
		vbys, err := vval.Bytes()
		if err != nil {
			return err
		}
		//交易对应的区块和位置
		bt.Put(TXS_PREFIX, id[:], vbys)
		vps := map[HASH160]bool{}
		//写入金额和索引
		err = tx.writeTxIndex(bi, blk, vps, bt)
		if err != nil {
			return err
		}
		//写入账户相关的交易
		for pkh, _ := range vps {
			bt.Put(TXP_PREFIX, pkh[:], id[:], vbys)
		}
	}
	return nil
}

func (blk *BlockInfo) GetMerkle() (HASH256, error) {
	if h, b := blk.merkel.IsSet(); b {
		return h, nil
	}
	ids := []HASH256{}
	for _, tv := range blk.Txs {
		if vid, err := tv.ID(); err != nil {
			return ZERO256, err
		} else {
			ids = append(ids, vid)
		}
	}
	root, err := BuildMerkleTree(ids).ExtractRoot()
	if err != nil {
		return root, err
	}
	blk.merkel.SetHash(root)
	return root, nil
}

func (blk *BlockInfo) SetMerkle() error {
	merkle, err := blk.GetMerkle()
	if err != nil {
		return err
	}
	blk.Header.Merkle = merkle
	return nil
}

//检查引用的tx是否存在区块中
func (blk *BlockInfo) CheckRefsTx(bi *BlockIndex, tx *TX) error {
	for _, in := range tx.Ins {
		//不检测coinbase交易
		if in.IsCoinBase() {
			continue
		}
		//获取引用的输出
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return fmt.Errorf("out tx miss %w", err)
		}
		//如果是来自交易池，交易必须存在区块中
		if out.pool && !blk.HasTx(in.OutHash) {
			return errors.New("refs tx pool miss")
		}
	}
	return nil
}

//添加多个交易
//有重复消费输出将会失败
func (blk *BlockInfo) AddTxs(bi *BlockIndex, txs []*TX) error {
	if len(txs) == 0 {
		return errors.New("txs empty")
	}
	//保存旧的交易列表
	otxs := blk.Txs
	//加入多个交易到区块中
	for _, tx := range txs {
		id, err := tx.ID()
		if err != nil {
			return err
		}
		//如果已经存在忽略
		if blk.HasTx(id) {
			continue
		}
		if !blk.IsFinal(tx) {
			continue
		}
		if err := blk.CheckRefsTx(bi, tx); err != nil {
			return err
		}
		if err := tx.Check(bi, true); err != nil {
			return err
		}
		blk.Txs = append(blk.Txs, tx)
	}
	//不允许重复消费同一个输出
	if err := blk.CheckRepCostTxOut(bi); err != nil {
		blk.Txs = otxs
		return err
	}
	return nil
}

//查找区块内的单个交易是否存在
func (blk *BlockInfo) HasTx(id HASH256) bool {
	for _, tx := range blk.Txs {
		tid, err := tx.ID()
		if err != nil {
			panic(err)
		}
		if tid.Equal(id) {
			return true
		}
	}
	return false
}

//从交易池加载可用的交易
func (blk *BlockInfo) LoadTxs(bi *BlockIndex) error {
	txs, err := bi.txp.GetTxs(bi, blk)
	if err != nil {
		return err
	}
	if len(txs) == 0 {
		return nil
	}
	return blk.AddTxs(bi, txs)
}

//添加单个交易
//有重复消费输出将会失败
func (blk *BlockInfo) AddTx(bi *BlockIndex, tx *TX) error {
	id, err := tx.ID()
	if err != nil {
		return err
	}
	//不能重复添加
	if blk.HasTx(id) {
		return nil
	}
	if !blk.IsFinal(tx) {
		return errors.New("not final tx")
	}
	//检测引用的交易是否存在
	if err := blk.CheckRefsTx(bi, tx); err != nil {
		return err
	}
	//保存旧的交易列表
	otxs := blk.Txs
	//检测交易是否可进行
	if err := tx.Check(bi, true); err != nil {
		return err
	}
	blk.Txs = append(blk.Txs, tx)
	//不允许重复消费同一个输出
	if err := blk.CheckRepCostTxOut(bi); err != nil {
		blk.Txs = otxs
		return err
	}
	return nil
}

//获取区块id
func (blk *BlockInfo) MustID() HASH256 {
	return blk.Header.MustID()
}

//获取区块id
func (blk *BlockInfo) ID() (HASH256, error) {
	return blk.Header.ID()
}

func (blk *BlockInfo) IsGenesis() bool {
	return blk.Header.IsGenesis()
}

//获取coinse out fee sum
func (blk *BlockInfo) CoinbaseFee() (Amount, error) {
	if len(blk.Txs) == 0 {
		return 0, errors.New("miss txs")
	}
	return blk.Txs[0].CoinbaseFee()
}

//获取总的交易费
func (blk *BlockInfo) GetFee(bi *BlockIndex) (Amount, error) {
	fee := Amount(0)
	for _, tx := range blk.Txs {
		if tx.IsCoinBase() {
			continue
		}
		f, err := tx.GetTransFee(bi)
		if err != nil {
			return fee, err
		}
		fee += f
	}
	if !fee.IsRange() {
		return 0, errors.New("amount range error")
	}
	return fee, nil
}

//获取区块收益
func (blk *BlockInfo) GetIncome(bi *BlockIndex) (Amount, error) {
	rfee := GetCoinbaseReward(blk.Meta.Height)
	fee, err := blk.GetFee(bi)
	if err != nil {
		return 0, err
	}
	return fee + rfee, nil
}

//检查所有的交易
func (blk *BlockInfo) CheckTxs(bi *BlockIndex, csp bool) error {
	//必须有交易
	if len(blk.Txs) == 0 {
		return errors.New("txs miss, too little")
	}
	//获取区块奖励
	rfee := GetCoinbaseReward(blk.Meta.Height)
	if !rfee.IsRange() {
		return errors.New("coinbase reward amount error")
	}
	//检测所有交易
	for i, tx := range blk.Txs {
		if i == 0 && !tx.IsCoinBase() {
			return errors.New("coinbase tx miss")
		}
		err := tx.Check(bi, csp)
		if err != nil {
			return err
		}
		//存入缓存
		err = bi.SetTx(tx)
		if err != nil {
			return err
		}
	}
	//获取交易费
	tfee, err := blk.GetFee(bi)
	if err != nil {
		return err
	}
	//coinbase输出
	cfee, err := blk.CoinbaseFee()
	if err != nil {
		return err
	}
	//奖励+交易费之和不能大于coinbase输出
	sfee := rfee + tfee
	if !sfee.IsRange() {
		return errors.New("sum fee fee error")
	}
	if cfee > sfee {
		return errors.New("coinbase fee error")
	}
	return nil
}

//重置所有hash缓存
func (blk *BlockInfo) ResetHasher() {
	//重置hash缓存用来计算merkle
	for _, tx := range blk.Txs {
		tx.ResetAll()
	}
	blk.merkel.Reset()
	blk.Header.hasher.Reset()
}

//完成块数据
func (blk *BlockInfo) Finish(bi *BlockIndex) error {
	lptr := bi.GetListener()
	if lptr == nil {
		return errors.New("listener null")
	}
	if len(blk.Txs) == 0 {
		return errors.New("txs miss, too little")
	}
	//最后设置merkleid
	if err := lptr.OnFinished(blk); err != nil {
		return err
	}
	//重置缓存设置merkle
	blk.ResetHasher()
	if err := blk.SetMerkle(); err != nil {
		return err
	}
	return blk.Check(bi, true)
}

//检查是否有多个输入消费同一个输出
func (blk *BlockInfo) CheckRepCostTxOut(bi *BlockIndex) error {
	imap := map[HASH256]bool{}
	for _, tx := range blk.Txs {
		for _, in := range tx.Ins {
			key := in.OutKey()
			if _, has := imap[key]; has {
				return fmt.Errorf("%v %d repeat cost", in.OutHash, in.OutIndex)
			}
			imap[key] = true
		}
	}
	imap = nil
	return nil
}

//验证区块数据
func (blk *BlockInfo) Verify(ele *TBEle, bi *BlockIndex) error {
	id, err := blk.ID()
	if err != nil {
		return err
	}
	eid, err := ele.ID()
	if err != nil {
		return err
	}
	if !id.Equal(eid) {
		return errors.New("block id != header id")
	}
	if !CheckProofOfWork(id, blk.Meta.Bits) {
		return errors.New("check pow error")
	}
	return blk.Check(bi, false)
}

//检查区块数据
func (blk *BlockInfo) Check(bi *BlockIndex, csp bool) error {
	//检测工作难度
	bits := bi.CalcBits(blk.Meta.Height)
	if bits != blk.Header.Bits {
		return errors.New("block header bits error")
	}
	//检查merkle树
	merkle, err := blk.GetMerkle()
	if err != nil {
		return err
	}
	if !merkle.Equal(blk.Header.Merkle) {
		return errors.New("txs merkle hash error")
	}
	//检查重复消费
	if err := blk.CheckRepCostTxOut(bi); err != nil {
		return err
	}
	//检查所有的交易
	return blk.CheckTxs(bi, csp)
}

func (blk *BlockInfo) Encode(w IWriter) error {
	if err := blk.Header.Encode(w); err != nil {
		return err
	}
	if err := VarUInt(len(blk.Txs)).Encode(w); err != nil {
		return err
	}
	for _, v := range blk.Txs {
		if err := v.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

func (blk *BlockInfo) Decode(r IReader) error {
	if err := blk.Header.Decode(r); err != nil {
		return err
	}
	txn := VarUInt(0)
	if err := txn.Decode(r); err != nil {
		return err
	}
	blk.Txs = make([]*TX, txn)
	for i, _ := range blk.Txs {
		tx := &TX{}
		if err := tx.Decode(r); err != nil {
			return err
		}
		blk.Txs[i] = tx
	}
	return nil
}

//交易输入
type TxIn struct {
	OutHash  HASH256 //输出交易hash
	OutIndex VarUInt //对应的输出索引
	Script   Script  //签名后填充脚本
	Sequence uint32  //阻止后续交易入块，当前序交易达到某个块高度，或者到达某个相对时间，当前交易才能进区块
}

func NewTxIn() *TxIn {
	return &TxIn{
		Sequence: SEQUENCE_FINAL,
	}
}

//如果比较seq可替换
func (in *TxIn) IsReplace(sin *TxIn) bool {
	return in.Sequence&SEQUENCE_DISABLE_FLAG != 0 && sin.Sequence&SEQUENCE_DISABLE_FLAG != 0 && in.Sequence > sin.Sequence
}

//设置按时间的seq
//tv是秒数
func (in *TxIn) SetSeqTime(seconds uint32) {
	seconds = seconds >> SEQUENCE_GRANULARITY
	seconds = seconds & SEQUENCE_MASK
	in.Sequence = seconds | SEQUENCE_TYPE_FLAG
}

//禁用seq
func (in *TxIn) DisableSeq() {
	in.Sequence |= SEQUENCE_DISABLE_FLAG
}

//按高度锁定
func (in *TxIn) SetSeqHeight(height uint32) {
	in.Sequence = height & SEQUENCE_MASK
}

func (in TxIn) Equal(v *TxIn) bool {
	return in.OutIndex == v.OutIndex && in.OutHash.Equal(v.OutHash)
}

//获取引用的coin
func (in TxIn) GetCoin(bi *BlockIndex) (*CoinKeyValue, error) {
	//输入引用的输出
	out, err := in.LoadTxOut(bi)
	if err != nil {
		return nil, err
	}
	//获取输出对应的金额
	pkh, err := out.Script.GetPkh()
	if err != nil {
		return nil, err
	}
	return bi.GetCoin(pkh, in.OutHash, in.OutIndex)
}

//消费key,用来记录输入对应的输出是否已经别消费
func (in TxIn) SpentKey() []byte {
	buf := NewWriter()
	err := buf.WriteFull(COINS_PREFIX)
	if err != nil {
		panic(err)
	}
	err = in.OutHash.Encode(buf)
	if err != nil {
		panic(err)
	}
	err = in.OutIndex.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

//获取输入引用key
func (in TxIn) OutKey() HASH256 {
	buf := NewWriter()
	err := in.OutHash.Encode(buf)
	if err != nil {
		panic(err)
	}
	err = in.OutIndex.Encode(buf)
	if err != nil {
		panic(err)
	}
	return Hash256From(buf.Bytes())
}

//获取输入引用的输出
func (in *TxIn) LoadTxOut(bi *BlockIndex) (*TxOut, error) {
	if in.OutHash.IsZero() {
		return nil, errors.New("zero hash id")
	}
	tp := bi.GetTxPool()
	otx, err := bi.LoadTX(in.OutHash)
	if err != nil {
		//如果在交易池中
		otx, err = tp.Get(in.OutHash)
	}
	if err != nil {
		return nil, fmt.Errorf("txin outtx miss %w", err)
	}
	oidx := in.OutIndex.ToInt()
	if oidx < 0 || oidx >= len(otx.Outs) {
		return nil, fmt.Errorf("outindex out of bound")
	}
	out := otx.Outs[oidx]
	out.pool = otx.pool
	return out, nil
}

func (in *TxIn) Check(bi *BlockIndex) error {
	if in.IsCoinBase() {
		return nil
	} else if in.Script.IsWitness() {
		return nil
	} else {
		return errors.New("txin unlock script type error")
	}
}

//计算交易id用到的数据
func (in *TxIn) ForID(w IWriter) error {
	if err := in.OutHash.Encode(w); err != nil {
		return err
	}
	if err := in.OutIndex.Encode(w); err != nil {
		return err
	}
	if err := in.Script.ForID(w); err != nil {
		return err
	}
	if err := w.TWrite(in.Sequence); err != nil {
		return err
	}
	return nil
}

func (in *TxIn) Encode(w IWriter) error {
	if err := in.OutHash.Encode(w); err != nil {
		return err
	}
	if err := in.OutIndex.Encode(w); err != nil {
		return err
	}
	if err := in.Script.Encode(w); err != nil {
		return err
	}
	if err := w.TWrite(in.Sequence); err != nil {
		return err
	}
	return nil
}

func (in *TxIn) Decode(r IReader) error {
	if err := in.OutHash.Decode(r); err != nil {
		return err
	}
	if err := in.OutIndex.Decode(r); err != nil {
		return err
	}
	if err := in.Script.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&in.Sequence); err != nil {
		return err
	}
	return nil
}

//txs的第一个一定是coinbase类型
func (in *TxIn) IsCoinBase() bool {
	return in.OutHash.IsZero() && in.OutIndex == 0 && in.Script.IsCoinBase()
}

//交易输出
type TxOut struct {
	Value  Amount //距离奖励 GetRewardRate 计算比例，所有输出之和不能高于总奖励
	Script Script //锁定脚本
	pool   bool   //是否来自交易池中的交易
}

//获取输入引用的输出和金额
func (out *TxOut) GetCoin(in *TxIn, bi *BlockIndex) (*CoinKeyValue, error) {
	pkh, err := out.Script.GetPkh()
	if err != nil {
		return nil, err
	}
	coin, err := bi.GetCoin(pkh, in.OutHash, in.OutIndex)
	if err != nil {
		return nil, err
	}
	if !coin.IsMatured(bi.NextHeight()) {
		return nil, errors.New("coin not matured")
	}
	return coin, err
}

//in引用的coin状态是否正常
func (out *TxOut) HasCoin(in *TxIn, bi *BlockIndex) bool {
	pkh, err := out.Script.GetPkh()
	if err != nil {
		panic(fmt.Errorf("get pkh error %w", err))
	}
	coin, err := bi.GetCoin(pkh, in.OutHash, in.OutIndex)
	if err != nil {
		return false
	}
	//coin没成熟
	if !coin.IsMatured(bi.NextHeight()) {
		return false
	}
	return true
}

func (out *TxOut) Check(bi *BlockIndex) error {
	if !out.Value.IsRange() {
		return errors.New("txout value error")
	}
	if out.Script.IsLocked() {
		return nil
	}
	return errors.New("unknow script type")
}

func (out *TxOut) Encode(w IWriter) error {
	if err := out.Value.Encode(w); err != nil {
		return err
	}
	if err := out.Script.Encode(w); err != nil {
		return err
	}
	return nil
}

func (out *TxOut) Decode(r IReader) error {
	if err := out.Value.Decode(r); err != nil {
		return err
	}
	if err := out.Script.Decode(r); err != nil {
		return err
	}
	return nil
}

//交易
type TX struct {
	Ver      VarUInt    //版本
	Ins      []*TxIn    //输入
	Outs     []*TxOut   //输出
	LockTime uint32     //锁定时间或者区块
	idcs     HashCacher //hash缓存
	outs     HashCacher //签名hash缓存
	pres     HashCacher //签名hash缓存
	pool     bool       //是否来自内存池
}

func NewTx() *TX {
	tx := &TX{}
	tx.Ver = 1
	tx.Outs = []*TxOut{}
	tx.Ins = []*TxIn{}
	tx.LockTime = 0
	return tx
}

//检测输入是否被锁定
//返回true表示被锁住，无法进入区块
func (tx *TX) CheckSeqLocks(bi *BlockIndex) (bool, error) {
	height := bi.Height()
	//如果链空忽略seq检测
	if height == 0 || height == InvalidHeight {
		return false, nil
	}
	minh, mint := int64(-1), int64(-1)
	for _, in := range tx.Ins {
		if in.Sequence&SEQUENCE_DISABLE_FLAG != 0 {
			continue
		}
		//获取当前引用的块高
		ch := uint32(0)
		coin, err := in.GetCoin(bi)
		if err != nil {
			return false, err
		}
		if coin.pool {
			ch = height + 1
		} else {
			ch = coin.Height.ToUInt32()
		}
		//如果是按时间锁定
		if in.Sequence&SEQUENCE_TYPE_FLAG != 0 {
			ctime := bi.GetMedianTime(ch - 1) //计算中间时间
			stime := int64(ctime) + int64(in.Sequence&SEQUENCE_MASK)<<SEQUENCE_GRANULARITY - 1
			if stime > mint {
				mint = stime
			}
		} else {
			//按高度锁定
			sheight := int64(ch) + int64(in.Sequence&SEQUENCE_MASK) - 1
			if sheight > minh {
				minh = sheight
			}
		}
	}
	if minh > 0 && minh >= int64(height) {
		return true, nil
	}
	blktime := bi.GetMedianTime(height)
	if mint > 0 && mint >= int64(blktime) {
		return true, nil
	}
	return false, nil
}

//查找消费金额的输入
func (tx *TX) FindTxIn(hash HASH256, idx VarUInt) *TxIn {
	for _, in := range tx.Ins {
		if in.OutHash.Equal(hash) && in.OutIndex == idx {
			return in
		}
	}
	return nil
}

//当locktime ！=0 时，如果所有输入 Sequence==SEQUENCE_FINAL 交易及时生效
//否则要达到自定高度或者时间交易才能生效
//未生效前，输入可以被替换，也就是输入对应的输出可以被消费，这时原先的交易将被移除
func (tx *TX) IsFinal(hv uint32, tv uint32) bool {
	if tx.LockTime == 0 {
		return true
	}
	lt := uint32(0)
	if tx.LockTime < LOCKTIME_THRESHOLD {
		lt = hv
	} else {
		lt = tv
	}
	if tx.LockTime < lt {
		return true
	}
	for _, v := range tx.Ins {
		if v.Sequence != SEQUENCE_FINAL {
			return false
		}
	}
	return true
}

//重置缓存
func (tx *TX) ResetAll() {
	tx.idcs.Reset()
	tx.outs.Reset()
	tx.pres.Reset()
}

func (tx TX) String() string {
	id, err := tx.ID()
	if err != nil {
		panic(err)
	}
	return id.String()
}

//第一个必须是base交易
func (tx *TX) IsCoinBase() bool {
	return len(tx.Ins) == 1 && tx.Ins[0].IsCoinBase()
}

//写入交易信息索引和回退索引
func (tx *TX) writeTxIndex(bi *BlockIndex, blk *BlockInfo, vps map[HASH160]bool, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return errors.New("batch miss rev")
	}
	base := tx.IsCoinBase()
	//输入coin
	for _, in := range tx.Ins {
		if in.IsCoinBase() {
			continue
		}
		//out将被消耗掉
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		pkh := out.Script.MustPkh()
		vps[pkh] = true //交易相关的pkh
		//引用的金额
		coin, err := bi.GetCoin(pkh, in.OutHash, in.OutIndex)
		if err != nil {
			return err
		}
		if !coin.IsMatured(blk.Meta.Height) {
			return errors.New("ref out coin not matured")
		}
		//被消费删除
		bt.Del(coin.MustKey())
		//添加回退日志用来恢复,如果是引用本区块的忽略
		if !coin.pool {
			rt.Put(coin.MustKey(), coin.MustValue())
		}
	}
	//输出coin
	for idx, out := range tx.Outs {
		pkh := out.Script.MustPkh()
		tk := &CoinKeyValue{}
		tk.Value = out.Value
		tk.CPkh = pkh
		tk.Index = VarUInt(idx)
		tk.TxId = tx.MustID()
		if base {
			tk.Base = 1
		} else {
			tk.Base = 0
		}
		tk.Height = VarUInt(blk.Meta.Height)
		bt.Put(tk.MustKey(), tk.MustValue())
		vps[pkh] = true //交易相关的pkh
	}
	return nil
}

//验证交易输入数据
func (tx *TX) Verify(bi *BlockIndex) error {
	for idx, in := range tx.Ins {
		//不验证base的签名
		if in.IsCoinBase() {
			continue
		}
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		err = NewSigner(tx, out, in).Verify()
		if err != nil {
			return fmt.Errorf("Verify in %d error %w", idx, err)
		}
	}
	return nil
}

//获取某个输入的签名器
func (tx *TX) GetSigner(bi *BlockIndex, idx int) (ISigner, error) {
	if idx < 0 || idx >= len(tx.Ins) {
		return nil, errors.New("tx index out bound")
	}
	in := tx.Ins[idx]
	if in.IsCoinBase() {
		return nil, errors.New("conbase no signer")
	}
	out, err := in.LoadTxOut(bi)
	if err != nil {
		return nil, err
	}
	//检查是否已经被消费
	if !out.HasCoin(in, bi) {
		return nil, errors.New("coin miss")
	}
	return NewSigner(tx, out, in), nil
}

//签名交易数据
//cspent 是否检测输出金额是否存在
func (tx *TX) Sign(bi *BlockIndex) error {
	lptr := bi.GetListener()
	if lptr == nil {
		return errors.New("block index listener null,can't sign")
	}
	for idx, in := range tx.Ins {
		if in.IsCoinBase() {
			continue
		}
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		if !out.HasCoin(in, bi) {
			return errors.New("coin miss")
		}
		pkh, err := out.Script.GetPkh()
		if err != nil {
			return err
		}
		acc, err := bi.lptr.GetWallet().GetAccountWithPkh(pkh)
		if err != nil {
			return err
		}
		//对每个输入签名
		err = NewSigner(tx, out, in).Sign(bi, acc)
		if err != nil {
			return fmt.Errorf("sign in %d error %w", idx, err)
		}
	}
	return nil
}

func (tx *TX) MustID() HASH256 {
	id, err := tx.ID()
	if err != nil {
		panic(err)
	}
	return id
}

//交易id计算,不包括见证数据
func (tx *TX) ID() (HASH256, error) {
	if hash, ok := tx.idcs.IsSet(); ok {
		return hash, nil
	}
	id := HASH256{}
	buf := NewWriter()
	//版本
	if err := tx.Ver.Encode(buf); err != nil {
		return id, err
	}
	//输入数量
	err := VarUInt(len(tx.Ins)).Encode(buf)
	if err != nil {
		return id, err
	}
	//输入
	for _, in := range tx.Ins {
		err := in.ForID(buf)
		if err != nil {
			return id, err
		}
	}
	//输出数量
	err = VarUInt(len(tx.Outs)).Encode(buf)
	if err != nil {
		return id, err
	}
	//输出
	for _, out := range tx.Outs {
		err := out.Encode(buf)
		if err != nil {
			return id, err
		}
	}
	//锁定时间
	if err := buf.TWrite(tx.LockTime); err != nil {
		return id, err
	}
	return tx.idcs.Hash(buf.Bytes()), nil
}

//获取coinse out fee sum
func (tx *TX) CoinbaseFee() (Amount, error) {
	if !tx.IsCoinBase() {
		return 0, errors.New("tx not coinbase")
	}
	fee := Amount(0)
	for _, out := range tx.Outs {
		fee += out.Value
	}
	if !fee.IsRange() {
		return 0, errors.New("amount range error")
	}
	return fee, nil
}

//获取此交易交易费
func (tx *TX) GetTransFee(bi *BlockIndex) (Amount, error) {
	if tx.IsCoinBase() {
		return 0, errors.New("coinbase not trans fee")
	}
	fee := Amount(0)
	for _, in := range tx.Ins {
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return 0, err
		}
		fee += out.Value
	}
	for _, out := range tx.Outs {
		fee -= out.Value
	}
	return fee, nil
}

//检测交易中是否有重复的输入
func (tx *TX) HasRepTxIn(bi *BlockIndex, csp bool) bool {
	mps := map[HASH256]bool{}
	for _, iv := range tx.Ins {
		key := iv.OutKey()
		if _, has := mps[key]; has {
			return true
		}
		mps[key] = true
	}
	return false
}

//检测除coinbase交易外的交易金额
//csp是否检测输出金额是否已经被消费,如果交易已经打包进区块，输入引用的输出肯定被消费,coin将不存在
func (tx *TX) Check(bi *BlockIndex, csp bool) error {
	//至少有一个交易
	if len(tx.Ins) == 0 {
		return errors.New("tx ins too slow")
	}
	//检测输入是否重复引用了相同的输出
	if tx.HasRepTxIn(bi, csp) {
		return errors.New("txin repeat cost txout")
	}
	//这里不检测coinbase交易
	if tx.IsCoinBase() {
		return nil
	}
	//锁定的交易不能进入 csp=true时才会检查，如果交易已经在区块中不需要检查
	if csp {
		lck, err := tx.CheckSeqLocks(bi)
		if err != nil || lck {
			return fmt.Errorf("tx seq locked %v %w", tx, err)
		}
	}
	//总的输入金额
	itv := Amount(0)
	for _, in := range tx.Ins {
		err := in.Check(bi)
		if err != nil {
			return err
		}
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		//是否校验金额是否存在
		if csp && !out.HasCoin(in, bi) {
			return errors.New("coin miss")
		}
		itv += out.Value
	}
	//总的输出金额
	otv := Amount(0)
	for _, out := range tx.Outs {
		err := out.Check(bi)
		if err != nil {
			return err
		}
		otv += out.Value
	}
	//金额必须在合理的范围
	if !itv.IsRange() || !otv.IsRange() {
		return errors.New("in or out amount error")
	}
	//每个交易的输出不能大于输入,差值会输出到coinbase交易当作交易费
	if itv < 0 || otv < 0 || otv > itv {
		return errors.New("ins amount must >= outs amount")
	}
	//检查签名
	return tx.Verify(bi)
}

func (tx *TX) Encode(w IWriter) error {
	if err := tx.Ver.Encode(w); err != nil {
		return err
	}
	if err := VarUInt(len(tx.Ins)).Encode(w); err != nil {
		return err
	}
	for _, v := range tx.Ins {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	if err := VarUInt(len(tx.Outs)).Encode(w); err != nil {
		return err
	}
	for _, v := range tx.Outs {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	return w.TWrite(tx.LockTime)
}

func (tx *TX) Decode(r IReader) error {
	if err := tx.Ver.Decode(r); err != nil {
		return err
	}
	inum := VarUInt(0)
	if err := inum.Decode(r); err != nil {
		return err
	}
	tx.Ins = make([]*TxIn, inum)
	for i, _ := range tx.Ins {
		in := NewTxIn()
		err := in.Decode(r)
		if err != nil {
			return err
		}
		tx.Ins[i] = in
	}
	onum := VarUInt(0)
	if err := onum.Decode(r); err != nil {
		return err
	}
	tx.Outs = make([]*TxOut, onum)
	for i, _ := range tx.Outs {
		out := &TxOut{}
		err := out.Decode(r)
		if err != nil {
			return err
		}
		tx.Outs[i] = out
	}
	return r.TRead(&tx.LockTime)
}
