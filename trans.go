package xginx

import (
	"errors"
)

//ITransListener 转账监听器
type ITransListener interface {
	//获取金额对应的账户方法
	GetAcc(ckv *CoinKeyValue) *Account
	//获取输出地址的扩展不同的地址可以返回不同的扩展信息
	GetExt(addr Address) []byte
	//获取使用的金额列表
	GetCoins() Coins
	//获取找零地址
	GetKeep() Address
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
func (m *Trans) NewTx(lt ...uint32) (*TX, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	if !m.Fee.IsRange() {
		return nil, errors.New("fee error")
	}
	tx := NewTx()
	if len(lt) > 0 {
		tx.LockTime = lt[0]
	}
	//输出总计
	sum := m.Fee
	for _, v := range m.Amt {
		sum += v
	}
	//使用哪些金额
	for _, ckv := range m.lis.GetCoins() {
		//获取消费金额对应的账户
		acc := m.lis.GetAcc(ckv)
		if acc == nil {
			return nil, errors.New("get account error for coin")
		}
		//生成待签名的输入
		in, err := ckv.NewTxIn(acc)
		if err != nil {
			return nil, err
		}
		tx.Ins = append(tx.Ins, in)
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
	for i, v := range m.Amt {
		dst := m.Dst[i]
		ext := m.lis.GetExt(dst)
		out, err := dst.NewTxOut(v, ext)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//多减的需要找零钱给自己，否则金额就会丢失
	if amt := -sum; amt > 0 {
		//获取找零地址
		addr := m.lis.GetKeep()
		ext := m.lis.GetExt(addr)
		out, err := addr.NewTxOut(amt, ext)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//如果lis实现了签名lis
	if slis, ok := m.lis.(ISignerListener); ok {
		err := tx.Sign(m.bi, slis)
		if err != nil {
			return nil, err
		}
	}
	return tx, nil
}

//BroadTx 广播交易
func (m *Trans) BroadTx(tx *TX) {
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
