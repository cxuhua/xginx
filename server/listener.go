package main

import (
	"errors"
	"fmt"
	"time"

	. "github.com/cxuhua/xginx"
)

//测试用监听器
type listener struct {
	wallet IWallet
}

func newListener(wdir string) IListener {
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

func (lis *listener) GetWallet() IWallet {
	return lis.wallet
}

func (lis *listener) OnLinkBlock(bi *BlockIndex, blk *BlockInfo) {

}

//当块创建完毕
func (lis *listener) OnNewBlock(bi *BlockIndex, blk *BlockInfo) error {
	//获取矿工账号
	acc := Miner.GetMiner()
	if acc == nil {
		return fmt.Errorf("miner set miss")
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
	if script, err := acc.NewLockedScript(); err != nil {
		return err
	} else {
		out.Script = script
	}
	tx.Outs = []*TxOut{out}

	blk.Txs = []*TX{tx}
	return nil
}

func (lis *listener) OnStartup() {

	//获取并设置矿工账号
	acc, err := lis.wallet.GetMiner()
	if err != nil {
		panic(err)
	}
	addr, err := acc.GetAddress()
	if err != nil {
		panic(err)
	}
	LogInfo("miner address = ", addr)
	err = Miner.SetMiner(acc)
	if err != nil {
		panic(err)
	}
	//测试挖一个矿

	//ps := GetPubSub()
	//ps.Pub(MinerAct{
	//	Opt: OptGenBlock,
	//	Arg: uint32(1),
	//}, NewMinerActTopic)
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

//从钱包获取签名账号
func (lis *listener) GetAccount(bi *BlockIndex, pkh HASH160) (*Account, error) {
	addr, err := EncodeAddress(pkh)
	if err != nil {
		return nil, err
	}
	return lis.wallet.GetAccount(addr)
}
