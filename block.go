package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	// 最大块大小
	MAX_BLOCK_SIZE = 1024 * 1024 * 4
	//最大ExtScript大小
	MAX_SCRIPT_SIZE = 4 * 1024
)

//存储交易索引值
type TxValue struct {
	BlkId  HASH256 //块hash
	TxsIdx VarUInt //txs 索引
}

func (v TxValue) GetTX(bi *BlockIndex) (*TX, error) {
	blk, err := bi.LoadBlock(v.BlkId)
	if err != nil {
		return nil, err
	}
	uidx := v.TxsIdx.ToInt()
	if uidx < 0 || uidx >= len(blk.Txs) {
		return nil, errors.New("txsidx out of bound")
	}
	return blk.Txs[uidx], nil
}

func (v TxValue) Encode(w IWriter) error {
	if err := v.BlkId.Encode(w); err != nil {
		return err
	}
	if err := v.TxsIdx.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *TxValue) Decode(r IReader) error {
	if err := v.BlkId.Decode(r); err != nil {
		return err
	}
	if err := v.TxsIdx.Decode(r); err != nil {
		return err
	}
	return nil
}

func (v TxValue) Bytes() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := v.Encode(buf)
	return buf.Bytes(), err
}

//区块头数据
type HeaderBytes []byte

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

func (b *HeaderBytes) Header() *BlockHeader {
	buf := bytes.NewReader(*b)
	hptr := &BlockHeader{}
	err := hptr.Decode(buf)
	if err != nil {
		panic(err)
	}
	return hptr
}

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

func (v BlockHeader) Bytes() HeaderBytes {
	buf := &bytes.Buffer{}
	err := v.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (v BlockHeader) IsGenesis() bool {
	return v.Prev.IsZero() && conf.genesisId.Equal(v.ID())
}

func (v *BlockHeader) ID() HASH256 {
	if h, has := v.hasher.IsSet(); has {
		return h
	}
	buf := &bytes.Buffer{}
	err := v.Encode(buf)
	if err != nil {
		panic(err)
	}
	return v.hasher.Hash(buf.Bytes())
}

//一个记录单元必须同一个用户连续的链数据
//块信息
//Bodys记录中不能用相同的clientid，items必须时间上连续，hash能前后衔接
//txs交易部分和比特币类似
//块大小限制为4M大小
type BlockInfo struct {
	Header BlockHeader //区块头
	Txs    []*TX       //交易记录，类似比特币
	Meta   *TBEle      //指向链数据节点
	utsher HashCacher  //uts 缓存
	merher HashCacher  //mer hash 缓存
}

//消费out
func (v *BlockInfo) costOut(bi *BlockIndex, tv *TX, in *TxIn, out *TxOut, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return errors.New("batch miss rev")
	}
	if out.Value == 0 {
		return errors.New("out value zero")
	}
	tk := CoinKeyValue{}
	tk.Value = out.Value.ToVarUInt()
	pkh, err := out.GetPKH()
	if err != nil {
		return err
	}
	tk.CPkh = pkh
	tk.Index = in.OutIndex
	tk.TxId = in.OutHash
	key := tk.GetKey()
	//消耗积分后删除输出
	bt.Del(key)
	//添加恢复日志
	rt.Put(key, out.Value.Bytes())
	return nil
}

//添加out
func (v *BlockInfo) incrOut(bi *BlockIndex, tv *TX, idx int, out *TxOut, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return errors.New("rev batch miss")
	}
	if out.Value == 0 {
		return errors.New("out value zero")
	}
	tk := CoinKeyValue{}
	tk.Value = out.Value.ToVarUInt()
	pkh, err := out.GetPKH()
	if err != nil {
		return err
	}
	tk.CPkh = pkh
	tk.Index = VarUInt(idx)
	tk.TxId = tv.Hash()
	bt.Put(tk.GetKey(), tk.GetValue())
	return nil
}

func (v *BlockInfo) writeTxIns(bi *BlockIndex, tv *TX, ins []*TxIn, b *Batch) error {
	r := b.GetRev()
	if r == nil {
		return errors.New("batch miss rev")
	}
	for _, in := range ins {
		if in.IsCoinBase() {
			continue
		}
		//out将被消耗掉
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		err = v.costOut(bi, tv, in, out, b)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockInfo) writeTxOuts(bi *BlockIndex, tv *TX, outs []*TxOut, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return errors.New("batch miss rev")
	}
	for idx, out := range outs {
		err := v.incrOut(bi, tv, idx, out, bt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockInfo) WriteTxsIdx(bi *BlockIndex, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return errors.New("batch miss rev")
	}
	for i, tv := range v.Txs {
		//保存交易id对应的区块和位置
		tid := tv.Hash()
		vval := TxValue{
			BlkId:  v.ID(),
			TxsIdx: VarUInt(i),
		}
		vbys, err := vval.Bytes()
		if err != nil {
			return err
		}
		bt.Put(TXS_PREFIX, tid[:], vbys)
		//处理交易输入
		err = v.writeTxIns(bi, tv, tv.Ins, bt)
		if err != nil {
			return err
		}
		//处理交易输出
		err = v.writeTxOuts(bi, tv, tv.Outs, bt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockInfo) GetMerkle() (HASH256, error) {
	if h, b := v.merher.IsSet(); b {
		return h, nil
	}
	ids := []HASH256{}
	for _, tv := range v.Txs {
		ids = append(ids, tv.Hash())
	}
	root := BuildMerkleTree(ids).ExtractRoot()
	v.merher.SetHash(root)
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

func (b *BlockInfo) AddTx(bi *BlockIndex, tx *TX) error {
	if err := tx.Check(bi, b); err != nil {
		return err
	}
	b.Txs = append(b.Txs, tx)
	//发布交易
	bi.PublishTx(b, tx)
	return nil
}

func (b *BlockInfo) ID() HASH256 {
	return b.Header.ID()
}

func (b *BlockInfo) IsGenesis() bool {
	return b.Header.IsGenesis()
}

//HASH256 meta,bytes
func (b *BlockInfo) ToTBMeta() (HASH256, *TBMeta, []byte, error) {
	meta := &TBMeta{
		BlockHeader: b.Header,
		Txs:         VarUInt(len(b.Txs)),
	}
	id := meta.ID()
	buf := &bytes.Buffer{}
	if err := b.Encode(buf); err != nil {
		return id, nil, nil, err
	}
	if buf.Len() > MAX_BLOCK_SIZE {
		return id, nil, nil, errors.New("block too big > MAX_BLOCK_SIZE")
	}
	return id, meta, buf.Bytes(), nil
}

//获取coinse out fee sum
func (v *BlockInfo) CoinbaseFee() Amount {
	if len(v.Txs) == 0 {
		panic(errors.New("miss txs"))
	}
	return v.Txs[0].CoinbaseFee()
}

//获取总的交易费
func (v *BlockInfo) GetFee(bi *BlockIndex) Amount {
	fee := Amount(0)
	for _, tx := range v.Txs {
		fee += tx.GetFee(bi)
	}
	return fee
}

//检查所有的交易
func (v *BlockInfo) CheckTxs(bi *BlockIndex) error {
	//奖励
	rfee := GetCoinbaseReward(v.Meta.Height)
	if !rfee.IsRange() {
		return errors.New("coinbase reward amount error")
	}
	//检测所有交易
	for i, tx := range v.Txs {
		if i == 0 && !tx.IsCoinBase() {
			return errors.New("coinbase tx miss")
		}
		err := tx.Check(bi, v)
		if err != nil {
			return err
		}
	}
	//获取交易费
	tfee := v.GetFee(bi)
	if !tfee.IsRange() {
		return errors.New("trans fee error")
	}
	//coinbase输出
	cfee := v.CoinbaseFee()
	if !cfee.IsRange() {
		return errors.New("coinbase fee error")
	}
	fee := rfee + tfee
	if !fee.IsRange() {
		return errors.New("sum fee fee error")
	}
	if cfee > fee {
		return errors.New("coinbase fee error")
	}
	return nil
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
	if err := lptr.OnFinished(bi, blk); err != nil {
		return err
	}
	return blk.SetMerkle()
}

//检查工作难度
func (v *BlockInfo) CheckPow() error {
	if !CheckProofOfWork(v.ID(), v.Header.Bits) {
		return errors.New("proof of work bits error")
	}
	return nil
}

//检查区块数据
func (v *BlockInfo) Check(bi *BlockIndex) error {
	if len(v.Txs) == 0 {
		return errors.New("txs miss, too little")
	}
	//检查所有的交易
	if err := v.CheckTxs(bi); err != nil {
		return err
	}
	//检查merkle树
	merkle, err := v.GetMerkle()
	if err != nil {
		return err
	}
	if !merkle.Equal(v.Header.Merkle) {
		return errors.New("txs merkle hash error")
	}
	//检查区块大小
	buf := &bytes.Buffer{}
	if err := v.Encode(buf); err != nil {
		return err
	}
	if buf.Len() > MAX_BLOCK_SIZE {
		return errors.New("block size > MAX_BLOCK_SIZE")
	}
	//发布区块
	bi.PublishBlk(v)
	return nil
}

func (v *BlockHeader) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, v.Ver); err != nil {
		return err
	}
	if err := v.Prev.Encode(w); err != nil {
		return err
	}
	if err := v.Merkle.Encode(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, v.Time); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, v.Bits); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, v.Nonce); err != nil {
		return err
	}
	return nil
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

func (v *BlockHeader) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &v.Ver); err != nil {
		return err
	}
	if err := v.Prev.Decode(r); err != nil {
		return err
	}
	if err := v.Merkle.Decode(r); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.Time); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.Bits); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &v.Nonce); err != nil {
		return err
	}
	return nil
}

func (v *BlockInfo) Decode(r IReader) error {
	if err := v.Header.Decode(r); err != nil {
		return err
	}
	tnum := VarUInt(0)
	if err := tnum.Decode(r); err != nil {
		return err
	}
	v.Txs = make([]*TX, tnum)
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
	Script   Script  //解锁脚本
}

//获取对应的输出
func (v *TxIn) LoadTxOut(bi *BlockIndex) (*TxOut, error) {
	otx, err := bi.LoadTX(v.OutHash)
	if err != nil {
		return nil, fmt.Errorf("txin outtx miss %w", err)
	}
	idx := v.OutIndex.ToInt()
	if idx < 0 || idx >= len(otx.Outs) {
		return nil, fmt.Errorf("outindex out of bound")
	}
	return otx.Outs[idx], nil
}

func (v *TxIn) Check(bi *BlockIndex) error {
	if v.IsCoinBase() {
		return nil
	} else if v.Script.IsStdUnlockScript() {
		return nil
	} else {
		return errors.New("txin unlock script type error")
	}
}

func (v *TxIn) Encode(w IWriter) error {
	if err := v.OutHash.Encode(w); err != nil {
		return err
	}
	if err := v.OutIndex.Encode(w); err != nil {
		return err
	}
	if err := v.Script.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *TxIn) Decode(r IReader) error {
	if err := v.OutHash.Decode(r); err != nil {
		return err
	}
	if err := v.OutIndex.Decode(r); err != nil {
		return err
	}
	if err := v.Script.Decode(r); err != nil {
		return err
	}
	return nil
}

//是否基本单元，txs的第一个一定是base类型
func (in *TxIn) IsCoinBase() bool {
	return in.OutHash.IsZero() && in.OutIndex == 0 && in.Script.IsBaseScript()
}

const (
	COIN      = Amount(100000000)
	MAX_MONEY = Amount(21000000 * COIN)
)

//结算当前奖励
func GetCoinbaseReward(h uint32) Amount {
	halvings := int(h) / conf.Halving
	if halvings >= 64 {
		return 0
	}
	n := 50 * COIN
	n >>= halvings
	return n
}

type Amount int64

func (a *Amount) Decode(r IReader) error {
	v := int64(0)
	err := binary.Read(r, Endian, &v)
	if err != nil {
		return err
	}
	*a = Amount(v)
	return nil
}

func (a Amount) Bytes() []byte {
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutVarint(lb, int64(a))
	return lb[:l]
}

func (v *Amount) From(b []byte) int {
	vv, l := binary.Varint(b)
	*v = Amount(vv)
	return l
}

func (a Amount) ToVarUInt() VarUInt {
	return VarUInt(a)
}

func (a Amount) Encode(w IWriter) error {
	return binary.Write(w, Endian, int64(a))
}

func (a Amount) IsRange() bool {
	return a >= 0 && a < MAX_MONEY
}

//交易输出
type TxOut struct {
	Value  Amount //距离奖励 GetRewardRate 计算比例，所有输出之和不能高于总奖励
	Script Script //锁定脚本
}

//获取签名解锁器
func (v *TxOut) GetSigner(bi *BlockIndex, tx *TX, in *TxIn, idx int) (ISigner, error) {
	if v.Script.IsStdLockedcript() {
		return newStdSigner(bi, tx, v, in, idx), nil
	}
	return nil, errors.New("not support")
}

//获取输出金额所属公钥hash
func (v *TxOut) GetPKH() (HASH160, error) {
	pkh := HASH160{}
	if v.Script.IsStdLockedcript() {
		return v.Script.StdPKH(), nil
	}
	return pkh, errors.New("unknow script type")
}

//输出是否可以被in消费
func (v *TxOut) IsSpent(in *TxIn, bi *BlockIndex) error {
	tk := CoinKeyValue{}
	tk.Value = v.Value.ToVarUInt()
	pkh, err := v.GetPKH()
	if err != nil {
		return err
	}
	tk.CPkh = pkh
	tk.Index = in.OutIndex
	tk.TxId = in.OutHash
	key := tk.GetKey()
	if !bi.db.Index().Has(key) {
		return errors.New("out is spent")
	}
	return nil
}

func (v *TxOut) Check(bi *BlockIndex) error {
	if v.Script.IsStdLockedcript() {
		return nil
	}
	return errors.New("unknow script type")
}

func (v *TxOut) Encode(w IWriter) error {
	if err := v.Value.Encode(w); err != nil {
		return err
	}
	if err := v.Script.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *TxOut) Decode(r IReader) error {
	if err := v.Value.Decode(r); err != nil {
		return err
	}
	if err := v.Script.Decode(r); err != nil {
		return err
	}
	return nil
}

//交易
type TX struct {
	Ver    VarUInt    //版本
	Ins    []*TxIn    //输入
	Outs   []*TxOut   //输出
	hasher HashCacher //hash缓存
}

//第一个必须是base交易
func (tx *TX) IsCoinBase() bool {
	return len(tx.Ins) == 1 && tx.Ins[0].IsCoinBase()
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
		signer, err := out.GetSigner(bi, tx, in, idx)
		if err != nil {
			return err
		}
		err = signer.Verify()
		if err != nil {
			return fmt.Errorf("Verify in %d error %w", idx, err)
		}
	}
	return bi.SetTx(tx)
}

//签名交易数据
func (tx *TX) Sign(bi *BlockIndex, blk *BlockInfo) error {
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
		if err := out.IsSpent(in, bi); err != nil {
			return err
		}
		signer, err := out.GetSigner(bi, tx, in, idx)
		if err != nil {
			return err
		}
		pri, err := lptr.OnPrivateKey(bi, blk, out)
		if err != nil {
			return err
		}
		err = signer.Sign(pri)
		if err != nil {
			return fmt.Errorf("sign in %d error %w", idx, err)
		}
	}
	return bi.SetTx(tx)
}

func (tx *TX) Hash() HASH256 {
	if hash, ok := tx.hasher.IsSet(); ok {
		return hash
	}
	h := HASH256{}
	buf := &bytes.Buffer{}
	_ = tx.Encode(buf)
	copy(h[:], Hash256(buf.Bytes()))
	return tx.hasher.Hash(buf.Bytes())
}

//获取coinse out fee sum
func (v *TX) CoinbaseFee() Amount {
	if !v.IsCoinBase() {
		panic(errors.New("tx not coinbase"))
	}
	a := Amount(0)
	for _, out := range v.Outs {
		a += out.Value
	}
	return a
}

//获取此交易交易费
func (v *TX) GetFee(bi *BlockIndex) Amount {
	if v.IsCoinBase() {
		return 0
	}
	a := Amount(0)
	for _, in := range v.Ins {
		out, err := in.LoadTxOut(bi)
		if err != nil {
			panic(err)
		}
		err = out.IsSpent(in, bi)
		if err != nil {
			panic(err)
		}
		a += out.Value
	}
	for _, out := range v.Outs {
		a -= out.Value
	}
	return a
}

//检测除coinbase交易外的交易金额
func (v *TX) Check(bi *BlockIndex, b *BlockInfo) error {
	if len(v.Ins) == 0 {
		return errors.New("tx ins too slow")
	}
	//这里不检测coinbase交易
	if v.IsCoinBase() {
		return nil
	}
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
		err = out.IsSpent(in, bi)
		if err != nil {
			return err
		}
		itv += out.Value
	}
	otv := Amount(0)
	for _, out := range v.Outs {
		err := out.Check(bi)
		if err != nil {
			return err
		}
		otv += out.Value
	}
	//金额必须在合理的范围
	if !itv.IsRange() {
		return errors.New("in amount error")
	}
	if !otv.IsRange() {
		return errors.New("out amount error")
	}
	//每个交易的输出不能大于输入
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
	return nil
}
