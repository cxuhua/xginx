package xginx

import (
	"errors"
	"fmt"
)

//ITransListener 转账监听器
//先获取可使用的金额，然后获取金额相关的账户用来签名
//根据转出地址获取扩展数据，剩下的金额转到找零地址
type ITransListener interface {
	//创建输入脚本
	NewWitnessScript(ckv *CoinKeyValue) (*WitnessScript, error)
	//获取使用的金额列表 amt=当前需要的金额
	GetCoins(amt Amount) Coins
}

//Trans 交易数据结构
type Trans struct {
	bi  *BlockIndex
	lis ITransListener
	Dst []Address //目标地址
	Amt []Amount  //目标金额 大小与dst对应
	Fee Amount    //交易费
}

//Clean 清楚交易对象
func (m *Trans) Clean() {
	m.Dst = []Address{}
	m.Amt = []Amount{}
}

//Add 设置一个转账对象
func (m *Trans) Add(dst Address, amt Amount) {
	m.Dst = append(m.Dst, dst)
	m.Amt = append(m.Amt, amt)
}

//Check 检测参数
func (m *Trans) Check() error {
	if m.lis == nil || m.bi == nil {
		return errors.New("lis or bi args error")
	}
	if len(m.Dst) == 0 {
		return errors.New("dst count == 0")
	}
	if len(m.Dst) != len(m.Amt) {
		return errors.New("dst address and amount error")
	}
	return nil
}

//NewTx 生成交易,不签名，不放入交易池
//lt = tx locktime
func (m *Trans) NewTx(exetime uint32, execs ...[]byte) (*TX, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	if !m.Fee.IsRange() {
		return nil, fmt.Errorf("fee %d error", m.Fee)
	}
	tx := NewTx(exetime, execs...)
	//输出金额总计
	sum := m.Fee
	for _, v := range m.Amt {
		sum += v
	}
	//最后一个输入地址默认作为找零地址（如果有零）
	var lout Address
	//使用哪些金额
	for _, ckv := range m.lis.GetCoins(sum) {
		//获取消费金额对应的账户
		wits, err := m.lis.NewWitnessScript(ckv)
		if err != nil {
			return nil, err
		}
		//生成待签名的输入
		in, err := ckv.NewTxIn(wits)
		if err != nil {
			return nil, err
		}
		tx.Ins = append(tx.Ins, in)
		//保存最后一个地址
		lout = ckv.GetAddress()
		sum -= ckv.Value
		if sum <= 0 {
			break
		}
	}
	//没有减完，余额不足
	if sum > 0 {
		return nil, errors.New("insufficient balance")
	}
	//转出到其他账号的输出
	for i, amt := range m.Amt {
		dst := m.Dst[i]
		out, err := dst.NewTxOut(amt, "", DefaultLockedScript)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//多减的需要找零钱给自己，否则金额就会丢失
	if amt := -sum; amt > 0 {
		//默认找零到最后一个地址
		out, err := lout.NewTxOut(amt, "", DefaultLockedScript)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//签名前回调通知
	var err error = nil
	//如果lis实现了签名
	if slis, ok := m.lis.(ISignTx); ok {
		err = tx.Sign(m.bi, slis)
	}
	return tx, err
}

//BroadTx 广播链上交易
func (m *Trans) BroadTx(bi *BlockIndex, tx *TX) {
	ps := GetPubSub()
	ps.Pub(tx, NewTxTopic)
}

//NewTrans 创建待回调的交易对象
func (bi *BlockIndex) NewTrans(lis ITransListener) *Trans {
	return &Trans{
		bi:  bi,
		lis: lis,
		Dst: []Address{},
		Amt: []Amount{},
		Fee: 0,
	}
}
