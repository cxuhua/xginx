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
	"time"
)

func CreateGenesisBlock(db DBImp, wg *sync.WaitGroup, ctx context.Context, cancel context.CancelFunc) {
	defer wg.Done()
	pub, err := LoadPublicKey("8aKby6XxwmoaiYt6gUbS1u2RHco37iHfh6sAPstME33Qh6ujd9")
	if err != nil {
		panic(err)
	}

	tv := uint32(time.Now().Unix())

	b := &BlockInfo{
		Ver:    1,
		Prev:   HASH256{},
		Merkle: HASH256{},
		Time:   tv,
		Bits:   0x1d00ffff,
		Nonce:  0x58f3e185,
		Uts:    []*Units{},
		Txs:    []*TX{},
	}

	tx := &TX{}
	tx.Ver = 1

	in := &TxIn{}
	in.Script = BaseScript(0, []byte("The value of a man should be seen in what he gives and not in what he is able to receive."))
	tx.Ins = []*TxIn{in}

	us := &Units{}
	u0 := &Unit{}
	u1 := &Unit{}
	_ = us.Add(db, u0)
	_ = us.Add(db, u1)

	b.Uts = []*Units{us}

	out := &TxOut{}
	out.Value = 529
	out.Script = LockedScript(pub)
	tx.Outs = []*TxOut{out}

	tx.Hash()
	b.Txs = []*TX{tx}

	//生成merkle root id
	if err := b.SetMerkle(); err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}
	SetRandInt(&b.Nonce)
	err = b.EncodeHeader(buf)
	if err != nil {
		panic(err)
	}
	heaerbytes := buf.Bytes()
	nhash := HASH256{}
	for i := uint64(b.Nonce); ; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			//写入数字
			Endian.PutUint32(heaerbytes[len(heaerbytes)-4:], b.Nonce)
			//计算hash
			copy(nhash[:], Hash256(heaerbytes))
			//检测hash
			if CheckProofOfWork(nhash, b.Bits) {
				log.Printf("%x %v %x\n", b.Nonce, nhash, b.Time)
				cancel()
				break
			}
			if i%100000 == 0 {
				//重新计算时间
				log.Println(i, nhash, b.Nonce)
				b.Time = uint32(time.Now().Unix())
				Endian.PutUint32(heaerbytes[len(heaerbytes)-12:], b.Time)
				continue
			}
			b.Nonce = uint32(i)
		}
	}
}

//一个记录单元必须同一个用户连续的链数据
//块信息
//Bodys记录中不能用相同的clientid，items必须时间上连续，hash能前后衔接
//txs交易部分和比特币类似
//块大小限制为4M大小
type BlockInfo struct {
	Ver    uint32     //block ver
	Prev   HASH256    //pre block hash
	Merkle HASH256    //txs Merkle tree hash + Units hash
	Time   uint32     //时间戳
	Bits   uint32     //难度
	Nonce  uint32     //随机值
	Uts    []*Units   //记录单元 没有记录单元将不会获得奖励
	Txs    []*TX      //交易记录，类似比特币
	hasher HashCacher //hash 缓存
	utsher HashCacher //uts 缓存
	merher HashCacher //mer hash 缓存
}

func (v *BlockInfo) UTSHash() HASH256 {
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

func (v *BlockInfo) SetMerkle() error {
	if _, b := v.merher.IsSet(); b {
		return nil
	}
	ids := []HASH256{}
	for _, tv := range v.Txs {
		ids = append(ids, tv.Hash())
	}
	root, _, _ := BuildMerkleTree(ids).Extract()
	if root.IsZero() {
		return errors.New("merkle root error")
	}
	//root + utshash = merkle hash
	buf := &bytes.Buffer{}
	if _, err := buf.Write(root[:]); err != nil {
		return err
	}
	uts := v.UTSHash()
	if _, err := buf.Write(uts[:]); err != nil {
		return err
	}
	v.Merkle = v.merher.Hash(buf.Bytes())
	return nil
}

func (b *BlockInfo) Hash() HASH256 {
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

func UseIdLoadBlock(db DBImp, id HASH256, ft string) (*BlockInfo, error) {
	bid, err := db.BlockId(id, ft)
	if err != nil {
		return nil, fmt.Errorf("use id get block id error %w", err)
	}
	return LoadBlock(db, bid)
}

//加载区块
func LoadBlock(db DBImp, id HASH256) (*BlockInfo, error) {
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

//HASH256 meta,bytes
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
	hash := Hash256(buf.Bytes())
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
		uvs := &Units{}
		err := uvs.Decode(r)
		if err != nil {
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
	OutHash  HASH256 //输出交易hash
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

func (tx TX) Save(db DBImp) error {
	v := &TTx{}
	v.Ver = uint32(tx.Ver)
	v.Ins = make([]*TTxIn, len(tx.Ins))
	for i, iv := range tx.Ins {
		tin := &TTxIn{}
		tin.Script = iv.Script[:]
		tin.OutHash = iv.OutHash[:]
		tin.OutIndex = uint32(iv.OutIndex)
		v.Ins[i] = tin
	}
	v.Outs = make([]*TTxOut, len(tx.Outs))
	for i, ov := range tx.Outs {
		tout := &TTxOut{}
		tout.Value = uint64(ov.Value)
		tout.Script = ov.Script[:]
		v.Outs[i] = tout
	}
	v.Hash = tx.Hash().Bytes()
	return db.SetTX(v.Hash, v)
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

func (v *Units) Add(db DBImp, uv *Unit) error {
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
func (v *Units) GetPrev(db DBImp) (*Unit, error) {
	//如果有数据并且是第一个数据单元直接返回
	if len(*v) > 0 && (*v)[0].IsFirst() {
		return (*v)[0], nil
	}
	if len(*v) == 0 {
		return nil, errors.New("units empty")
	}
	//获取第一个所在的区块id
	uid := (*v)[0].Hash()
	//获取区块
	block, err := UseIdLoadBlock(db, uid, USE_UIDS)
	if err != nil {
		return nil, fmt.Errorf("block info miss %w", err)
	}
	uts := block.FindUnits((*v)[0].CPks)
	if uts == nil {
		return nil, errors.New("cli units miss")
	}
	last := uts.Last()
	if last == nil {
		return nil, errors.New("cli last unit miss")
	}
	return last, nil
}

//计算积分
func (v *Units) CalcToken(db DBImp, calcer ITokenCalcer) error {
	if len(*v) < 2 {
		return errors.New("Unit too small ")
	}
	//获取上一个参与计算
	prev, err := v.GetPrev(db)
	if err != nil {
		return err
	}
	is := []*Unit{}
	if !prev.Eqial(*(*v)[0]) {
		is = append(is, prev)
	}
	is = append(is, *v...)
	return calcer.Calc(is)
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
	Calc(items []*Unit) error
	//总的积分
	Total() VarUInt
	//矿工获得的积分
	Miner() VarUInt
	//用户获得的积分
	Client() VarUInt
	//标签获得的积分
	Tags() map[HASH160]VarUInt
	//重置
	Reset()
}

type TokenCalcer struct {
	total  float64             //总的的积分
	vmap   map[HASH160]float64 //标签获得的积分
	miner  float64             //矿工奖励的积分
	client float64             //用户获得的积分
}

func NewTokenCalcer() *TokenCalcer {
	return &TokenCalcer{
		total:  0,
		vmap:   map[HASH160]float64{},
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
	calcer.vmap = map[HASH160]float64{}
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
func (calcer *TokenCalcer) Tags() map[HASH160]VarUInt {
	ret := map[HASH160]VarUInt{}
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
//bits 区块难度
func (calcer *TokenCalcer) Calc(bits uint32, items []*Unit) error {
	if len(items) < 2 {
		return errors.New("items count error")
	}
	if !CheckProofOfWorkBits(bits) {
		return errors.New("proof of work bits error")
	}
	calcer.Reset()
	tpv := CalculateWorkTimeScale(bits)
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
			dis = st / 3600.0
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
		calcer.client += cdis
		//保存矿工获得的总量
		calcer.miner += mdis
	}
	if calcer.total < 0 || calcer.total > EARTH_RADIUS {
		return errors.New("total range error")
	}
	return nil
}
