package main

import (
	"errors"
	"time"

	. "github.com/cxuhua/xginx"
)

var (
	MinerAccount, _ = LoadAccount("BNE8o2x2MEVvqvr2tBocqcLRkKTw9oAbLquruDBkH87t9nBuNg84znbBonLJyEZAATrvXZm3MSiK2giFSZgszovHf8VhkjjjXyXpByFSBn2A958aYRQ8vVwZBULYtjmZn9qD3BJ8CkgcSHBn1rxCCHGUMyWFjWvuQSdhPhfVTi4B6nQsVbgQXiY2UN5q5m5aC8tFumWswX4qnZ9BvUHzprotLWpGDoCbmBiVVYKgigXoGy7kfok18ecTVR4XXSdh4UoAbcWhWSrpEdnLa4AxUm8NW5LqnUyvKpxqymTAJmAdB9iZqxG5jpn2hpcjnfx7pRGHp13SvMM461YCWbpf1rJUpWeg8P89x2uXaq9XRsdoBz9yTu3Rj1rRaLgVfREd7QTjtEnkJq1K8LEe4N74wRb7jxvnqQsGq89YrqH8mXaL7Tn5qarxUAnovQskNByb7F7R8dzUeKs1iZg1oVfhsenuCpPj2igCpspQn6oFTKtR45KF5KSMdLKKx7qn4Jx")
)

//测试用监听器
type listener struct {
	wallet IWallet
}

func newListener(wdir string) *listener {
	w, err := NewLevelDBWallet(wdir)
	if err != nil {
		panic(err)
	}
	return &listener{
		wallet: w,
	}
}

func (lis *listener) OnClose(bi *BlockIndex) {
	lis.wallet.Close()
}

func (lis *listener) OnLinkBlock(bi *BlockIndex, blk *BlockInfo) {

}

//当块创建完毕
func (lis *listener) OnNewBlock(bi *BlockIndex, blk *BlockInfo) error {
	//设置base out script
	//创建coinbase tx
	tx := &TX{}
	tx.Ver = 1

	txt := time.Now().Format("2006-01-02 15:04:05")

	//base tx
	in := &TxIn{}
	in.Script = blk.CoinbaseScript([]byte(txt))
	tx.Ins = []*TxIn{in}
	//
	out := &TxOut{}
	out.Value = blk.CoinbaseReward()
	if script, err := MinerAccount.NewLockedScript(); err != nil {
		return err
	} else {
		out.Script = script
	}
	tx.Outs = []*TxOut{out}

	blk.Txs = []*TX{tx}
	return nil
}

//完成区块
func (lis *listener) OnFinished(bi *BlockIndex, blk *BlockInfo) error {
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
func (lis *listener) GetAccount(bi *BlockIndex, pkh HASH160) (*Account, error) {
	return MinerAccount, nil
}
