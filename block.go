package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
)

//一个块由n个条目组成

//块信息
//Bodys记录中不能用相同的clientid，items必须时间上连续，hash能前后衔接
//txs交易部分和比特币类似
//块大小限制为4M大小
type BlockInfo struct {
	Ver    uint32 //block ver
	Prev   HashID //pre block hash
	Merkle HashID //txs Merkle tree hash + Units hash
	Time   uint32 //时间戳
	Bits   uint32 //难度
	Nonce  uint32 //随机值
	Units  []Unit //记录单元 没有记录单元将不会获得奖励
	Txs    []TX   //交易记录，类似比特币
}

func (v *BlockInfo) Check() error {
	return nil
}

func (v BlockInfo) Encode(w IWriter) error {
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
	if err := VarUInt(len(v.Units)).Encode(w); err != nil {
		return err
	}
	for _, v := range v.Units {
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
	v.Units = make([]Unit, unum)
	for i, _ := range v.Units {
		err := v.Units[i].Decode(r)
		if err != nil {
			return err
		}
	}
	tnum := VarUInt(0)
	if err := tnum.Decode(r); err != nil {
		return err
	}
	v.Txs = make([]TX, tnum)
	for i, _ := range v.Txs {
		err := v.Txs[i].Decode(r)
		if err != nil {
			return err
		}
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
	Ver  VarUInt //版本
	Ins  []TxIn  //输入
	Outs []TxOut //输出
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
	v.Ins = make([]TxIn, inum)
	for i, _ := range v.Ins {
		err := v.Ins[i].Decode(r)
		if err != nil {
			return err
		}
	}
	onum := VarUInt(0)
	if err := onum.Decode(r); err != nil {
		return err
	}
	v.Outs = make([]TxOut, onum)
	for i, _ := range v.Outs {
		err := v.Outs[i].Decode(r)
		if err != nil {
			return err
		}
	}
	return nil
}

//条目
type Unit struct {
	ClientID  UserID      //用户公钥的hash160
	PrevHash  HashID      //上个块hash
	PrevIndex VarUInt     //上个块所在Units索引
	Items     []UnitBlock //
	Distance  VarUInt     //
}

//多个连续的记录信息，记录client链,至少有两个记录
//两个点之间的服务器时间差超过1天将忽略距离 SpanTime(秒）设置
//定位点与标签点差距超过1km，距离递减 GetDisRate 计算
//以上都不影响链的链接，只是会减少距离提成
//标签距离合计，后一个经纬度与前一个距离之和 单位：米,如果有prevhash需要计算第一个与prevhash指定的最后一个单元距离
//所有distance之和就是clientid的总的distance
func CalcDistance(items []UnitBlock) (float64, error) {
	if len(items) < 2 {
		return 0, errors.New("items count error")
	}
	ssum := float64(0)
	for i := 1; i < len(items); i++ {
		cv := items[i+0]
		pv := items[i-1]
		if cv.IsFirst() {
			return 0, errors.New("curr point error")
		}
		//记录时间差太多忽略这个点
		if cv.TimeSub() > conf.TimeDis {
			continue
		}
		if !cv.Prev.Equal(pv.Hash()) {
			return 0, errors.New("prev hash error")
		}
		//两次记录时间必须连续
		st := pv.STimeSub(cv)
		if st < 0 {
			return ssum, errors.New("stime error")
		}
		//两次记录时间差不能太大
		if st > conf.SpanTime {
			continue
		}
		//获取当前点定位差
		csl := cv.LocSub()
		//上一点的定位差
		psl := pv.LocSub()
		//定位不准范围太大将影响距离的计算
		csr := GetDisRate(csl)
		psr := GetDisRate(psl)
		dis := pv.TLocSub(cv) * csr * psr
		ssum += dis
	}
	return ssum, nil
}

func (v *Unit) Check() error {
	//检测上一个has
	if len(v.Items) < 2 {
		return errors.New("items count too slow")
	}
	items := make([]UnitBlock, 0)
	//不是第一个将上获取上一个点并加入items之前
	if !v.Items[0].IsFirst() {
		//
	}
	items = append(items, v.Items...)
	for _, v := range items {
		buf := &bytes.Buffer{}
		err := v.Encode(buf)
		if err != nil {
			return err
		}
		err = v.SerPart.Verify(conf, buf.Bytes())
		if err != nil {
			return err
		}
	}
	dis, err := CalcDistance(items)
	if err != nil {
		return err
	}
	if dis < 0 || dis > EARTH_RADIUS {
		return errors.New("distance range error")
	}
	v.Distance = VarUInt((dis))
	return nil
}

func (v Unit) Encode(w IWriter) error {
	if err := v.ClientID.Encode(w); err != nil {
		return err
	}
	if err := v.PrevHash.Encode(w); err != nil {
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
	if err := v.Distance.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *Unit) Decode(r IReader) error {
	if err := v.ClientID.Decode(r); err != nil {
		return err
	}
	if err := v.PrevHash.Decode(r); err != nil {
		return err
	}
	if err := v.PrevIndex.Decode(r); err != nil {
		return err
	}
	inum := VarUInt(0)
	if err := inum.Decode(r); err != nil {
		return err
	}
	v.Items = make([]UnitBlock, inum)
	for i, _ := range v.Items {
		err := v.Items[i].Decode(r)
		if err != nil {
			return err
		}
	}
	if err := v.Distance.Decode(r); err != nil {
		return err
	}
	return nil
}
