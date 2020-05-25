package xginx

import (
	"errors"
	"fmt"
)

//ITransNotice 回调通知
//返回错误会导致创建交易失败
type ITransNotice interface {
	//当输入创建好
	OnNewTxIn(tx *TX, in *TxIn) error
	//当输出创建好
	OnNewTxOut(tx *TX, out *TxOut) error
	//当交易创建完毕
	OnNewTx(tx *TX) error
}

//ITransListener 转账监听器
//先获取可使用的金额，然后获取金额相关的账户用来签名
//根据转出地址获取扩展数据，剩下的金额转到找零地址
type ITransListener interface {
	//获取金额对应的账户方法
	GetAcc(ckv *CoinKeyValue) (*Account, error)
	//获取输出执行脚本 addr 输出的地址
	GetTxOutExec(addr Address) []byte
	//获取输入执行脚本 ckv消费的金额对象
	GetTxInExec(ckv *CoinKeyValue) []byte
	//获取使用的金额列表
	GetCoins() Coins
	//获取找零地址，当转账有剩余的金额转到这个地址
	GetKeep() Address
}

//Trans 交易数据结构
type Trans struct {
	bi   *BlockIndex
	lis  ITransListener
	Dst  []Address //目标地址
	Amt  []Amount  //目标金额 大小与dst对应
	Outs []Script  //对应输出脚本
	Fee  Amount    //交易费
}

//Clean 清楚交易对象
func (m *Trans) Clean() {
	m.Dst = []Address{}
	m.Amt = []Amount{}
}

//Add 设置一个转账对象
func (m *Trans) Add(dst Address, amt Amount, out Script) {
	m.Dst = append(m.Dst, dst)
	m.Amt = append(m.Amt, amt)
	m.Outs = append(m.Outs, out)
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
		return nil, errors.New("fee error")
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
	for _, ckv := range m.lis.GetCoins() {
		//获取消费金额对应的账户
		acc, err := m.lis.GetAcc(ckv)
		if err != nil {
			return nil, err
		}
		//输入执行脚本
		exec := m.lis.GetTxInExec(ckv)
		//生成待签名的输入
		in, err := ckv.NewTxIn(acc, exec)
		if err != nil {
			return nil, err
		}
		//添加前回调通知
		if np, ok := m.lis.(ITransNotice); ok {
			err = np.OnNewTxIn(tx, in)
		}
		if err != nil {
			return nil, err
		}
		tx.Ins = append(tx.Ins, in)
		//保存最后一个地址
		lout, err = acc.GetAddress()
		if err != nil {
			return nil, err
		}
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
		outs := m.Outs[i]
		//如果未设置从回调获取
		if outs.Len() == 0 {
			outs = m.lis.GetTxOutExec(dst)
		}
		out, err := dst.NewTxOut(amt, outs)
		if err != nil {
			return nil, err
		}
		//添加前回调通知
		if np, ok := m.lis.(ITransNotice); ok {
			err = np.OnNewTxOut(tx, out)
		}
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//多减的需要找零钱给自己，否则金额就会丢失
	if amt := -sum; amt > 0 {
		//获取找零地址
		addr := m.lis.GetKeep()
		//如果没有设置找零地址使用最后一个输入地址
		if addr == EmptyAddress {
			addr = lout
		}
		if addr == EmptyAddress {
			return nil, fmt.Errorf("keep address empty")
		}
		//添加前回调通知
		exec := m.lis.GetTxOutExec(addr)
		out, err := addr.NewTxOut(amt, exec)
		if err != nil {
			return nil, err
		}
		if np, ok := m.lis.(ITransNotice); ok {
			err = np.OnNewTxOut(tx, out)
		}
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//签名前回调通知
	var err error = nil
	if np, ok := m.lis.(ITransNotice); ok {
		err = np.OnNewTx(tx)
	}
	if err != nil {
		return nil, err
	}
	//如果lis实现了签名
	if slis, ok := m.lis.(ISignerListener); ok {
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
		bi:   bi,
		lis:  lis,
		Dst:  []Address{},
		Amt:  []Amount{},
		Outs: []Script{},
		Fee:  0,
	}
}
