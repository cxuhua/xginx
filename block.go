package xginx

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
)

func CreateGenesisBlock(wg *sync.WaitGroup, ctx context.Context, cancel context.CancelFunc) {
	defer wg.Done()
	pub, err := LoadPublicKey("8aKby6XxwmoaiYt6gUbS1u2RHco37iHfh6sAPstME33Qh6ujd9")
	if err != nil {
		panic(err)
	}

	b := &BlockInfo{
		Ver:    1,
		Prev:   HashID{},
		Merkle: HashID{},
		Time:   1572669878,
		Bits:   0x1d00ffff,
		Uts:    []*Units{},
		Txs:    []*TX{},
	}

	tx := &TX{}
	tx.Ver = 1

	in := &TxIn{}
	in.Script = BaseScript([]byte("The value of a man should be seen in what he gives and not in what he is able to receive."))
	tx.Ins = []*TxIn{in}

	out := &TxOut{}
	out.Value = 529
	out.Script = LockedScript(pub)
	tx.Outs = []*TxOut{out}

	tx.Hash()
	b.Txs = []*TX{tx}

	//生成merkle root id
	if err := b.MerkleRoot(); err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}
	SetRandInt(&b.Nonce)
	err = b.EncodeHeader(buf)
	if err != nil {
		panic(err)
	}
	bb := buf.Bytes()
	nhash := HashID{}
	for i := uint64(0); ; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			Endian.PutUint32(bb[len(bb)-4:], b.Nonce)
			copy(nhash[:], HASH256(bb))
			if CheckProofOfWork(nhash, b.Bits) {
				log.Printf("%x %v\n", b.Nonce, nhash)
				cancel()
				break
			}
			if i%100000 == 0 {
				SetRandInt(&b.Nonce)
				log.Println(i, nhash, b.Nonce)
				continue
			}
			b.Nonce++
		}
	}
}

//一个块由n个条目组成

//块信息
//Bodys记录中不能用相同的clientid，items必须时间上连续，hash能前后衔接
//txs交易部分和比特币类似
//块大小限制为4M大小
type BlockInfo struct {
	Ver    uint32     //block ver
	Prev   HashID     //pre block hash
	Merkle HashID     //txs Merkle tree hash + Units hash
	Time   uint32     //时间戳
	Bits   uint32     //难度
	Nonce  uint32     //随机值
	Uts    []*Units   //记录单元 没有记录单元将不会获得奖励
	Txs    []*TX      //交易记录，类似比特币
	hasher HashCacher //hash 缓存
	utsher HashCacher //uts 缓存
	merher HashCacher //mer hash 缓存
}

func (v *BlockInfo) UtsHash() HashID {
	if h, b := v.utsher.IsSet(); b {
		return h
	}
	buf := &bytes.Buffer{}
	if err := VarUInt(len(v.Uts)).Encode(buf); err != nil {
		panic(err)
	}
	for _, uv := range v.Uts {
		err := uv.Encode(buf)
		if err != nil {
			panic(err)
		}
	}
	return v.utsher.Hash(buf.Bytes())
}

func (v *BlockInfo) MerkleRoot() error {
	if _, b := v.merher.IsSet(); b {
		return nil
	}
	ids := []HashID{}
	for _, tv := range v.Txs {
		ids = append(ids, tv.Hash())
	}
	root, _, _ := BuildMerkleTree(ids).Extract()
	if root.IsZero() {
		return errors.New("merkle root error")
	}
	buf := &bytes.Buffer{}
	if _, err := buf.Write(root[:]); err != nil {
		return err
	}
	uts := v.UtsHash()
	if _, err := buf.Write(uts[:]); err != nil {
		return err
	}
	v.Merkle = v.merher.Hash(buf.Bytes())
	return nil
}

func (b *BlockInfo) Hash() HashID {
	if id, ok := b.hasher.IsSet(); ok {
		return id
	}
	buf := &bytes.Buffer{}
	if err := b.EncodeHeader(buf); err != nil {
		panic(err)
	}
	return b.hasher.Hash(buf.Bytes())
}

func (b *BlockInfo) IsGenesis() bool {
	return b.Prev.IsZero() && conf.IsGenesisId(b.Hash())
}

//加载区块
func LoadBlock(db DBImp, id HashID) (*BlockInfo, error) {
	if b, err := CBlock.Get(id[:]); err == nil {
		return b.(*BlockInfo), nil
	}
	v := &BlockInfo{}
	if err := db.GetBlock(id[:], v); err != nil {
		return nil, err
	}
	if b, err := CBlock.Set(id[:], v); err == nil {
		return b.(*BlockInfo), nil
	} else {
		log.Println("WARN", "cblock set cache error", err)
	}
	return v, nil
}

//hashid meta,bytes
func (b *BlockInfo) ToTBMeta() ([]byte, *TBMeta, []byte, error) {
	meta := &TBMeta{
		Ver:    b.Ver,
		Prev:   b.Prev[:],
		Merkle: b.Merkle[:],
		Time:   b.Time,
		Bits:   b.Bits,
		Nonce:  b.Nonce,
		Uts:    uint32(len(b.Uts)),
		Txs:    uint32(len(b.Txs)),
	}
	buf := &bytes.Buffer{}
	if err := b.Encode(buf); err != nil {
		return nil, nil, nil, err
	}
	hash := HASH256(buf.Bytes())
	return hash, meta, buf.Bytes(), nil
}

func (v *BlockInfo) Check(db DBImp) error {
	return nil
}

func (v BlockInfo) EncodeHeader(w IWriter) error {
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

func (v BlockInfo) Encode(w IWriter) error {
	if err := v.EncodeHeader(w); err != nil {
		return err
	}
	if err := VarUInt(len(v.Uts)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Uts {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	if err := VarUInt(len(v.Txs)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Txs {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockInfo) Decode(r IReader) error {
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
	unum := VarUInt(0)
	if err := unum.Decode(r); err != nil {
		return err
	}
	v.Uts = make([]*Units, unum)
	for i, _ := range v.Uts {
		uv := &Units{}
		err := uv.Decode(r)
		if err != nil {
			return err
		}
		v.Uts[i] = uv
	}
	tnum := VarUInt(0)
	if err := tnum.Decode(r); err != nil {
		return err
	}
	v.Txs = make([]*TX, tnum)
	for i, _ := range v.Txs {
		tx := &TX{}
		err := tx.Decode(r)
		if err != nil {
			return err
		}
		v.Txs[i] = tx
	}
	return nil
}

//交易输入
type TxIn struct {
	OutHash  HashID  //输出交易hash
	OutIndex VarUInt //对应的输出索引
	Script   Script  //解锁脚本
}

func (v TxIn) Check() error {
	if v.IsBase() {
		if v.Script.Len() > 100 || v.Script.Len() < 4 {
			return errors.New("base script len error")
		}
		if !v.Script.IsBaseScript() {
			return errors.New("base script type error")
		}
	} else {
		if !v.Script.IsUnlockScript() {
			return errors.New("txin unlock script type error")
		}
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

func (v TxOut) Check() error {
	if !v.Script.IsLockedcript() {
		return errors.New("txout script type error")
	}
	if v.Value > 100000000 {
		return errors.New("txout value too big")
	}
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

func (tx *TX) Hash() HashID {
	if hash, ok := tx.hasher.IsSet(); ok {
		return hash
	}
	h := HashID{}
	buf := &bytes.Buffer{}
	_ = tx.Encode(buf)
	copy(h[:], HASH256(buf.Bytes()))
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

type Alloc uint8

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
	m := (v >> 5) & 0b111
	t := (v >> 2) & 0b111
	c := v & 0b11
	av := m + t + c
	if av != 10 {
		return errors.New("value error,scale sum=10")
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

//条目
type Units struct {
	ClientID  UserID       //用户公钥的hash160
	PrevBlock HashID       //上个块hash
	PrevIndex VarUInt      //上个块所在Units索引
	Items     []*Unit      //
	calcer    ITokenCalcer //check 成功后积分存在这里
	hasher    HashCacher
}

//token结算接口
type ITokenCalcer interface {
	Calc(items []*Unit) error
	//总的积分
	Total() VarUInt
	//矿工获得的积分
	Miner() VarUInt
	//用户获得的积分
	Client() VarUInt
	//标签获得的积分
	Tags() map[UserID]VarUInt
	//重置
	Reset()
}

type TokenCalcer struct {
	total  float64            //总的的积分
	vmap   map[UserID]float64 //标签获得的积分
	miner  float64            //矿工奖励的积分
	client float64            //用户获得的积分
}

func NewTokenCalcer() *TokenCalcer {
	return &TokenCalcer{
		total:  0,
		vmap:   map[UserID]float64{},
		miner:  0,
		client: 0,
	}
}

func (calcer TokenCalcer) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Total=%d\n", calcer.Total()))
	sb.WriteString(fmt.Sprintf("Miner=%d\n", calcer.Miner()))
	sb.WriteString(fmt.Sprintf("Client=%d\n", calcer.Client()))
	for k, v := range calcer.Tags() {
		sb.WriteString(fmt.Sprintf("Tag=%s Value=%d\n", hex.EncodeToString(k[:]), v))
	}
	return sb.String()
}

func (calcer *TokenCalcer) Reset() {
	calcer.total = 0
	calcer.miner = 0
	calcer.client = 0
	calcer.vmap = map[UserID]float64{}
}

func (calcer *TokenCalcer) Total() VarUInt {
	return VarUInt(calcer.total)
}

//矿工获得的积分
func (calcer *TokenCalcer) Miner() VarUInt {
	return VarUInt(calcer.miner)
}

//用户获得的积分
func (calcer *TokenCalcer) Client() VarUInt {
	return VarUInt(calcer.client)
}

//标签获得的积分
func (calcer *TokenCalcer) Tags() map[UserID]VarUInt {
	ret := map[UserID]VarUInt{}
	for k, v := range calcer.vmap {
		ret[k] = VarUInt(v)
	}
	return ret
}

//多个连续的记录信息，记录client链,至少有两个记录
//两个点之间的服务器时间差超过1天将忽略距离 SpanTime(秒）设置
//定位点与标签点差距超过1km，距离递减 GetDisRate 计算
//以上都不影响链的链接，只是会减少距离提成
//标签距离合计，后一个经纬度与前一个距离之和 单位：米,如果有prevhash需要计算第一个与prevhash指定的最后一个单元距离
//所有distance之和就是clientid的总的distance
func (calcer *TokenCalcer) Calc(items []*Unit) error {
	//检测分配比例
	calcer.Reset()
	if len(items) < 2 {
		return errors.New("items count error")
	}
	for i := 1; i < len(items); i++ {
		cv := items[i+0]
		//使用当前标签设定的分配比例
		if err := cv.TASV.Check(); err != nil {
			return fmt.Errorf("item asv error %w", err)
		}
		mr, tr, cr := cv.TASV.Scale()
		pv := items[i-1]
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
		//两次记录时间必须连续
		st := pv.STimeSub(cv)
		if st < 0 {
			return errors.New("stime error")
		}
		//两次记录时间差太大将被忽略
		if st > conf.SpanTime {
			continue
		}
		//获取当前点定位差
		csr := cv.CTLocDisRate()
		//上一点的定位差
		psr := pv.CTLocDisRate()
		//计算距离奖励
		dis := pv.TTLocDis(cv) * csr * psr
		//所有和不能超过总量
		calcer.total += dis
		//矿工获得
		mdis := dis * mr
		//标签所有者获得,两标签平分
		tdis := (dis * tr) * 0.5
		calcer.vmap[cv.TPKH] += tdis
		calcer.vmap[pv.TPKH] += tdis
		cdis := dis * cr
		calcer.client += cdis
		//保存矿工获得的总量
		calcer.miner += mdis
	}
	if calcer.total < 0 || calcer.total > EARTH_RADIUS {
		return errors.New("total range error")
	}
	return nil
}

//查找上一个单元 根据:
//PrevBlock HashID  //上个块hash
//PrevIndex VarUInt //上个块所在Units索引
func (v *Units) findPrevUnit(db DBImp) (*Unit, error) {
	if v.PrevBlock.IsZero() && !v.Items[0].IsFirst() {
		return nil, errors.New("prev block hash zero")
	}
	//Items[0]为第一个元素
	if v.PrevBlock.IsZero() && v.Items[0].IsFirst() {
		return nil, nil
	}
	pb, err := LoadBlock(db, v.PrevBlock)
	if err != nil {
		return nil, err
	}
	idx := v.PrevIndex.ToInt()
	if idx < 0 || idx >= len(pb.Uts) {
		return nil, errors.New("prev index error")
	}
	uts := pb.Uts[idx]
	//必须是当前client的
	if !uts.ClientID.Equal(v.ClientID) {
		return nil, errors.New("block unit error")
	}
	if len(uts.Items) == 0 {
		return nil, errors.New("prv block units empty")
	}
	return uts.Last(), nil
}

func (v *Units) Hash() HashID {
	if h, b := v.hasher.IsSet(); b {
		return h
	}
	buf := &bytes.Buffer{}
	if err := v.Encode(buf); err != nil {
		panic(err)
	}
	return v.hasher.Hash(buf.Bytes())
}

func (v *Units) Last() *Unit {
	return v.Items[len(v.Items)-1]
}

func (v *Units) Check(db DBImp) error {
	//检测上一个has
	if len(v.Items) < 2 {
		return errors.New("items count too slow")
	}
	items := make([]*Unit, 0)
	//查询上一个
	puv, err := v.findPrevUnit(db)
	if err != nil {
		return err
	}
	//存在加入首位
	if puv != nil {
		items = append(items, puv)
	}
	buf := &bytes.Buffer{}
	items = append(items, v.Items...)
	for _, uv := range items {
		//items必须是一个用户的数据
		if !v.ClientID.Equal(uv.ClientID()) {
			return errors.New("client id error")
		}
		buf.Reset()
		err := uv.Encode(buf)
		if err != nil {
			return err
		}
		err = uv.SerPart.Verify(conf, buf.Bytes())
		if err != nil {
			return err
		}
	}
	v.calcer = NewTokenCalcer()
	if err := v.calcer.Calc(items); err != nil {
		return err
	}
	return nil
}

func (v Units) Encode(w IWriter) error {
	if err := v.ClientID.Encode(w); err != nil {
		return err
	}
	if err := v.PrevBlock.Encode(w); err != nil {
		return err
	}
	if err := v.PrevIndex.Encode(w); err != nil {
		return err
	}
	if err := VarUInt(len(v.Items)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Items {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Units) Decode(r IReader) error {
	if err := v.ClientID.Decode(r); err != nil {
		return err
	}
	if err := v.PrevBlock.Decode(r); err != nil {
		return err
	}
	if err := v.PrevIndex.Decode(r); err != nil {
		return err
	}
	inum := VarUInt(0)
	if err := inum.Decode(r); err != nil {
		return err
	}
	v.Items = make([]*Unit, inum)
	for i, _ := range v.Items {
		uv := &Unit{}
		err := uv.Decode(r)
		if err != nil {
			return err
		}
		v.Items[i] = uv
	}
	return nil
}
