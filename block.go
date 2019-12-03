package xginx

import (
	"errors"
	"fmt"
)

const (
	// 最大块大小
	MAX_BLOCK_SIZE = 1024 * 1024 * 4
	//最大索引数据大小
	MAX_LOG_SIZE = 1024 * 1024 * 2
	//最大扩展数据
	MAX_EXT_SIZE = 4 * 1024
	//锁定时间分界值
	LOCKTIME_THRESHOLD = uint32(500000000)
)

//用户交易索引
type TxIndex struct {
	TxId  HASH256
	Value TxValue
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
	return blk.Txs[uidx], nil
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
	id, _ := v.ID()
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
		return ZERO, err
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

func (blk BlockInfo) String() string {
	id, err := blk.ID()
	if err != nil {
		panic(err)
	}
	return id.String()
}

//创建Cosinbase 脚本
func (blk *BlockInfo) CoinbaseScript(bs ...[]byte) Script {
	return NewCoinbaseScript(blk.Meta.Height, bs...)
}

//获取区块奖励
func (blk *BlockInfo) CoinbaseReward() Amount {
	return GetCoinbaseReward(blk.Meta.Height)
}

//检测coinbas
func (v *BlockInfo) CheckCoinbase() error {
	if v.Meta == nil {
		return errors.New("not set block meta,can't check coinbase")
	}
	if len(v.Txs) < 1 {
		return errors.New("txs count == 0,coinbase miss")
	}
	if len(v.Txs[0].Ins) != 1 {
		return errors.New("ins count == 0,coinbase miss")
	}
	script := v.Txs[0].Ins[0].Script
	if !script.IsCoinBase() {
		return errors.New("ins script type error,coinbase miss")
	}
	if script.Height() != v.Meta.Height {
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
		bt.Put(TXS_PREFIX, id[:], vbys)
		err = tx.writetx(bi, bt)
		if err != nil {
			return err
		}
	}
	//写入账户相关的交易信息索引
	//TXP_PREFIX + pkh + txid -> txvalue
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
		//交易中有哪些账户
		vps := map[HASH160]bool{}
		for _, in := range tx.Ins {
			if in.IsCoinBase() {
				continue
			}
			//消费的输出
			out, err := in.LoadTxOut(bi)
			if err != nil {
				return err
			}
			pkh, err := out.Script.GetPkh()
			if err != nil {
				return err
			}
			vps[pkh] = true
		}
		//输出
		for _, out := range tx.Outs {
			pkh, err := out.Script.GetPkh()
			if err != nil {
				return err
			}
			vps[pkh] = true
		}
		for pkh, _ := range vps {
			key := append([]byte{}, pkh[:]...)
			key = append(key, id[:]...)
			bt.Put(TXP_PREFIX, key, vbys)
		}
	}
	return nil
}

func (v *BlockInfo) GetMerkle() (HASH256, error) {
	if h, b := v.merkel.IsSet(); b {
		return h, nil
	}
	ids := []HASH256{}
	for _, tv := range v.Txs {
		if vid, err := tv.ID(); err != nil {
			return ZERO, err
		} else {
			ids = append(ids, vid)
		}
	}
	root, err := BuildMerkleTree(ids).ExtractRoot()
	if err != nil {
		return root, err
	}
	v.merkel.SetHash(root)
	return root, nil
}

func (v *BlockInfo) SetMerkle() error {
	merkle, err := v.GetMerkle()
	if err != nil {
		return err
	}
	v.Header.Merkle = merkle
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
		if err := tx.CheckLockTime(blk); err != nil {
			return err
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
			return false
		}
		if tid.Equal(id) {
			return true
		}
	}
	return false
}

//添加单个交易
//有重复消费输出将会失败
func (blk *BlockInfo) AddTx(bi *BlockIndex, tx *TX) error {
	if err := tx.CheckLockTime(blk); err != nil {
		return err
	}
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

func (blk *BlockInfo) GetIncome(bi *BlockIndex) (Amount, error) {
	rfee := GetCoinbaseReward(blk.Meta.Height)
	fee, err := blk.GetFee(bi)
	if err != nil {
		return 0, err
	}
	return fee + rfee, nil
}

//检查所有的交易
func (blk *BlockInfo) CheckTxs(bi *BlockIndex) error {
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
		err := tx.Check(bi, false)
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
	//检查所有的交易
	if err := blk.CheckTxs(bi); err != nil {
		return err
	}
	//最后设置merkleid
	if err := lptr.OnFinished(blk); err != nil {
		return err
	}
	//重置缓存设置merkle
	blk.ResetHasher()
	return blk.SetMerkle()
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
	return blk.Check(bi)
}

//检查区块数据
func (blk *BlockInfo) Check(bi *BlockIndex) error {
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
	return blk.CheckTxs(bi)
}

func (v *BlockInfo) Encode(w IWriter) error {
	if err := v.Header.Encode(w); err != nil {
		return err
	}
	if err := VarUInt(len(v.Txs)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Txs {
		if err := v.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockInfo) Decode(r IReader) error {
	if err := v.Header.Decode(r); err != nil {
		return err
	}
	txn := VarUInt(0)
	if err := txn.Decode(r); err != nil {
		return err
	}
	v.Txs = make([]*TX, txn)
	for i, _ := range v.Txs {
		tx := &TX{}
		if err := tx.Decode(r); err != nil {
			return err
		}
		v.Txs[i] = tx
	}
	return nil
}

//交易输入
type TxIn struct {
	OutHash  HASH256 //输出交易hash
	OutIndex VarUInt //对应的输出索引
	Script   Script  //签名后填充脚本
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

//引用的输出是否已经被in消费
func (out *TxOut) IsSpent(in *TxIn, bi *BlockIndex) bool {
	db := bi.GetStoreDB()
	tp := bi.GetTxPool()
	tk := &CoinKeyValue{}
	tk.Value = out.Value
	pkh, err := out.Script.GetPkh()
	if err != nil {
		panic(fmt.Errorf("get pkh error %w", err))
	}
	tk.CPkh = pkh
	tk.Index = in.OutIndex
	tk.TxId = in.OutHash
	//从索引和交易池中查询是否有可用的金额，都不存在肯定已经被消费
	//如果输出来自交易池，从交易池查询
	if key := tk.GetKey(); out.pool {
		return !tp.HasCoin(tk)
	} else {
		return !db.Index().Has(key)
	}
}

func (out *TxOut) Check(bi *BlockIndex) error {
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
func (tx *TX) writetx(bi *BlockIndex, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return errors.New("batch miss rev")
	}
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
		tk := CoinKeyValue{}
		tk.Value = out.Value
		if pkh, err := out.Script.GetPkh(); err != nil {
			return err
		} else {
			tk.CPkh = pkh
		}
		tk.Index = in.OutIndex
		tk.TxId = in.OutHash
		key := tk.GetKey()
		//被消费删除
		bt.Del(key)
		//添加回退日志用来恢复,如果是引用本区块的忽略
		if !out.pool {
			rt.Put(key, out.Value.Bytes())
		}
	}
	//输出coin
	for idx, out := range tx.Outs {
		tk := CoinKeyValue{}
		tk.Value = out.Value
		if pkh, err := out.Script.GetPkh(); err != nil {
			return err
		} else {
			tk.CPkh = pkh
		}
		tk.Index = VarUInt(idx)
		if tid, err := tx.ID(); err != nil {
			return err
		} else {
			tk.TxId = tid
		}
		key := tk.GetKey()
		bt.Put(key, tk.GetValue())
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
	if out.IsSpent(in, bi) {
		return nil, errors.New("out is spent")
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
		if out.IsSpent(in, bi) {
			return errors.New("out is spent")
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
func (v *TX) CoinbaseFee() (Amount, error) {
	if !v.IsCoinBase() {
		return 0, errors.New("tx not coinbase")
	}
	fee := Amount(0)
	for _, out := range v.Outs {
		fee += out.Value
	}
	if !fee.IsRange() {
		return 0, errors.New("amount range error")
	}
	return fee, nil
}

//获取此交易交易费
func (v *TX) GetTransFee(bi *BlockIndex) (Amount, error) {
	if v.IsCoinBase() {
		return 0, errors.New("coinbase not trans fee")
	}
	fee := Amount(0)
	for _, in := range v.Ins {
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return 0, err
		}
		fee += out.Value
	}
	for _, out := range v.Outs {
		fee -= out.Value
	}
	return fee, nil
}

//检测locktime
//当locktime < LOCKTIME_THRESHOLD 表示区块高度限制
//当locktime >= LOCKTIME_THRESHOLD 表示时间戳
func (v *TX) CheckLockTime(blk *BlockInfo) error {
	if v.LockTime == 0 {
		return nil
	}
	if v.LockTime < LOCKTIME_THRESHOLD && v.LockTime < blk.Meta.Height {
		return nil
	}
	if v.LockTime < blk.Meta.Time {
		return nil
	}
	return errors.New("locktime limit,can't join block")
}

//检测除coinbase交易外的交易金额
//csp是否检测输出金额是否已经被消费
func (v *TX) Check(bi *BlockIndex, csp bool) error {
	//这里不检测coinbase交易
	if v.IsCoinBase() {
		return nil
	}
	if len(v.Ins) == 0 {
		return errors.New("tx ins too slow")
	}
	//总的输入金额
	itv := Amount(0)
	for _, in := range v.Ins {
		err := in.Check(bi)
		if err != nil {
			return err
		}
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		if csp && out.IsSpent(in, bi) {
			return errors.New("out is spent")
		}
		itv += out.Value
	}
	//总的输出金额
	otv := Amount(0)
	for _, out := range v.Outs {
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
	return v.Verify(bi)
}

func (v *TX) Encode(w IWriter) error {
	if err := v.Ver.Encode(w); err != nil {
		return err
	}
	if err := VarUInt(len(v.Ins)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Ins {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	if err := VarUInt(len(v.Outs)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Outs {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	if err := w.TWrite(v.LockTime); err != nil {
		return err
	}
	return nil
}

func (v *TX) Decode(r IReader) error {
	if err := v.Ver.Decode(r); err != nil {
		return err
	}
	inum := VarUInt(0)
	if err := inum.Decode(r); err != nil {
		return err
	}
	v.Ins = make([]*TxIn, inum)
	for i, _ := range v.Ins {
		in := &TxIn{}
		err := in.Decode(r)
		if err != nil {
			return err
		}
		v.Ins[i] = in
	}
	onum := VarUInt(0)
	if err := onum.Decode(r); err != nil {
		return err
	}
	v.Outs = make([]*TxOut, onum)
	for i, _ := range v.Outs {
		out := &TxOut{}
		err := out.Decode(r)
		if err != nil {
			return err
		}
		v.Outs[i] = out
	}
	if err := r.TRead(&v.LockTime); err != nil {
		return err
	}
	return nil
}
