package main

import (
	"errors"
	"fmt"
	"time"

	xx "github.com/cxuhua/xginx"
)

const (
	maddr = "st1q363x0zvheem0a5f0r0z9qr9puj7l900jc8glh0" //区块奖励地址
)

//测试用监听器
type listener struct {
	wallet xx.IWallet
}

func newListener(wdir string) *listener {
	w, err := xx.NewLevelDBWallet(wdir)
	if err != nil {
		panic(err)
	}
	return &listener{
		wallet: w,
	}
}

func (lis *listener) OnClose(bi *xx.BlockIndex) {
	lis.wallet.Close()
}

func (lis *listener) OnLinkBlock(bi *xx.BlockIndex, blk *xx.BlockInfo) {

}

//当块创建完毕
func (lis *listener) OnNewBlock(bi *xx.BlockIndex, blk *xx.BlockInfo) error {
	id, err := xx.DecodeAddress(maddr)
	if err != nil {
		return err
	}
	script, err := xx.NewLockedScript(id)
	if err != nil {
		return fmt.Errorf("new stdlocked script error %w", err)
	}
	//设置base out script
	//创建coinbase tx
	tx := &xx.TX{}
	tx.Ver = 1

	txt := time.Now().Format("2006-01-02 15:04:05")

	//base tx
	in := &xx.TxIn{}
	in.Script = blk.CoinbaseScript([]byte(txt))
	tx.Ins = []*xx.TxIn{in}
	//
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
	fee, err := blk.GetFee(bi)
	if err != nil {
		return err
	}
	if fee == 0 {
		return nil
	}
	btx.Outs[0].Value += fee
	return blk.CheckTxs(bi)
}

//获取签名私钥
func (lis *listener) OnPrivateKey(bi *xx.BlockIndex, blk *xx.BlockInfo, out *xx.TxOut) (*xx.PrivateKey, error) {
	pkh := out.Script.GetPkh()
	addr, err := xx.EncodeAddress(pkh)
	if err != nil {
		return nil, err
	}
	if lis.wallet == nil {
		return nil, errors.New("wallet not set")
	}
	return lis.wallet.GetPrivate(addr)
}
