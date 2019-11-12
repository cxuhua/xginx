package main

import (
	"errors"

	xx "github.com/cxuhua/xginx"
)

//测试用监听器
type listener struct {
}

//当块创建完毕
func (lis *listener) OnNewBlock(bi *xx.BlockIndex, blk *xx.BlockInfo) error {
	script, err := xx.NewStdLockedScript(nil)
	if err != nil {
		return err
	}
	//设置base out script
	//创建coinbase tx
	tx := &xx.TX{}
	tx.Ver = 1

	//base tx
	in := &xx.TxIn{}
	in.Script = blk.CoinbaseScript([]byte("Test Block"))
	tx.Ins = []*xx.TxIn{in}

	out := &xx.TxOut{}
	out.Value = blk.CoinbaseReward()
	out.Script = script
	tx.Outs = []*xx.TxOut{out}

	blk.Txs = []*xx.TX{tx}

	return nil
}

//完成区块
func (lis *listener) OnFinished(bi *xx.BlockIndex, blk *xx.BlockInfo) error {
	if len(blk.Txs) == 0 {
		return errors.New("txs miss")
	}
	btx := blk.Txs[0]
	if !btx.IsCoinBase() {
		return errors.New("coinbase tx miss")
	}
	//交易费用处理
	fee := blk.GetFee(bi)
	if fee == 0 {
		return nil
	}
	btx.Outs[0].Value += fee
	return blk.CheckTxs(bi)
}

//获取签名私钥
func (lis *listener) OnPrivateKey(bi *xx.BlockIndex, blk *xx.BlockInfo, out *xx.TxOut) (*xx.PrivateKey, error) {
	return nil, nil
}
