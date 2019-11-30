package xginx

import "errors"

//交易数据结构
type MulTransInfo struct {
	bi   *BlockIndex
	Src  []Address //原地址
	Keep int       //找零到这个索引对应的src地址
	Dst  []Address //目标地址
	Amts []Amount  //目标金额 大小与dst对应
	Fee  Amount    //交易费
	Ext  []byte    //扩展信息
}

func (m *MulTransInfo) Check() error {
	if len(m.Src) == 0 || len(m.Dst) == 0 || len(m.Dst) != len(m.Amts) {
		return errors.New("src dst amts num error")
	}
	if m.Keep < 0 || m.Keep >= len(m.Src) {
		return errors.New("keep index out bound")
	}
	if !m.Fee.IsRange() {
		return errors.New("fee value error")
	}
	sum := Amount(0)
	for _, v := range m.Amts {
		sum += v
	}
	if !sum.IsRange() {
		return errors.New("amts value error")
	}
	return nil
}

//获取地址对应的账户和金额列表
func (m *MulTransInfo) getAddressInfo(addr Address) (*Account, Coins, error) {
	spkh, err := addr.GetPkh()
	if err != nil {
		return nil, nil, err
	}
	acc, err := m.bi.lptr.GetWallet().GetAccountWithPkh(spkh)
	if err != nil {
		return nil, nil, err
	}
	ds, err := m.bi.ListCoinsWithID(spkh)
	if err != nil {
		return nil, nil, err
	}
	return acc, ds, nil
}

//生成交易
//pri=true表示只使用有私钥的账户
func (m *MulTransInfo) NewTx(pri bool) (*TX, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	tx := NewTx()
	//输出总计
	sum := m.Fee
	for _, v := range m.Amts {
		sum += v
	}
	//计算使用哪些输入
	for _, src := range m.Src {
		//获取转出账号信息
		acc, ds, err := m.getAddressInfo(src)
		if err != nil {
			return nil, err
		}
		//是否只使用有私钥的账户
		if pri && !acc.HasPrivate() {
			continue
		}
		//获取需要消耗的输出
		for _, cv := range ds {
			//生成待签名的输入
			in, err := cv.NewTxIn(acc)
			if err != nil {
				return nil, err
			}
			tx.Ins = append(tx.Ins, in)
			sum -= cv.Value
			if sum <= 0 {
				break
			}
		}
	}
	//没有减完，余额不足
	if sum > 0 {
		return nil, errors.New("insufficient balance")
	}
	//转出到其他账号的输出
	for i, v := range m.Amts {
		//创建目标输出
		out, err := m.Dst[i].NewTxOut(v, m.Ext)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//多减的需要找零钱给自己，否则金额就会丢失
	if amt := -sum; amt > 0 {
		out, err := m.Src[m.Keep].NewTxOut(amt)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//开始签名
	if err := tx.Sign(m.bi); err != nil {
		return nil, err
	}
	//放入交易池
	if err := m.bi.txp.PushTx(m.bi, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

func (bi *BlockIndex) EmptyMulTransInfo() *MulTransInfo {
	return &MulTransInfo{
		bi:   bi,
		Src:  []Address{},
		Keep: 0,
		Dst:  []Address{},
		Amts: []Amount{},
		Fee:  0,
	}
}
