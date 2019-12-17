package xginx

import "errors"

//转账监听器
type ITransListener interface {
	//获取金额对应的账户方法
	GetAcc(ckv *CoinKeyValue) *Account
	//获取输出地址的扩展
	GetExt(addr Address) []byte
	//获取使用的金额
	GetCoins() Coins
	//获取找零地址
	GetKeep() Address
}

//交易数据结构
type Trans struct {
	bi   *BlockIndex
	lis  ITransListener
	Dst  []Address //目标地址
	Amts []Amount  //目标金额 大小与dst对应
	Fee  Amount    //交易费
}

//检测参数
func (m *Trans) Check() error {
	if m.lis == nil || m.bi == nil {
		return errors.New("lis or bi args error")
	}
	if len(m.Dst) == 0 {
		return errors.New("dst count == 0")
	}
	if len(m.Dst) != len(m.Amts) {
		return errors.New("dst address and amount error")
	}
	return nil
}

//生成交易,不签名，不放入交易池
func (m *Trans) NewTx() (*TX, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	tx := NewTx()
	//输出总计
	sum := m.Fee
	for _, v := range m.Amts {
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
		return nil, errors.New("insufficient balance or miss private key")
	}
	//转出到其他账号的输出
	for i, v := range m.Amts {
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
		addr := m.lis.GetKeep()
		out, err := addr.NewTxOut(amt)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	return tx, nil
}

func (m *Trans) BroadTx(tx *TX) {
	ps := GetPubSub()
	ps.Pub(tx, NewTxTopic)
}

//创建待回调的交易对象
func (bi *BlockIndex) NewTrans(lis ITransListener) *Trans {
	return &Trans{
		bi:   bi,
		lis:  lis,
		Dst:  []Address{},
		Amts: []Amount{},
		Fee:  0,
	}
}
