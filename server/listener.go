package main

import (
	"errors"
	"fmt"
	"time"

	. "github.com/cxuhua/xginx"
)

var (
	MinerAccount, _ = LoadAccount("")
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
	id, err := DecodeAddress(maddr)
	if err != nil {
		return err
	}
	script, err := NewLockedScript(id)
	if err != nil {
		return fmt.Errorf("new stdlocked script error %w", err)
	}
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
	out.Script = script
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
func (lis *listener) GetAccount(bi *BlockIndex, blk *BlockInfo, out *TxOut) (*Account, error) {

}
