package main

import (
	"errors"
	"time"

	. "github.com/cxuhua/xginx"
)

//测试用监听器
type listener struct {
}

func newListener() IListener {
	return &listener{}
}

func (lis *listener) OnTxPool(tx *TX) error {
	return nil
}

func (lis *listener) OnTxPoolRep(old *TX, new *TX) {

}

func (lis *listener) OnInit(bi *BlockIndex) error {
	if bv := bi.GetBestValue(); !bv.IsValid() {
		bi.WriteGenesis()
	}
	return nil
}

func (lis *listener) OnLinkBlock(blk *BlockInfo) {

}

func (lis *listener) OnClientMsg(c *Client, msg MsgIO) {
	//LogInfo(msg.Type())
}

func (lis *listener) TimeNow() uint32 {
	return uint32(time.Now().Unix())
}

func (lis *listener) OnUnlinkBlock(blk *BlockInfo) {

}

func (lis *listener) OnClose() {

}

//当块创建完毕
func (lis *listener) OnNewBlock(blk *BlockInfo) error {
	conf := GetConfig()
	//设置base out script
	//创建coinbase tx
	tx := NewTx()
	txt := time.Now().Format("2006-01-02 15:04:05")
	addr := conf.GetNetAddr()
	//base tx
	in := NewTxIn()
	in.Script = blk.CoinbaseScript(addr.IP(), []byte(txt))
	tx.Ins = []*TxIn{in}
	//
	out := &TxOut{}
	out.Value = blk.CoinbaseReward()
	//锁定到矿工账号
	pkh, err := conf.MinerAddr.GetPkh()
	if err != nil {
		return err
	}
	script, err := NewLockedScript(pkh)
	if err != nil {
		return err
	}
	out.Script = script
	tx.Outs = []*TxOut{out}
	blk.Txs = []*TX{tx}
	return nil
}

func (lis *listener) OnStartup() {

}

//当账户没有私钥时调用此方法签名
//singer 签名器
func (lis *listener) OnSignTx(signer ISigner) error {
	return errors.New("not imp OnSignTx")
}

//完成区块
func (lis *listener) OnFinished(blk *BlockInfo) error {
	//处理交易费用
	if len(blk.Txs) == 0 {
		return errors.New("coinbase tx miss")
	}
	tx := blk.Txs[0]
	if !tx.IsCoinBase() {
		return errors.New("coinbase tx miss")
	}
	bi := GetBlockIndex()
	//交易费用处理，添加给矿工
	fee, err := blk.GetFee(bi)
	if err != nil {
		return err
	}
	if fee > 0 {
		tx.Outs[0].Value += fee
	}
	return nil
}
