package xginx

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

//测试用监听器
type testLisener struct {
	bi     *BlockIndex
	wallet IWallet
	st     uint32
}

func newListener(wdir string) IListener {
	w, err := NewLevelDBWallet(wdir)
	if err != nil {
		panic(err)
	}
	return &testLisener{
		wallet: w,
		st:     uint32(time.Now().Unix()),
	}
}

func (lis *testLisener) OnTxRep(old *TX, new *TX) {

}

func (lis *testLisener) OnUnlinkBlock(blk *BlockInfo) {

}

func (lis *testLisener) OnTxPool(tx *TX) error {
	return nil
}

func (lis *testLisener) OnSignTx(singer ISigner, wits *WitnessScript) error {
	return errors.New("not imp OnSignTx")
}

func (lis *testLisener) SetBlockIndex(bi *BlockIndex) {
	lis.bi = bi
}

func (lis *testLisener) OnNewTx(tx *TX) error {
	return nil
}

func (lis *testLisener) GetConfig() *Config {
	return conf
}

func (lis *testLisener) OnLinkBlock(blk *BlockInfo) {

}

func (lis *testLisener) TimeNow() uint32 {
	lis.st++
	return lis.st
}

func (lis *testLisener) OnClientMsg(c *Client, msg MsgIO) {

}

func (lis *testLisener) OnInitHttp(m *gin.Engine) {

}

func (lis *testLisener) OnClose() {
	lis.wallet.Close()
}

func (lis *testLisener) GetWallet() IWallet {
	return lis.wallet
}

func (lis *testLisener) IsTest() bool {
	return true
}

//当块创建完毕
func (lis *testLisener) OnNewBlock(blk *BlockInfo) error {
	//获取矿工账号
	acc := Miner.GetMiner()
	if acc == nil {
		return fmt.Errorf("miner set miss")
	}
	//设置base out script
	//创建coinbase tx
	tx := NewTx()

	txt := time.Now().Format("2006-01-02 15:04:05")
	//base tx
	in := NewTxIn()
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

func (lis *testLisener) OnStartup() {
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
	return blk.CheckTxs(lis.bi)
}
