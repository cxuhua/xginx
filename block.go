package xginx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	// 最大块大小
	MAX_BLOCK_SIZE = 1024 * 1024 * 4
)

//存储单元索引值
type UvValue struct {
	BlkId  HASH256 //块hash
	UtsIdx VarUInt //units 索引
	UvsIdx VarUInt //units链索引
}

func (v UvValue) GetUnit(chain *BlockIndex) (*Unit, error) {
	blk, err := chain.LoadBlock(v.BlkId)
	if err != nil {
		return nil, err
	}
	uidx := v.UtsIdx.ToInt()
	if uidx < 0 || uidx >= len(blk.Uts) {
		return nil, errors.New("utxidx out of bound")
	}
	units := blk.Uts[uidx]
	vidx := v.UvsIdx.ToInt()
	if vidx < 0 || vidx >= len(*units) {
		return nil, errors.New("uvxidx out of bound")
	}
	return (*units)[vidx], nil
}

func (v UvValue) Bytes() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := v.Encode(buf)
	return buf.Bytes(), err
}

func (v UvValue) Encode(w IWriter) error {
	if err := v.BlkId.Encode(w); err != nil {
		return err
	}
	if err := v.UtsIdx.Encode(w); err != nil {
		return err
	}
	if err := v.UvsIdx.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *UvValue) Decode(r IReader) error {
	if err := v.BlkId.Decode(r); err != nil {
		return err
	}
	if err := v.UtsIdx.Decode(r); err != nil {
		return err
	}
	if err := v.UvsIdx.Decode(r); err != nil {
		return err
	}
	return nil
}

//存储交易索引值
type TxValue struct {
	BlkId  HASH256 //块hash
	TxsIdx VarUInt //txs 索引
}

func (v TxValue) GetTX(chain *BlockIndex) (*TX, error) {
	blk, err := chain.LoadBlock(v.BlkId)
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

//区块头
type BlockHeader struct {
	Ver     uint32  //block ver
	Prev    HASH256 //pre block hash
	TMerkle HASH256 //txs Merkle tree hash
	UMerkle HASH256 //所有记录单元的hash Merkle
	Time    uint32  //时间戳
	Bits    uint32  //难度
	Nonce   uint32  //随机值
	hasher  HashCacher
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
	Uts    []*Units    //记录单元 没有记录单元将不会获得奖励
	Txs    []*TX       //交易记录，类似比特币
	Meta   *TBEle      //指向链数据节点
	utsher HashCacher  //uts 缓存
	merher HashCacher  //mer hash 缓存
}

//断开时回退数据
func (v *BlockInfo) unlink() error {
	return nil
}

func (v BlockInfo) WriteTxsIdx(b *Batch) error {
	for i, tv := range v.Txs {
		//保存交易id对应的区块
		tid := tv.Hash()
		vval := TxValue{
			BlkId:  v.ID(),
			TxsIdx: VarUInt(i),
		}
		vbys, err := vval.Bytes()
		if err != nil {
			return err
		}
		b.Put(TXS_PREFIX, tid[:], vbys)
	}
	return nil
}

func (v BlockInfo) WriteCliBestId(b *Batch) error {
	for _, uts := range v.Uts {
		if len(*uts) < 2 {
			return errors.New("client units num error")
		}
		//保存 cli 最后一个数据单元所在的快
		uid := (*uts)[len(*uts)-1].Hash()
		cid := uts.CliId()
		b.Put(CBI_PREFIX, cid[:], uid[:])
	}
	return nil
}

func (v *BlockInfo) WriteUvsIdx(b *Batch) error {
	for i, uts := range v.Uts {
		if len(*uts) < 2 {
			return errors.New("client units num error")
		}
		for j, uv := range *uts {
			//保存单元id所在的区块
			uid := uv.Hash()
			uval := UvValue{
				BlkId:  v.ID(),
				UtsIdx: VarUInt(i),
				UvsIdx: VarUInt(j),
			}
			ubys, err := uval.Bytes()
			if err != nil {
				return err
			}
			b.Put(UXS_PREFIX, uid[:], ubys)
		}
	}
	return v.WriteCliBestId(b)
}

func (v *BlockInfo) GetUMerkle() (HASH256, error) {
	if h, b := v.utsher.IsSet(); b {
		return h, nil
	}
	ids := []HASH256{}
	for _, uv := range v.Uts {
		ids = append(ids, uv.GetMerkle())
	}
	root := BuildMerkleTree(ids).ExtractRoot()
	v.utsher.SetHash(root)
	return root, nil
}

func (v BlockInfo) GetTMerkle() (HASH256, error) {
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
	merkle, err := v.GetTMerkle()
	if err != nil {
		return err
	}
	v.Header.TMerkle = merkle
	merkle, err = v.GetUMerkle()
	if err != nil {
		return err
	}
	v.Header.UMerkle = merkle
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
		Uts:         VarUInt(len(b.Uts)),
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

//获取分配所得
func (v BlockInfo) GetBaseOuts() (map[HASH160]VarUInt, error) {
	for i, txs := range v.Txs {
		if i == 0 && len(txs.Ins) != 1 {
			return nil, errors.New("base txin miss")
		}
		if i == 0 && !txs.Ins[0].IsBase() {
			return nil, errors.New("base txin type error")
		}
		if i == 0 && len(txs.Outs) > 0 {
			return txs.BaseOuts()
		}
	}
	return nil, errors.New("base outs miss")
}

//检查所有的交易
func (v BlockInfo) CheckTxs() error {
	for _, tx := range v.Txs {
		err := tx.Check()
		if err != nil {
			return err
		}
	}
	return nil
}

//检查所有的单元数据
func (v BlockInfo) CheckUts(chain *BlockIndex) error {
	cmap := map[HASH160]bool{}
	for _, uvs := range v.Uts {
		err := uvs.Check(chain)
		if err != nil {
			return err
		}
		//记录的用户不能重复
		cid := uvs.CliId()
		if _, has := cmap[cid]; has {
			return errors.New("uts client repeat")
		}
		cmap[cid] = true
	}
	return nil
}

func (v BlockInfo) Check(chain *BlockIndex) error {
	if len(v.Txs) == 0 {
		return errors.New("txs miss, too little")
	}
	if len(v.Uts) == 0 {
		return errors.New("uts miss, too little")
	}
	if !CheckProofOfWork(v.ID(), v.Header.Bits) {
		return errors.New("proof of work bits error")
	}
	merkle, err := v.GetTMerkle()
	if err != nil {
		return err
	}
	if !merkle.Equal(v.Header.TMerkle) {
		return errors.New("txs merkle hash error")
	}
	merkle, err = v.GetUMerkle()
	if err != nil {
		return err
	}
	if !merkle.Equal(v.Header.UMerkle) {
		return errors.New("units merkle hash error")
	}
	//检查所有的交易
	if err := v.CheckTxs(); err != nil {
		return err
	}
	//检查所有的数据单元
	if err := v.CheckUts(chain); err != nil {
		return err
	}
	//获取积分分配
	outs, err := v.GetBaseOuts()
	if err != nil {
		return err
	}
	//计算积分分配
	calcer := NewTokenCalcer()
	for _, uv := range v.Uts {
		uc := NewTokenCalcer()
		err := uv.CalcToken(chain, v.Header.Bits, uc)
		if err != nil {
			return err
		}
		calcer.Merge(uc)
	}
	//检验分配是否正确
	for ck, cv := range calcer.Outs() {
		if outs[ck] != cv {
			return errors.New("token alloc error")
		}
	}
	return nil
}

func (v BlockHeader) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, v.Ver); err != nil {
		return err
	}
	if err := v.Prev.Encode(w); err != nil {
		return err
	}
	if err := v.TMerkle.Encode(w); err != nil {
		return err
	}
	if err := v.UMerkle.Encode(w); err != nil {
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

func (v BlockInfo) Encode(w IWriter) error {
	if err := v.Header.Encode(w); err != nil {
		return err
	}
	if err := VarUInt(len(v.Uts)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Uts {
		if err := v.Encode(w); err != nil {
			return err
		}
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
	if err := v.TMerkle.Decode(r); err != nil {
		return err
	}
	if err := v.UMerkle.Decode(r); err != nil {
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
	unum := VarUInt(0)
	if err := unum.Decode(r); err != nil {
		return err
	}
	v.Uts = make([]*Units, unum)
	for i, _ := range v.Uts {
		uvs := &Units{}
		if err := uvs.Decode(r); err != nil {
			return err
		}
		v.Uts[i] = uvs
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

func (v TxIn) Check() error {
	if v.IsBase() {
		if v.Script.Len() > 128 || v.Script.Len() < 4 {
			return errors.New("base script len error")
		}
		if !v.Script.IsBaseScript() {
			return errors.New("base script type error")
		}
	} else if v.Script.IsStdUnlockScript() {

	} else if v.Script.IsAucUnlockScript() {

	} else {
		return errors.New("txin unlock script type error")
	}
	return nil
}

func (v TxIn) Encode(w IWriter) error {
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

//是否基本单元，txs的第一个一定是base，输出为奖励计算的距离
//所得奖励的70%给区块矿工，30%按标签贡献分给标签所有者
//涉及两个标签，两标签平分
//所有奖励向下取整
func (in TxIn) IsBase() bool {
	return in.OutHash.IsZero() && in.OutIndex == 0
}

//交易输出
type TxOut struct {
	Value  VarUInt //距离奖励 GetRewardRate 计算比例，所有输出之和不能高于总奖励
	Script Script  //锁定脚本
}

//获取竞价脚本
func (v TxOut) ToAuctionScript() (*AucLockScript, error) {
	typ := v.Script.Type()
	//其他类型可消费
	if typ != SCRIPT_AUCLOCK_TYPE {
		return nil, errors.New("type error")
	}
	if v.Script.Len() > 256 {
		return nil, errors.New("script length too long")
	}
	return v.Script.ToAuction()
}

//获取区块中所有指定类型的拍卖输出
func (b *BlockInfo) FindAucScript(obj ObjectId) []*AucLockScript {
	ass := []*AucLockScript{}
	//获取区块中所有和obj相关的竞价输出
	for _, tx := range b.Txs {
		for _, out := range tx.Outs {
			as, err := out.ToAuctionScript()
			if err != nil {
				continue
			}
			if !as.ObjId.Equal(obj) {
				continue
			}
			ass = append(ass, as)
		}
	}
	return ass
}

func (v TxOut) Check() error {
	return nil
}

func (v TxOut) Encode(w IWriter) error {
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

func (tx TX) BaseOuts() (map[HASH160]VarUInt, error) {
	outs := map[HASH160]VarUInt{}
	for _, v := range tx.Outs {
		if !v.Script.IsStdLockedcript() {
			return nil, errors.New("base tx out script error")
		}
		outs[v.Script.StdLockedHash()] = v.Value
	}
	return outs, nil
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

func (v TX) Check() error {
	if len(v.Ins) == 0 {
		return errors.New("tx ins too slow")
	}
	if len(v.Outs) == 0 {
		return errors.New("tx outs too slow")
	}
	for _, v := range v.Ins {
		err := v.Check()
		if err != nil {
			return err
		}
		//校验签名
	}
	for _, v := range v.Outs {
		err := v.Check()
		if err != nil {
			return err
		}
	}
	return nil
}

func (v TX) Encode(w IWriter) error {
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

type Units []*Unit

func (v Units) CliId() HASH160 {
	return v[0].CPks.Hash()
}

func (v Units) GetMerkle() HASH256 {
	ids := []HASH256{}
	for _, uv := range v {
		ids = append(ids, uv.Hash())
	}
	return BuildMerkleTree(ids).ExtractRoot()
}

func (v Units) Encode(w IWriter) error {
	if err := VarUInt(len(v)).Encode(w); err != nil {
		return err
	}
	for _, uv := range v {
		err := uv.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Units) Last() *Unit {
	if len(*v) == 0 {
		return nil
	}
	return (*v)[len(*v)-1]
}

func (v *Units) Add(uv *Unit) error {
	if err := uv.Check(); err != nil {
		return err
	}
	if uv.IsFirst() {
		*v = append(*v, uv)
		return nil
	}
	last := v.Last()
	if last == nil {
		return errors.New("last unit miss")
	}
	if !uv.Prev.Equal(last.Hash()) {
		return errors.New("hash not consecutive")
	}
	*v = append(*v, uv)
	return nil
}

func (v *Units) Decode(r IReader) error {
	num := VarUInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	*v = make([]*Unit, num)
	for i, _ := range *v {
		un := &Unit{}
		err := un.Decode(r)
		if err != nil {
			return err
		}
		(*v)[i] = un
	}
	return nil
}

//查找用户cpk的链
func (b BlockInfo) FindUnits(cpk PKBytes) *Units {
	for _, uts := range b.Uts {
		if len(*uts) > 0 && (*uts)[0].CPks.Equal(cpk) {
			return uts
		}
	}
	return nil
}

//获取上一个unit
func (v *Units) LastUnit(chain *BlockIndex) (*Unit, error) {
	lid, err := chain.LoadCliLastUnit(v.CliId())
	//client不存在last unit的清空下，如果是第一个直接返回
	if len(*v) > 0 && (*v)[0].IsFirst() && err != nil {
		return (*v)[0], nil
	}
	if err != nil {
		return nil, err
	}
	//第一个必须指向最后一个
	if !lid.Equal((*v)[0].Prev) {
		return nil, errors.New("first must is last unit id")
	}
	if len(*v) == 0 {
		return nil, errors.New("units empty")
	}
	//加载最后一个单元数据
	uv, err := chain.LoadUvValue(lid)
	if err != nil {
		return nil, err
	}
	return uv.GetUnit(chain)
}

//是否是连续的
func (v *Units) IsConsecutive() bool {
	var pv *Unit = nil
	for i, uv := range *v {
		if i == 0 {
			pv = uv
			continue
		}
		if !uv.Prev.Equal(pv.Hash()) {
			return false
		}
		pv = uv
	}
	return true
}

func (v *Units) Check(chain *BlockIndex) error {
	if len(*v) < 2 {
		return errors.New("unit too little")
	}
	if !v.IsConsecutive() {
		return errors.New("unit not continuous")
	}
	prev, err := v.LastUnit(chain)
	if err != nil {
		return err
	}
	//如果不是第一个，prev必须指向最后一个
	first := (*v)[0]
	if !prev.Equal(*first) && !first.Prev.Equal(prev.Hash()) {
		return errors.New("unit not continuous")
	}
	for _, uv := range *v {
		if err := uv.Check(); err != nil {
			return err
		}
	}
	return nil
}

//计算积分
func (v *Units) CalcToken(chain *BlockIndex, bits uint32, calcer ITokenCalcer) error {
	if len(*v) < 2 {
		return errors.New("Unit too small ")
	}
	//获取上一个参与计算
	prev, err := v.LastUnit(chain)
	if err != nil {
		return err
	}
	is := &Units{}
	if !prev.Equal(*(*v)[0]) {
		*is = append(*is, prev)
	}
	*is = append(*is, *v...)
	return calcer.Calc(bits, is)
}

type Alloc uint8

func (v Alloc) ToUInt8() uint8 {
	return uint8(v)
}

func (v Alloc) Encode(w IWriter) error {
	return binary.Write(w, Endian, v)
}

func (v *Alloc) Decode(r IReader) error {
	return binary.Read(r, Endian, &v)
}

//矿工，标签，用户，获得积分比例
func (v Alloc) Scale() (float64, float64, float64) {
	m := float64((v >> 5) & 0b111)
	t := float64((v >> 2) & 0b111)
	c := float64(v & 0b11)
	return m / 10.0, t / 10.0, c / 10.0
}

//3个值之和应该为10
func (v Alloc) Check() error {
	av := ((v >> 5) & 0b111) + ((v >> 2) & 0b111) + (v & 0b11)
	if av != 10 {
		return errors.New("value error,alloc sum != 10")
	}
	return nil
}

const (
	S631 = 0b110_011_01
	S622 = 0b110_010_10
	S640 = 0b110_100_00
	S550 = 0b101_101_00
	S721 = 0b111_010_01
)

//token结算接口
type ITokenCalcer interface {
	Calc(bits uint32, items *Units) error
	//总的积分
	Total() VarUInt
	//标签获得的积分
	Outs() map[HASH160]VarUInt
	//重置
	Reset()
	//合并
	Merge(c *TokenCalcer)
}

type TokenCalcer struct {
	total float64             //总的的积分
	vmap  map[HASH160]float64 //标签获得的积分
}

func NewTokenCalcer() *TokenCalcer {
	return &TokenCalcer{
		total: 0,
		vmap:  map[HASH160]float64{},
	}
}

func (calcer TokenCalcer) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Total=%d\n", calcer.Total()))
	for k, v := range calcer.Outs() {
		sb.WriteString(fmt.Sprintf("Tag=%s Value=%d\n", hex.EncodeToString(k[:]), v))
	}
	return sb.String()
}

func (calcer *TokenCalcer) Reset() {
	calcer.total = 0
	calcer.vmap = map[HASH160]float64{}
}

func (calcer *TokenCalcer) Total() VarUInt {
	return VarUInt(calcer.total)
}

//标签获得的积分
func (calcer *TokenCalcer) Outs() map[HASH160]VarUInt {
	ret := map[HASH160]VarUInt{}
	for k, v := range calcer.vmap {
		ret[k] += VarUInt(v)
	}
	return ret
}

func (calcer *TokenCalcer) Merge(c *TokenCalcer) {
	calcer.total += c.total
	for k, v := range c.vmap {
		calcer.vmap[k] += v
	}
}

//多个连续的记录信息，记录client链,至少有两个记录
//两个点之间的服务器时间差超过1天将忽略距离 SpanTime(秒）设置
//定位点与标签点差距超过1km，距离递减 GetDisRate 计算
//以上都不影响链的链接，只是会减少距离提成
//标签距离合计，后一个经纬度与前一个距离之和 单位：米,如果有prevhash需要计算第一个与prevhash指定的最后一个单元距离
//所有distance之和就是clientid的总的distance
//bits 区块难度
func (calcer *TokenCalcer) Calc(bits uint32, items *Units) error {
	if len(*items) < 2 {
		return errors.New("items count error")
	}
	if !CheckProofOfWorkBits(bits) {
		return errors.New("proof of work bits error")
	}
	mph := conf.minerpk.Hash()
	calcer.Reset()
	tpv := CalculateWorkTimeScale(bits)
	for i := 1; i < len(*items); i++ {
		cv := (*items)[i+0]
		//使用当前标签设定的分配比例
		if err := cv.TASV.Check(); err != nil {
			return fmt.Errorf("item asv error %w", err)
		}
		mr, tr, cr := cv.TASV.Scale()
		pv := (*items)[i-1]
		if !cv.ClientID().Equal(pv.ClientID()) {
			return errors.New("client error")
		}
		if cv.IsFirst() {
			return errors.New("curr point error")
		}
		//记录时间差太多忽略这个点
		if cv.TimeSub() > conf.TimeErr {
			continue
		}
		if !cv.Prev.Equal(pv.Hash()) {
			return errors.New("prev hash error")
		}
		//两次记录时间必须连续 st=两次时间间隔，单位：秒
		st := pv.STimeSub(cv)
		if st < 0 {
			return errors.New("stime error")
		}
		//两次记录时间差太大将被忽略,根据当前区块难度放宽
		if st > conf.SpanTime*tpv {
			continue
		}
		//忽略超人的存在，速度太快
		sp := pv.TTSpeed(cv)
		if sp < 0 || sp > conf.MaxSpeed {
			continue
		}
		dis := float64(0)
		//如果两次都是同一打卡点，按时间获得积分
		if cv.TUID.Equal(pv.TUID) {
			//按每小时1km速度结算
			dis = st / 3.6
		} else {
			//获取定位不准惩罚系数
			csr := cv.CTLocDisRate()
			//上一点的定位差
			psr := pv.CTLocDisRate()
			//计算距离奖励 rr为递减
			dis = pv.TTLocDis(cv) * csr * psr
		}
		//所有和不能超过总量
		calcer.total += dis
		//矿工获得
		mdis := dis * mr
		//标签所有者获得,两标签平分
		tdis := (dis * tr) * 0.5
		calcer.vmap[cv.TPKH] += tdis
		calcer.vmap[pv.TPKH] += tdis
		cdis := dis * cr
		calcer.vmap[cv.ClientID()] += cdis
		//保存矿工获得的总量
		calcer.vmap[mph] += mdis
	}
	if calcer.total < 0 || calcer.total > EARTH_RADIUS {
		return errors.New("total range error")
	}
	return nil
}
