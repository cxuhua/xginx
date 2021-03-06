package xginx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

//常量定义
const (
	//最大块大小
	MaxBlockSize = 1024 * 1024 * 8
	//最大日志大小
	MaxLogSize = 1024 * 1024 * 4
	//最大执行脚本长度
	MaxExecSize = 1024 * 2
	//最大meta数据长度
	MaxMetaSize = 1024 * 4
	//默认: 50步,1=30毫秒
	DefaultExeLimit = (30 << 16) | 30000
	//高16位限制执行步数,低16位限制执行时间,单位:微秒
	MaxExeTimeLimit = uint32(0xFFFF) //65毫秒
	MaxExeStepLimit = uint32(30000)  //6万step
	//coinbase需要100区块后可用
	CoinbaseMaturity = 100
	//如果所有的输入全是 >= FinalSequence，交易立即生效
	FinalSequence = VarUInt(0xFFFFFF)
)

//TxIndex 用户交易索引
type TxIndex struct {
	TxID   HASH256
	Height uint32
	Value  TxValue
	pool   bool
}

//NewTxIndex 从kv数据创建交易索引
func NewTxIndex(k []byte, v []byte) (*TxIndex, error) {
	iv := &TxIndex{}
	off := len(ZERO160) + len(TxpPrefix)
	iv.Height = binary.BigEndian.Uint32(k[off : off+4])
	off += 4
	copy(iv.TxID[:], k[off:off+len(ZERO256)])
	err := iv.Value.Decode(NewReader(v))
	return iv, err
}

//IsPool 是否来自内存
func (ti TxIndex) IsPool() bool {
	return ti.pool
}

//TxIndexs 交易索引集合
type TxIndexs []*TxIndex

//TxValue 存储交易索引值
type TxValue struct {
	//块hash
	BlkID HASH256
	//txs 索引
	TxIdx VarUInt
}

//GetTX 获取交易信息
func (v TxValue) GetTX(bi *BlockIndex) (*TX, error) {
	blk, err := bi.LoadBlock(v.BlkID)
	if err != nil {
		return nil, err
	}
	uidx := v.TxIdx.ToInt()
	if uidx < 0 || uidx >= len(blk.Txs) {
		return nil, errors.New("txsidx out of bound")
	}
	return blk.Txs[uidx], nil
}

//Encode 编码数据
func (v TxValue) Encode(w IWriter) error {
	if err := v.BlkID.Encode(w); err != nil {
		return err
	}
	if err := v.TxIdx.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码数据
func (v *TxValue) Decode(r IReader) error {
	if err := v.BlkID.Decode(r); err != nil {
		return err
	}
	if err := v.TxIdx.Decode(r); err != nil {
		return err
	}
	return nil
}

//Bytes 返回编码二进制数据
func (v TxValue) Bytes() ([]byte, error) {
	buf := NewWriter()
	err := v.Encode(buf)
	return buf.Bytes(), err
}

//HeaderBytes 区块头数据
type HeaderBytes []byte

//Clone 复制区块头
func (b HeaderBytes) Clone() HeaderBytes {
	v := make([]byte, len(b))
	copy(v, b)
	return v
}

//SetNonce 设置随机值
func (b *HeaderBytes) SetNonce(v uint32) {
	l := len(*b)
	Endian.PutUint32((*b)[l-4:], v)
}

//SetTime 设置时间戳
func (b *HeaderBytes) SetTime(v uint32) {
	l := len(*b)
	Endian.PutUint32((*b)[l-12:], v)
}

//Hash 计算hash
func (b *HeaderBytes) Hash() HASH256 {
	return Hash256From(*b)
}

//Header 返回区块头
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
	err := b.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Len()
}

var (
	blockheadersize = getblockheadersize()
)

//BlockHeader 区块头
type BlockHeader struct {
	Ver    uint32  //block ver
	Prev   HASH256 //pre block hash
	Merkle HASH256 //txs Merkle tree hash
	Time   uint32  //时间戳
	Bits   uint32  //难度
	Nonce  uint32  //随机值
	hasher HashCacher
}

//Check 取测区块头是否正确
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

//Bytes 转换位区块数据
func (v BlockHeader) Bytes() HeaderBytes {
	buf := NewWriter()
	err := v.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

//IsGenesis 是否是第一个区块
func (v BlockHeader) IsGenesis() bool {
	id, err := v.ID()
	if err != nil {
		panic(err)
	}
	return v.Prev.IsZero() && conf.IsGenesisID(id)
}

//MustID 获取区块ID
func (v *BlockHeader) MustID() HASH256 {
	id, err := v.ID()
	if err != nil {
		panic(err)
	}
	return id
}

//ResetID 重置并结算ID
func (v *BlockHeader) ResetID() (HASH256, error) {
	v.hasher.Reset()
	return v.ID()
}

//ID 计算ID
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

//Encode 编码数据
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

//Decode 解码数据
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

//Headers 区块头集合
type Headers []BlockHeader

//Encode 编码区块头集合
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

//Decode 解码区块头集合
func (hs *Headers) Decode(r IReader) error {
	num := VarUInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	vs := make([]BlockHeader, num)
	for i := range vs {
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

//Add 添加区块头
func (hs *Headers) Add(h BlockHeader) {
	*hs = append(*hs, h)
}

//Reverse 倒置区块头集合
func (hs *Headers) Reverse() {
	vs := Headers{}
	for i := len(*hs) - 1; i >= 0; i-- {
		vs.Add((*hs)[i])
	}
	*hs = vs
}

//检测区块头的工作量难度
func (hs Headers) checkpow(h uint32, bh BlockHeader, bi *BlockIndex) error {
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

//Check 检测区块头列表高度从height开始
func (hs Headers) Check(height uint32, bi *BlockIndex) error {
	if len(hs) == 0 {
		return errors.New("empty headers")
	}
	nh := NextHeight(height)
	//检测第一个
	prev := hs[0]
	if err := hs.checkpow(nh, prev, bi); err != nil {
		return err
	}
	for i := 1; i < len(hs); i++ {
		curr := hs[i]
		//当前区块的prev指向上一个区块的ID
		if !curr.Prev.Equal(prev.MustID()) {
			return errors.New("prev hash != prev id")
		}
		//当前区块的时间戳必定比上一个区块的大
		if curr.Time <= prev.Time {
			return errors.New("time error")
		}
		//当前区块的难度检测
		if err := hs.checkpow(nh+uint32(i), curr, bi); err != nil {
			return err
		}
		prev = curr
	}
	return nil
}

//BlockInfo txs交易部分和比特币类似
//块大小限制为4M大小
type BlockInfo struct {
	Header BlockHeader //区块头
	Txs    []*TX       //交易记录，类似比特币
	Meta   *TBEle      //指向链节点
	merkel HashCacher  //merkel hash 缓存
}

//遍历区块中输出脚本
func (blk *BlockInfo) EachOutScript(fn func(tx *TX, idx int, script *LockedScript)) {
	for _, tx := range blk.Txs {
		for idx, out := range tx.Outs {
			locker, err := out.Script.ToLocked()
			if err != nil {
				continue
			}
			fn(tx, idx, locker)
		}
	}
}

//GetTx 获取区块交易
func (blk *BlockInfo) GetTx(idx int) (*TX, error) {
	if idx < 0 || idx >= len(blk.Txs) {
		return nil, errors.New("idx outbound")
	}
	return blk.Txs[idx], nil
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
	if buf.Len() > MaxBlockSize {
		return fmt.Errorf("block %v too big", bid)
	}
	//写入索引数据
	bt := bi.blkdb.Index().NewBatch()
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
	if bt.Len() > MaxLogSize || rt.Len() > MaxLogSize {
		return errors.New("opts state logs too big > MAX_LOG_SIZE")
	}
	//保存回退日志
	blk.Meta.Rev, err = bi.blkdb.Rev().Write(rt.Dump())
	if err != nil {
		return err
	}
	//保存区块数据
	blk.Meta.Blk, err = bi.blkdb.Blk().Write(buf.Bytes())
	if err != nil {
		return err
	}
	//保存区块头数据
	hbs, err := blk.Meta.Bytes()
	if err != nil {
		return err
	}
	bt.Put(BlockPrefix, bid[:], hbs)
	//写入索引数据
	return bi.blkdb.Index().Write(bt)
}

func (blk BlockInfo) String() string {
	id, err := blk.ID()
	if err != nil {
		panic(err)
	}
	return id.String()
}

//CoinbaseScript 创建Cosinbase 脚本
func (blk *BlockInfo) CoinbaseScript(ip []byte, bs ...[]byte) (Script, error) {
	return NewCoinbaseScript(blk.Meta.Height, ip, bs...)
}

//CoinbaseReward 获取区块奖励
func (blk *BlockInfo) CoinbaseReward() Amount {
	return GetCoinbaseReward(blk.Meta.Height)
}

//CheckCoinbase 检测coinbas
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
		return errors.New("ins script type error,coinbase script miss")
	}
	if script.Height() != blk.Meta.Height {
		return errors.New("coinbase height != meta height")
	}
	return nil
}

//EndianHeight 获取区块高度的二进制
func (blk *BlockInfo) EndianHeight() []byte {
	return EndianUInt32(blk.Meta.Height)
}

//WriteTxsIdx 写入交易索引
func (blk *BlockInfo) WriteTxsIdx(bi *BlockIndex, bt *Batch) error {
	bid, err := blk.ID()
	if err != nil {
		return err
	}
	//当前区块高度作为排序key
	hb := blk.EndianHeight()
	//交易所在的区块信息和金额信息索引
	for idx, tx := range blk.Txs {
		id, err := tx.ID()
		if err != nil {
			return err
		}
		vval := TxValue{
			TxIdx: VarUInt(idx),
			BlkID: bid,
		}
		vbys, err := vval.Bytes()
		if err != nil {
			return err
		}
		//交易对应的区块和位置
		bt.Put(TxsPrefix, id[:], vbys)
		//这里存储交易和哪些地址有关系
		vps := map[HASH160]bool{}
		//写入金额和索引
		err = tx.writeTxIndex(bi, blk, vps, bt)
		if err != nil {
			return err
		}
		//写入账户相关的交易
		for pkh := range vps {
			bt.Put(TxpPrefix, pkh[:], hb, id[:], vbys)
		}
	}
	return nil
}

//GetMerkle 获取默克尔树
func (blk *BlockInfo) GetMerkle() (HASH256, error) {
	if h, b := blk.merkel.IsSet(); b {
		return h, nil
	}
	ids := []HASH256{}
	for _, tv := range blk.Txs {
		vid, err := tv.ID()
		if err != nil {
			return ZERO256, err
		}
		ids = append(ids, vid)
	}
	root, err := BuildMerkleTree(ids).ExtractRoot()
	if err != nil {
		return root, err
	}
	blk.merkel.SetHash(root)
	return root, nil
}

//SetMerkle 结算并设置默克尔树
func (blk *BlockInfo) SetMerkle() error {
	merkle, err := blk.GetMerkle()
	if err != nil {
		return err
	}
	blk.Header.Merkle = merkle
	return nil
}

//CheckRefsTx 检查引用的tx是否存在区块中
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
		//不允许来自交易池的输出
		if out.IsPool() {
			return fmt.Errorf("refs tx pool error")
		}
	}
	return nil
}

//AddTxs 添加多个交易
//有重复消费输出将会失败
func (blk *BlockInfo) AddTxs(bi *BlockIndex, txs []*TX) error {
	if len(txs) == 0 {
		return errors.New("txs empty")
	}
	//过滤交易数据
	txs = bi.lptr.OnLoadTxs(txs)
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
		if err := blk.CheckRefsTx(bi, tx); err != nil {
			return err
		}
		if err := tx.Check(bi, true); err != nil {
			return err
		}
		//当加入区块时脚本执行错误将忽略
		if err := tx.ExecScript(bi); err != nil {
			continue
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

//HasTx 查找区块内的单个交易是否存在
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

//LoadTxs 从交易池加载可用的交易
func (blk *BlockInfo) LoadTxs(bi *BlockIndex) error {
	txs, err := bi.txp.LoadTxsWithBlk(bi, blk)
	if err != nil {
		return err
	}
	if len(txs) == 0 {
		return nil
	}
	return blk.AddTxs(bi, txs)
}

//MustID 获取区块id
func (blk *BlockInfo) MustID() HASH256 {
	return blk.Header.MustID()
}

//ID 获取区块id
func (blk *BlockInfo) ID() (HASH256, error) {
	return blk.Header.ID()
}

//IsGenesis 是否是第一个区块
func (blk *BlockInfo) IsGenesis() bool {
	return blk.Header.IsGenesis()
}

//CoinbaseFee 获取coinse out fee sum
func (blk *BlockInfo) CoinbaseFee() (Amount, error) {
	if len(blk.Txs) == 0 {
		return 0, errors.New("miss txs")
	}
	return blk.Txs[0].CoinbaseFee()
}

//GetFee 获取总的交易费
func (blk *BlockInfo) GetFee(bi *BlockIndex) (Amount, error) {
	fee := Amount(0)
	for _, tx := range blk.Txs {
		if tx.IsCoinBase() {
			continue
		}
		f, err := tx.GetFeeAmount(bi)
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

//GetIncome 获取区块收益
func (blk *BlockInfo) GetIncome(bi *BlockIndex) (Amount, error) {
	rfee := GetCoinbaseReward(blk.Meta.Height)
	fee, err := blk.GetFee(bi)
	if err != nil {
		return 0, err
	}
	return fee + rfee, nil
}

//CheckTxs 检查所有的交易
//csp 是否检查消费金额是否存在，只有消费此输出得时候才检查，如果对应
//的区块已经连接到主链，输出必定被消费了，只需要检查签名
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
		//检测每个交易
		if err := tx.Check(bi, csp); err != nil {
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

//ResetHasher 重置所有hash缓存
func (blk *BlockInfo) ResetHasher() {
	//重置hash缓存用来计算merkle
	for _, tx := range blk.Txs {
		tx.ResetAll()
	}
	blk.merkel.Reset()
	blk.Header.hasher.Reset()
}

//Finish 完成块数据
func (blk *BlockInfo) Finish(bi *BlockIndex) error {
	if len(blk.Txs) == 0 {
		return errors.New("txs miss, too little")
	}
	//最后设置merkleid
	if err := bi.lptr.OnFinished(blk); err != nil {
		return err
	}
	//重置缓存设置merkle
	blk.ResetHasher()
	if err := blk.SetMerkle(); err != nil {
		return err
	}
	return blk.Check(bi, true)
}

//CheckRepCostTxOut 检查是否有多个输入消费同一个输出
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

//Verify 验证区块数据
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

//ExecScript 执行脚本检测
func (blk BlockInfo) ExecScript(bi *BlockIndex) error {
	for _, tx := range blk.Txs {
		err := tx.ExecScript(bi)
		if err != nil {
			return err
		}
	}
	return nil
}

//Check 检查区块数据
//csp 是否检查消费输出
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
	//检测coinbase
	if err := blk.CheckCoinbase(); err != nil {
		return err
	}
	//检查重复消费
	if err := blk.CheckRepCostTxOut(bi); err != nil {
		return err
	}
	//检查所有的交易
	return blk.CheckTxs(bi, csp)
}

//Encode 编码区块数据
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

//Decode 解码区块数据
func (blk *BlockInfo) Decode(r IReader) error {
	if err := blk.Header.Decode(r); err != nil {
		return err
	}
	txn := VarUInt(0)
	if err := txn.Decode(r); err != nil {
		return err
	}
	blk.Txs = make([]*TX, txn)
	for i := range blk.Txs {
		tx := &TX{}
		if err := tx.Decode(r); err != nil {
			return err
		}
		blk.Txs[i] = tx
	}
	return nil
}

//TxIn 交易输入
type TxIn struct {
	OutHash  HASH256 //输出交易hash
	OutIndex VarUInt //对应的输出索引
	Script   Script  //签名后填充脚本
	Sequence VarUInt //连续号
}

//NewTxIn 创建输入
func NewTxIn() *TxIn {
	return &TxIn{
		Sequence: FinalSequence,
	}
}

//Clone 复制输入
func (in TxIn) Clone(seq ...uint) *TxIn {
	n := NewTxIn()
	n.OutHash = in.OutHash.Clone()
	n.OutIndex = in.OutIndex
	n.Script = in.Script.Clone()
	if len(seq) > 0 && in.Sequence < FinalSequence {
		n.Sequence = in.Sequence + VarUInt(seq[0])
	} else {
		n.Sequence = in.Sequence
	}
	return n
}

//GetCoin 获取引用的coin
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

//SpentKey 消费key,用来记录输入对应的输出是否已被消费
func (in TxIn) SpentKey() []byte {
	buf := NewWriter()
	err := buf.WriteFull(CoinsPrefix)
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

//OutKey 获取输入引用key
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

//LoadTxOut 获取输入引用的输出
func (in *TxIn) LoadTxOut(bi *BlockIndex) (*TxOut, error) {
	if in.OutHash.IsZero() {
		return nil, fmt.Errorf("zero hash id")
	}
	tp := bi.GetTxPool()
	//在链中查找交易
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

//Check 检测输入是否正常
func (in *TxIn) Check(bi *BlockIndex) error {
	if err := in.Script.Check(); err != nil {
		return err
	}
	if in.IsCoinBase() {
		return nil
	} else if in.Script.IsWitness() {
		return nil
	} else {
		return fmt.Errorf("txin unlock script type error")
	}
}

//ForID 计算交易id用到的数据
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
	if err := in.Sequence.Encode(w); err != nil {
		return err
	}
	return nil
}

//Encode 编码交易数据
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
	if err := in.Sequence.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码交易数据
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
	if err := in.Sequence.Decode(r); err != nil {
		return err
	}
	return nil
}

//IsCoinBase txs的第一个一定是coinbase类型
func (in *TxIn) IsCoinBase() bool {
	return in.OutHash.IsZero() && in.OutIndex == 0 && in.Script.IsCoinBase()
}

//TxOut 交易输出
type TxOut struct {
	Value  Amount //输出金额
	Script Script //锁定脚本
	pool   bool   //是否来自交易池中的交易
}

//IsPool 如果来自交易池
func (out TxOut) IsPool() bool {
	return out.pool
}

//Clone 复制输入
func (out TxOut) Clone() *TxOut {
	n := &TxOut{}
	n.Value = out.Value
	n.Script = out.Script.Clone()
	n.pool = out.pool
	return n
}

//GetCoin 获取输入引用的输出 这个输出相关的金额信息
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
		return nil, fmt.Errorf("coin not matured")
	}
	return coin, err
}

//ForID 计算交易id用到的数据
func (out *TxOut) ForID(w IWriter) error {
	if err := out.Value.Encode(w); err != nil {
		return err
	}
	if err := out.Script.ForID(w); err != nil {
		return err
	}
	return nil
}

//HasCoin 获取输入引用的输出 输出对应的coin状态是否正常可用
func (out *TxOut) HasCoin(in *TxIn, bi *BlockIndex) bool {
	pkh, err := out.Script.GetPkh()
	if err != nil {
		panic(fmt.Errorf("get pkh error %w", err))
	}
	coin, err := bi.GetCoin(pkh, in.OutHash, in.OutIndex)
	if err != nil {
		return false
	}
	return coin.IsMatured(bi.NextHeight())
}

//Check 检测输出是否正常
func (out *TxOut) Check(bi *BlockIndex) error {
	if err := out.Script.Check(); err != nil {
		return err
	}
	if !out.Value.IsRange() {
		return fmt.Errorf("txout value error")
	}
	if out.Script.IsLocked() {
		return nil
	}
	return fmt.Errorf("unknow script type")
}

//Encode 编码输出
func (out *TxOut) Encode(w IWriter) error {
	if err := out.Value.Encode(w); err != nil {
		return err
	}
	if err := out.Script.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码输出
func (out *TxOut) Decode(r IReader) error {
	if err := out.Value.Decode(r); err != nil {
		return err
	}
	if err := out.Script.Decode(r); err != nil {
		return err
	}
	return nil
}

//TX 交易
type TX struct {
	Ver    VarUInt    //版本
	Ins    []*TxIn    //输入
	Outs   []*TxOut   //输出
	Script Script     //交易执行脚本，执行失败不会进入交易池
	idcs   HashCacher //hash缓存
	outs   HashCacher //签名hash缓存
	pres   HashCacher //签名hash缓存
	pool   bool       //是否来自内存池
}

//PackExeLimit 合并限制步数和时间
func PackExeLimit(step uint32, time uint32) uint32 {
	if step > MaxExeStepLimit {
		panic(fmt.Errorf("step limit error"))
	}
	if time > MaxExeTimeLimit {
		panic(fmt.Errorf("time limit error"))
	}
	return (step << 16) | time
}

//CheckExeLimit 检测
func CheckExeLimit(v uint32) error {
	step := (v >> 16) & 0xFFFF
	timev := v & 0xFFFF
	if step > MaxExeStepLimit {
		return fmt.Errorf("step limit error")
	}
	if timev > MaxExeTimeLimit {
		return fmt.Errorf("time limit error")
	}
	return nil
}

//ParseExeLimit 解析执行时间和步数
func GetExeLimit(v uint32) (int, time.Duration) {
	step := (v >> 16) & 0xFFFF
	timev := v & 0xFFFF
	if step > MaxExeStepLimit {
		step = MaxExeStepLimit
	}
	if timev > MaxExeTimeLimit {
		timev = MaxExeTimeLimit
	}
	return int(step), time.Microsecond * time.Duration(timev)
}

//ParseExeLimit 解析执行时间和步数
func ParseExeLimit(v uint32) (uint32, uint32) {
	step := (v >> 16) & 0xFFFF
	timev := v & 0xFFFF
	if step > MaxExeStepLimit {
		step = MaxExeStepLimit
	}
	if timev > MaxExeTimeLimit {
		timev = MaxExeTimeLimit
	}
	return step, timev
}

//FixExeLimit 修正执行时间和步数
func FixExeLimit(v uint32) uint32 {
	step := (v >> 16) & 0xFFFF
	timev := v & 0xFFFF
	if step > MaxExeStepLimit {
		step = MaxExeStepLimit
	}
	if timev > MaxExeTimeLimit {
		timev = MaxExeTimeLimit
	}
	return (step << 16) | timev
}

//NewTx 创建交易
//exeLimit 执行脚本限制
func NewTx(exeLimit uint32, execs ...[]byte) *TX {
	tx := &TX{}
	tx.Ver = 1
	tx.Outs = []*TxOut{}
	tx.Ins = []*TxIn{}
	script, err := NewTxScript(exeLimit, execs...)
	if err != nil {
		panic(err)
	}
	tx.Script = script
	return tx
}

//IsFinal 返回true表示交易生效不能替换
func (tx TX) IsFinal() bool {
	for _, in := range tx.Ins {
		if in.Sequence < FinalSequence {
			return false
		}
	}
	return true
}

//IsReplace 当前交易是否可替换原来的交易
//这个替换只能交易池中执行,执行之前签名已经通过
//交易一但被打包到区块就不能替换了，但可能会回退
func (tx TX) IsReplace(old *TX) bool {
	//原交易是final不能替换
	if old.IsFinal() {
		return false
	}
	//新交易是final立即替换
	if tx.IsFinal() {
		return true
	}
	//输入数量必须一致
	if len(tx.Ins) != len(old.Ins) {
		return false
	}
	//每个输入的seq 比之前的大
	for i, in := range tx.Ins {
		ov := old.Ins[i].Sequence
		if in.Sequence <= ov {
			return false
		}
	}
	return true
}

//Clone 复制交易
//seq 如果需要自增输入的Sequence设置
func (tx TX) Clone(seq ...uint) *TX {
	n := NewTx(0)
	n.Ver = tx.Ver
	for _, in := range tx.Ins {
		n.Ins = append(n.Ins, in.Clone(seq...))
	}
	for _, out := range tx.Outs {
		n.Outs = append(n.Outs, out.Clone())
	}
	//交易脚本包含了exetime和脚本
	n.Script = tx.Script.Clone()
	n.pool = tx.pool
	return n
}

//IsPool 是否来自交易池
func (tx TX) IsPool() bool {
	return tx.pool
}

//ResetSign 重置缓存
func (tx *TX) ResetSign() {
	tx.outs.Reset()
	tx.pres.Reset()
}

//ResetAll 重所有置缓存
func (tx *TX) ResetAll() {
	tx.idcs.Reset()
	tx.ResetSign()
}

func (tx TX) String() string {
	id, err := tx.ID()
	if err != nil {
		panic(err)
	}
	return id.String()
}

//IsCoinBase 是否是coinbase交易
func (tx *TX) IsCoinBase() bool {
	return len(tx.Ins) == 1 && tx.Ins[0].IsCoinBase()
}

//写入交易信息索引和回退索引
func (tx *TX) writeTxIndex(bi *BlockIndex, blk *BlockInfo, vps map[HASH160]bool, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return fmt.Errorf("batch miss rev")
	}
	cbase := tx.IsCoinBase()
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
		pkh, err := out.Script.GetPkh()
		if err != nil {
			return err
		}
		vps[pkh] = true //交易相关的pkh
		//引用的金额
		coin, err := bi.GetCoin(pkh, in.OutHash, in.OutIndex)
		if err != nil {
			return err
		}
		//金额是否可用
		if !coin.IsMatured(blk.Meta.Height) {
			return fmt.Errorf("ref out coin not matured")
		}
		//被消费删除
		bt.Del(coin.MustKey())
		//添加回退日志用来恢复,如果是引用本区块的忽略
		rt.Put(coin.MustKey(), coin.MustValue())
	}
	//输出coin
	for idx, out := range tx.Outs {
		pkh, err := out.Script.GetPkh()
		if err != nil {
			return err
		}
		tk := &CoinKeyValue{}
		tk.Value = out.Value
		tk.CPkh = pkh
		tk.Index = VarUInt(idx)
		tk.TxID = tx.MustID()
		if cbase {
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

//Verify 验证交易签名数据
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
		err = NewSigner(tx, out, in, idx).Verify(bi)
		if err != nil {
			return fmt.Errorf("Verify in %d error %w", idx, err)
		}
	}
	return nil
}

//验证输入对应的金额是否存在,不验证签名
func (tx *TX) CoinsIsValid(bi *BlockIndex) bool {
	//签名每一个输入
	for _, in := range tx.Ins {
		if in.IsCoinBase() {
			//不验证coinbase交易
			return true
		}
		//获取引用的输出
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return false
		}
		if !out.HasCoin(in, bi) {
			return false
		}
	}
	return true
}

//Sign 签名交易数据
//cspent 是否检测输出金额是否存在
func (tx *TX) Sign(bi *BlockIndex, lis ISignTx, pass ...string) error {
	//重置签名数据
	tx.ResetSign()
	//签名每一个输入
	for idx, in := range tx.Ins {
		//不签名coinbase
		if in.IsCoinBase() {
			continue
		}
		//获取引用的输出
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		if !out.HasCoin(in, bi) {
			return fmt.Errorf("sign tx, coin miss")
		}
		//对每个输入签名
		err = NewSigner(tx, out, in, idx).Sign(bi, lis, pass...)
		//这个错误可能是部
		if err != nil {
			return fmt.Errorf("sign in %d error %w", idx, err)
		}
	}
	return nil
}

//MustID 必须获取到交易ID
func (tx *TX) MustID() HASH256 {
	id, err := tx.ID()
	if err != nil {
		panic(err)
	}
	return id
}

//ID 交易id计算,不包括见证数据
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
	//执行脚本
	err = tx.Script.Encode(buf)
	if err != nil {
		return id, err
	}
	return tx.idcs.Hash(buf.Bytes()), nil
}

//CoinbaseFee 获取coinse out fee sum
func (tx *TX) CoinbaseFee() (Amount, error) {
	if !tx.IsCoinBase() {
		return 0, fmt.Errorf("tx not coinbase")
	}
	fee := Amount(0)
	for _, out := range tx.Outs {
		fee += out.Value
	}
	if !fee.IsRange() {
		return 0, fmt.Errorf("coinbase fee range error")
	}
	return fee, nil
}

//GetFeeAmount 获取此交易交易费
//如果是coinase返回coinbase输出金额
func (tx *TX) GetFeeAmount(bi *BlockIndex) (Amount, error) {
	if tx.IsCoinBase() {
		return tx.CoinbaseFee()
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
	if !fee.IsRange() {
		return 0, fmt.Errorf("fee range error")
	}
	return fee, nil
}

//HasRepTxIn 检测交易中是否有重复的输入
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

//Check 检测除coinbase交易外的交易金额
//csp是否检测输出金额是否已经被消费,如果交易已经打包进区块，输入引用的输出肯定被消费,coin将不存在
//clk 是否检查seqlock
func (tx *TX) Check(bi *BlockIndex, csp bool) error {
	//至少有一个交易
	if len(tx.Ins) == 0 {
		return fmt.Errorf("tx ins too slow")
	}
	//脚本不能太大
	if err := tx.Script.Check(); err != nil {
		return fmt.Errorf("script check error %w", err)
	}
	//检测输入是否重复引用了相同的输出
	if tx.HasRepTxIn(bi, csp) {
		return fmt.Errorf("txin repeat cost txout")
	}
	//这里不检测coinbase交易
	if tx.IsCoinBase() {
		return nil
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
		if out.IsPool() {
			return fmt.Errorf("has txpool txout error")
		}
		if !out.Value.IsRange() {
			return fmt.Errorf("ref'out value error")
		}
		//是否校验金额是否存在
		if csp && !out.HasCoin(in, bi) {
			return fmt.Errorf("coin miss")
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
		if !out.Value.IsRange() {
			return fmt.Errorf("out value error")
		}
		otv += out.Value
	}
	//金额必须在合理的范围
	if !itv.IsRange() || !otv.IsRange() {
		return fmt.Errorf("in or out amount error")
	}
	//每个交易的输出不能大于输入,差值会输出到coinbase交易当作交易费
	if itv < 0 || otv < 0 || otv > itv {
		return fmt.Errorf("ins amount must >= outs amount")
	}
	//检查签名
	return tx.Verify(bi)
}

//Encode 编码交易数据
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
	if err := tx.Script.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码交易数据
func (tx *TX) Decode(r IReader) error {
	if err := tx.Ver.Decode(r); err != nil {
		return err
	}
	inum := VarUInt(0)
	if err := inum.Decode(r); err != nil {
		return err
	}
	tx.Ins = make([]*TxIn, inum)
	for i := range tx.Ins {
		in := &TxIn{}
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
	for i := range tx.Outs {
		out := &TxOut{}
		err := out.Decode(r)
		if err != nil {
			return err
		}
		tx.Outs[i] = out
	}
	if err := tx.Script.Decode(r); err != nil {
		return err
	}
	return nil
}
