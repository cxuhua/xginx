package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	// 最大块大小
	MAX_BLOCK_SIZE = 1024 * 1024 * 4
	//最大索引数据大小
	MAX_LOG_SIZE = 1024 * 1024
	//最大扩展数据
	MAX_EXT_SIZE = 4 * 1024
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

func (b *HeaderBytes) SetTime(v time.Time) {
	l := len(*b)
	Endian.PutUint32((*b)[l-12:], uint32(v.Unix()))
}

func (b *HeaderBytes) Hash() HASH256 {
	return Hash256From(*b)
}

func (b *HeaderBytes) Header() BlockHeader {
	buf := bytes.NewReader(*b)
	hptr := BlockHeader{}
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

//查询本区块中的交易
//只查找之前添加的交易
func (blk *BlockInfo) FindTx(id HASH256, idx int) (*TX, error) {
	for i, v := range blk.Txs {
		if i > idx {
			return nil, errors.New("find out bound")
		}
		if id.Equal(v.ID()) {
			return v, nil
		}
	}
	return nil, errors.New("not found tx in block")
}

//创建Cosinbase 脚本
func (blk *BlockInfo) CoinbaseScript(bs ...[]byte) Script {
	return GetCoinbaseScript(blk.Meta.Height, bs...)
}

//获取区块奖励
func (blk *BlockInfo) CoinbaseReward() Amount {
	return GetCoinbaseReward(blk.Meta.Height)
}

func (v *BlockInfo) WriteTxsIdx(bi *BlockIndex, bt *Batch) error {
	for i, tx := range v.Txs {
		err := tx.Write(bi, v, i, bt)
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
		ids = append(ids, tv.ID())
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

func (blk *BlockInfo) AddTx(bi *BlockIndex, tx *TX) error {
	otxs := blk.Txs
	idx := len(blk.Txs) - 1
	//检测交易是否可进行
	if err := tx.Check(bi, blk, idx); err != nil {
		return err
	}
	blk.Txs = append(blk.Txs, tx)
	//不允许重复消费同一个输出
	if err := blk.CheckMulCostTxOut(bi); err != nil {
		blk.Txs = otxs
		return err
	}
	return nil
}

func (blk *BlockInfo) ID() HASH256 {
	return blk.Header.ID()
}

func (blk *BlockInfo) IsGenesis() bool {
	return blk.Header.IsGenesis()
}

//HASH256 meta,bytes
func (blk *BlockInfo) ToTBMeta() (HASH256, *TBMeta, []byte, error) {
	meta := &TBMeta{
		BlockHeader: blk.Header,
		Txs:         VarUInt(len(blk.Txs)),
	}
	id := meta.ID()
	buf := &bytes.Buffer{}
	if err := blk.Encode(buf); err != nil {
		return id, nil, nil, err
	}
	if buf.Len() > MAX_BLOCK_SIZE {
		return id, nil, nil, errors.New("block too big > MAX_BLOCK_SIZE")
	}
	return id, meta, buf.Bytes(), nil
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
	for idx, tx := range blk.Txs {
		if tx.IsCoinBase() {
			continue
		}
		f, err := tx.GetFee(bi, blk, idx)
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

//检查所有的交易
func (blk *BlockInfo) CheckTxs(bi *BlockIndex) error {
	//必须有交易
	if len(blk.Txs) == 0 {
		return errors.New("txs miss, too little")
	}
	rfee := GetCoinbaseReward(blk.Meta.Height)
	if !rfee.IsRange() {
		return errors.New("coinbase reward amount error")
	}
	//检测所有交易
	for i, tx := range blk.Txs {
		if i == 0 && !tx.IsCoinBase() {
			return errors.New("coinbase tx miss")
		}
		err := tx.Check(bi, blk, i)
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

//10000000
func (blk *BlockInfo) CalcPowHash(cnt uint32, bi *BlockIndex) error {
	hb := blk.Header.Bytes()
	for i := uint32(0); i < cnt; i++ {
		id := hb.Hash()
		if !CheckProofOfWork(id, blk.Header.Bits) {
			hb.SetNonce(i)
		} else {
			blk.Header = hb.Header()
			log.Printf("new block success ID=%v Bits=%x Height=%x\n", blk.ID(), blk.Meta.Bits, blk.Meta.Height)
			return nil
		}
		if cnt > 0 && i%(cnt/10) == 0 {
			log.Printf("genblock bits=%x ID=%v Nonce=%x Height=%d\n", blk.Meta.Bits, id, i, blk.Meta.Height)
		}
	}
	return errors.New("calc bits failed")
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

//检查是否有多个输入消费同一个输出
func (blk *BlockInfo) CheckMulCostTxOut(bi *BlockIndex) error {
	imap := map[string]bool{}
	for _, tx := range blk.Txs {
		for _, in := range tx.Ins {
			key := fmt.Sprintf("%v-%d", in.OutHash, in.OutIndex)
			if _, has := imap[key]; has {
				return errors.New("mul txin cost one txout error")
			}
			imap[key] = true
		}
	}
	imap = nil
	return nil
}

//检查区块数据
func (blk *BlockInfo) Check(bi *BlockIndex, cpow bool) error {
	//检测工作难度
	if cpow && !CheckProofOfWork(blk.ID(), blk.Header.Bits) {
		return errors.New("block bits error")
	}
	//检查merkle树
	merkle, err := blk.GetMerkle()
	if err != nil {
		return err
	}
	if !merkle.Equal(blk.Header.Merkle) {
		return errors.New("txs merkle hash error")
	}
	if err := blk.CheckMulCostTxOut(bi); err != nil {
		return err
	}
	//检查所有的交易
	if err := blk.CheckTxs(bi); err != nil {
		return err
	}
	//检查区块大小
	buf := &bytes.Buffer{}
	if err := blk.Encode(buf); err != nil {
		return err
	}
	if buf.Len() > MAX_BLOCK_SIZE {
		return errors.New("block size > MAX_BLOCK_SIZE")
	}
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
	Script   Script
}

//获取对应的输出
//并区分是否来自本区块
func (v *TxIn) LoadTxOut(bi *BlockIndex, blk *BlockInfo, idx int) (*TxOut, bool, error) {
	if v.OutHash.IsZero() {
		return nil, false, errors.New("zero hash id")
	}
	self := false
	otx, err := bi.LoadTX(v.OutHash)
	if err != nil {
		otx, err = blk.FindTx(v.OutHash, idx) //消费的交易在本区块中，并且肯定在之前就已经添加进去
		self = true
	}
	if err != nil {
		return nil, self, fmt.Errorf("txin outtx miss %w", err)
	}
	oidx := v.OutIndex.ToInt()
	if oidx < 0 || oidx >= len(otx.Outs) {
		return nil, self, fmt.Errorf("outindex out of bound")
	}
	return otx.Outs[oidx], self, nil
}

func (v *TxIn) Check(bi *BlockIndex) error {
	if v.IsCoinBase() {
		return nil
	} else if v.Script.IsWitness() {
		return nil
	} else {
		return errors.New("txin unlock script type error")
	}
}

func (v *TxIn) ForID(w IWriter) error {
	if err := v.OutHash.Encode(w); err != nil {
		return err
	}
	if err := v.OutIndex.Encode(w); err != nil {
		return err
	}
	if err := v.Script.ForID(w); err != nil {
		return err
	}
	return nil
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
	return in.OutHash.IsZero() && in.OutIndex == 0 && in.Script.IsCoinBase()
}

//交易输出
type TxOut struct {
	Value  Amount //距离奖励 GetRewardRate 计算比例，所有输出之和不能高于总奖励
	Script Script //锁定脚本
}

//获取签名解锁器
func (v *TxOut) GetSigner(bi *BlockIndex, tx *TX, in *TxIn, idx int) ISigner {
	return newStdSigner(bi, tx, v, in, idx)
}

//输出是否可以被in消费在blk区块中
func (v *TxOut) IsSpent(in *TxIn, bi *BlockIndex, blk *BlockInfo) bool {
	if !bi.IsCheckSpent(blk) {
		return false
	}
	tk := CoinKeyValue{}
	tk.Value = v.Value
	tk.CPkh = v.Script.GetPkh()
	tk.Index = in.OutIndex
	tk.TxId = in.OutHash
	key := tk.GetKey()
	return !bi.db.Index().Has(key)
}

func (v *TxOut) Check(bi *BlockIndex) error {
	if v.Script.IsLocked() {
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

type TxExt struct {
	Bytes VarBytes //扩展数据，不参与签名和id计算
	Hash  HASH256  //扩展数据hash，有数据时参与id和签名计算
}

func (v *TxExt) Check() error {
	if v.Bytes.Len() == 0 {
		return nil
	}
	if v.Bytes.Len() > MAX_EXT_SIZE {
		return errors.New("ext data too big")
	}
	if v.Hash.Equal(Hash256From(v.Bytes)) {
		return nil
	}
	return errors.New("data hash error")
}

//有数据用hash参与签名
func (v *TxExt) ForVerify(w IWriter) error {
	if v.Bytes.Len() == 0 {
		return nil
	}
	if err := v.Hash.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *TxExt) ForID(w IWriter) error {
	if v.Bytes.Len() == 0 {
		return nil
	}
	if err := v.Hash.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *TxExt) Encode(w IWriter) error {
	if err := v.Bytes.Encode(w); err != nil {
		return err
	}
	if v.Bytes.Len() == 0 {
		return nil
	}
	if err := v.Hash.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *TxExt) Decode(r IReader) error {
	if err := v.Bytes.Decode(r); err != nil {
		return err
	}
	if v.Bytes.Len() == 0 {
		return nil
	}
	if err := v.Hash.Decode(r); err != nil {
		return err
	}
	return nil
}

//交易
type TX struct {
	Ver  VarUInt    //版本
	Ins  []*TxIn    //输入
	Outs []*TxOut   //输出
	Ext  TxExt      //扩展数据
	idhc HashCacher //hash缓存
	outs HashCacher //签名hash缓存
	pres HashCacher //签名hash缓存
}

func (tx *TX) HasExt() bool {
	return tx.Ext.Bytes.Len() > 0
}

//扩展数据id
//hash160(exthash+txid) -> blockid+txindx
func (tx *TX) ExtID() (HASH160, bool) {
	id := HASH160{}
	if !tx.HasExt() {
		return id, false
	}
	buf := &bytes.Buffer{}
	err := tx.Ext.Hash.Encode(buf)
	if err != nil {
		return id, false
	}
	err = tx.ID().Encode(buf)
	if err != nil {
		return id, false
	}
	id = Hash160From(buf.Bytes())
	return id, true
}

//重置缓存
func (tx *TX) ResetAll() {
	tx.idhc.Reset()
	tx.outs.Reset()
	tx.pres.Reset()
}

//第一个必须是base交易
func (tx *TX) IsCoinBase() bool {
	return len(tx.Ins) == 1 && tx.Ins[0].IsCoinBase()
}

func (tx *TX) Write(bi *BlockIndex, blk *BlockInfo, idx int, bt *Batch) error {
	rt := bt.GetRev()
	if rt == nil {
		return errors.New("batch miss rev")
	}
	id := tx.ID()
	vval := TxValue{
		BlkId:  blk.ID(),
		TxsIdx: VarUInt(idx),
	}
	vbys, err := vval.Bytes()
	if err != nil {
		return err
	}
	bt.Put(TXS_PREFIX, id[:], vbys)
	//输入coin
	for _, in := range tx.Ins {
		if in.IsCoinBase() {
			continue
		}
		//out将被消耗掉
		out, self, err := in.LoadTxOut(bi, blk, idx)
		if err != nil {
			return err
		}
		if out.Value == 0 {
			continue
		}
		tk := CoinKeyValue{}
		tk.Value = out.Value
		tk.CPkh = out.Script.GetPkh()
		tk.Index = in.OutIndex
		tk.TxId = in.OutHash
		key := tk.GetKey()
		bt.Del(key)
		if !self {
			rt.Put(key, out.Value.Bytes())
		}
	}
	//输出coin
	for idx, out := range tx.Outs {
		//金额为0不写入coin索引
		if out.Value == 0 {
			continue
		}
		tk := CoinKeyValue{}
		tk.Value = out.Value
		tk.CPkh = out.Script.GetPkh()
		tk.Index = VarUInt(idx)
		tk.TxId = tx.ID()
		key := tk.GetKey()
		bt.Put(key, tk.GetValue())
	}
	//是否写入扩展数据
	if extId, has := tx.ExtID(); has {
		ev := ExtKeyValue{
			BlkId:  blk.ID(),
			TxIdx:  VarUInt(idx),
			ExtLen: VarUInt(tx.Ext.Bytes.Len()),
		}
		eb, err := ev.GetValue()
		if err != nil {
			return err
		}
		bt.Put(EXT_PREFIX, extId[:], eb)
	}
	return nil
}

//验证交易输入数据
func (tx *TX) Verify(bi *BlockIndex, blk *BlockInfo, idx int) error {
	for iidx, in := range tx.Ins {
		//不验证base的签名
		if in.IsCoinBase() {
			continue
		}
		out, _, err := in.LoadTxOut(bi, blk, idx)
		if err != nil {
			return err
		}
		err = out.GetSigner(bi, tx, in, iidx).Verify()
		if err != nil {
			return fmt.Errorf("Verify in %d error %w", iidx, err)
		}
	}
	return nil
}

//签名交易数据
func (tx *TX) Sign(bi *BlockIndex, blk *BlockInfo, idxs ...int) error {
	lptr := bi.GetListener()
	if lptr == nil {
		return errors.New("block index listener null,can't sign")
	}
	idx := -1
	if len(idxs) == 0 {
		idx = len(blk.Txs) - 1
	} else {
		idx = idxs[0]
	}
	if idx < 0 {
		return errors.New("idx args error")
	}
	for iidx, in := range tx.Ins {
		if in.IsCoinBase() {
			continue
		}
		out, self, err := in.LoadTxOut(bi, blk, idx)
		if err != nil {
			return err
		}
		//来自自身不检查是否被消费，因为索引中肯定没有
		if !self && out.IsSpent(in, bi, blk) {
			return errors.New("out is spent")
		}
		pri, err := lptr.OnPrivateKey(bi, blk, out)
		if err != nil {
			return err
		}
		err = out.GetSigner(bi, tx, in, iidx).Sign(pri)
		if err != nil {
			return fmt.Errorf("sign in %d error %w", iidx, err)
		}
	}
	return nil
}

//设置扩展数据
func (tx *TX) SetExt(bb []byte) {
	if len(bb) > MAX_EXT_SIZE {
		panic(errors.New("ext > max_ext_size"))
	}
	if len(bb) == 0 {
		return
	}
	tx.Ext = TxExt{
		Bytes: bb,
		Hash:  Hash256From(bb),
	}
}

//交易id计算
func (tx *TX) ID() HASH256 {
	if hash, ok := tx.idhc.IsSet(); ok {
		return hash
	}
	buf := &bytes.Buffer{}
	if err := tx.Ver.Encode(buf); err != nil {
		panic(err)
	}
	if err := VarUInt(len(tx.Ins)).Encode(buf); err != nil {
		panic(err)
	}
	for _, v := range tx.Ins {
		if err := v.ForID(buf); err != nil {
			panic(err)
		}
	}
	if err := VarUInt(len(tx.Outs)).Encode(buf); err != nil {
		panic(err)
	}
	for _, v := range tx.Outs {
		if err := v.Encode(buf); err != nil {
			panic(err)
		}
	}
	if err := tx.Ext.ForID(buf); err != nil {
		panic(err)
	}
	return tx.idhc.Hash(buf.Bytes())
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
func (v *TX) GetFee(bi *BlockIndex, blk *BlockInfo, idx int) (Amount, error) {
	if v.IsCoinBase() {
		return 0, errors.New("coinbase not trans fee")
	}
	fee := Amount(0)
	for _, in := range v.Ins {
		out, _, err := in.LoadTxOut(bi, blk, idx)
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

//检测除coinbase交易外的交易金额
func (v *TX) Check(bi *BlockIndex, blk *BlockInfo, idx int) error {
	if err := v.Ext.Check(); err != nil {
		return err
	}
	if len(v.Ins) == 0 {
		return errors.New("tx ins too slow")
	}
	//这里不检测coinbase交易
	if v.IsCoinBase() {
		return nil
	}
	//总的输入金额
	itv := Amount(0)
	for _, in := range v.Ins {
		err := in.Check(bi)
		if err != nil {
			return err
		}
		out, self, err := in.LoadTxOut(bi, blk, idx)
		if err != nil {
			return err
		}
		if !self && out.IsSpent(in, bi, blk) {
			return errors.New("out is spent")
		}
		itv += out.Value
	}
	if itv == 0 {
		return errors.New("input amount zero error")
	}
	//总的输入出金额
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
	//每个交易的输出不能大于输入
	if itv < 0 || otv < 0 || otv > itv {
		return errors.New("ins amount must >= outs amount")
	}
	//检查签名
	return v.Verify(bi, blk, idx)
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
	if err := v.Ext.Encode(w); err != nil {
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
	if err := v.Ext.Decode(r); err != nil {
		return err
	}
	return nil
}
