package xginx

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

//测试用监听器
type testLisener struct {
	wallet IWallet
}

func newListener(wdir string) IListener {
	w, err := NewLevelDBWallet(wdir)
	if err != nil {
		panic(err)
	}
	return &testLisener{
		wallet: w,
	}
}

func (lis *testLisener) OnNewTx(bi *BlockIndex, tx *TX) error {
	return nil
}

func (lis *testLisener) OnUpdateHeader(bi *BlockIndex, ele *TBEle) {

}

func (lis *testLisener) OnUpdateBlock(bi *BlockIndex, blk *BlockInfo) {

}

func (lis *testLisener) OnClientMsg(c *Client, msg MsgIO) {

}

func (lis *testLisener) OnInitHttp(m *gin.Engine) {

}

func (lis *testLisener) OnClose(bi *BlockIndex) {
	lis.wallet.Close()
}

func (lis *testLisener) GetWallet() IWallet {
	return lis.wallet
}

//当块创建完毕
func (lis *testLisener) OnNewBlock(bi *BlockIndex, blk *BlockInfo) error {
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
func (lis *testLisener) OnFinished(bi *BlockIndex, blk *BlockInfo) error {
	if len(blk.Txs) == 0 {
		return errors.New("txs miss")
	}
	tx := blk.Txs[0]
	if !tx.IsCoinBase() {
		return errors.New("coinbase tx miss")
	}
	//交易费用处理，添加给矿工
	fee, err := blk.GetFee(bi)
	if err != nil {
		return err
	}
	if fee == 0 {
		return nil
	}
	tx.Outs[0].Value += fee
	return blk.CheckTxs(bi)
}
