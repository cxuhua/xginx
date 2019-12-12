package xginx

import (
	"errors"
	"time"
)

//测试用监听器
type testLisener struct {
	bi *BlockIndex
	st uint32
}

func newListener() IListener {
	return &testLisener{
		st: uint32(time.Now().Unix()),
	}
}

func (lis *testLisener) OnTxRep(old *TX, new *TX) {

}

func (lis *testLisener) OnUnlinkBlock(blk *BlockInfo) {

}

func (lis *testLisener) OnTxPool(tx *TX) error {
	return nil
}

func (lis *testLisener) OnSignTx(singer ISigner) error {
	return errors.New("not imp OnSignTx")
}

func (lis *testLisener) SetBlockIndex(bi *BlockIndex) {
	lis.bi = bi
}

func (lis *testLisener) OnLinkBlock(blk *BlockInfo) {

}

func (lis *testLisener) TimeNow() uint32 {
	lis.st++
	return lis.st
}

func (lis *testLisener) OnClientMsg(c *Client, msg MsgIO) {

}

func (lis *testLisener) OnClose() {

}

func (lis *testLisener) IsTest() bool {
	return true
}

//当块创建完毕
func (lis *testLisener) OnNewBlock(blk *BlockInfo) error {
	conf = LoadConfig("test.json")
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

func (lis *testLisener) OnStartup() {

}

//完成区块
func (lis *testLisener) OnFinished(blk *BlockInfo) error {
	if len(blk.Txs) == 0 {
		return errors.New("txs miss")
	}
	tx := blk.Txs[0]
	if !tx.IsCoinBase() {
		return errors.New("coinbase tx miss")
	}
	//交易费用处理，添加给矿工
	fee, err := blk.GetFee(lis.bi)
	if err != nil {
		return err
	}
	if fee == 0 {
		return nil
	}
	tx.Outs[0].Value += fee
	return nil
}
