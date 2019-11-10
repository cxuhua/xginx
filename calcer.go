package xginx

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

//积分分配比例 矿工，标签属主，签到人
type TokenAlloc uint8

func (v TokenAlloc) ToUInt8() uint8 {
	return uint8(v)
}

func (v TokenAlloc) Encode(w IWriter) error {
	return binary.Write(w, Endian, v)
}

func (v *TokenAlloc) Decode(r IReader) error {
	return binary.Read(r, Endian, &v)
}

//矿工，标签，用户，获得积分比例
func (v TokenAlloc) Scale() (float64, float64, float64) {
	m := float64((v >> 5) & 0b111)
	t := float64((v >> 2) & 0b111)
	c := float64(v & 0b11)
	return m / 10.0, t / 10.0, c / 10.0
}

//3个值之和应该为10
func (v TokenAlloc) Check() error {
	av := ((v >> 5) & 0b111) + ((v >> 2) & 0b111) + (v & 0b11)
	if av != 10 {
		return errors.New("value error,alloc sum != 10")
	}
	return nil
}

const (
	S631 = TokenAlloc(0b110_011_01)
	S622 = TokenAlloc(0b110_010_10)
	S640 = TokenAlloc(0b110_100_00)
	S550 = TokenAlloc(0b101_101_00)
	S721 = TokenAlloc(0b111_010_01)
)

//token结算接口
type ITokenCalcer interface {
	//开始结算
	Calc(bits uint32, items *Units) error
	//总的积分
	Total() VarUInt
	//积分分配
	Outs() map[HASH160]VarUInt
	//设置矿工hash id
	SetMiner(id HASH160)
}

type TokenCalcer struct {
	vmap  map[HASH160]float64 //标签获得的积分
	miner HASH160
}

func NewTokenCalcer(miner HASH160) ITokenCalcer {
	return &TokenCalcer{
		vmap:  map[HASH160]float64{},
		miner: miner,
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

func (calcer *TokenCalcer) SetMiner(id HASH160) {
	calcer.miner = id
}

func (calcer *TokenCalcer) Total() VarUInt {
	v := VarUInt(0)
	for _, tv := range calcer.vmap {
		v += VarUInt(tv)
	}
	return v
}

//标签获得的积分
func (calcer *TokenCalcer) Outs() map[HASH160]VarUInt {
	ret := map[HASH160]VarUInt{}
	for k, v := range calcer.vmap {
		ret[k] += VarUInt(v)
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
func (calcer *TokenCalcer) Calc(bits uint32, us *Units) error {
	if len(*us) < 2 {
		return errors.New("items count error")
	}
	if !CheckProofOfWorkBits(bits) {
		return errors.New("proof of work bits error")
	}
	tpv := CalculateWorkTimeScale(bits)
	for i := 1; i < len(*us); i++ {
		cv := (*us)[i+0]
		//使用当前标签设定的分配比例
		if err := cv.TASV.Check(); err != nil {
			return fmt.Errorf("item asv error %w", err)
		}
		mr, tr, cr := cv.TASV.Scale()
		pv := (*us)[i-1]
		if !cv.CliID().Equal(pv.CliID()) {
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
		//矿工获得
		mdis := dis * mr
		//标签所有者获得,两标签平分
		tdis := (dis * tr) * 0.5
		calcer.vmap[cv.TPKH] += tdis
		calcer.vmap[pv.TPKH] += tdis
		cdis := dis * cr
		calcer.vmap[cv.CliID()] += cdis
		//保存矿工获得的总量
		calcer.vmap[calcer.miner] += mdis
	}
	if calcer.Total() < 0 {
		return errors.New("total range error")
	}
	return nil
}
