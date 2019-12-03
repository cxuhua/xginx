package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	. "github.com/cxuhua/xginx"
)

//测试用监听器
type listener struct {
	mu     sync.RWMutex
	bi     *BlockIndex
	conf   *Config
	wallet IWallet
}

func newListener(conf *Config) IListener {
	w, err := NewLevelDBWallet(conf.WalletDir)
	if err != nil {
		panic(err)
	}
	return &listener{
		conf:   conf,
		wallet: w,
	}
}

func (lis *listener) SetBlockIndex(bi *BlockIndex) {
	lis.bi = bi
}

func (lis *listener) GetConfig() *Config {
	return lis.conf
}

func (lis *listener) OnTxPool(tx *TX) error {
	return nil
}

func (lis *listener) OnLinkBlock(blk *BlockInfo) {

}

func (lis *listener) OnClientMsg(c *Client, msg MsgIO) {

}

func (lis *listener) TimeNow() uint32 {
	return uint32(time.Now().Unix())
}

func (lis *listener) OnInitHttp(m *gin.Engine) {

}

func (lis *listener) OnUnlinkBlock(blk *BlockInfo) {

}

func (lis *listener) OnClose() {
	lis.wallet.Close()
}

func (lis *listener) GetWallet() IWallet {
	return lis.wallet
}

//当块创建完毕
func (lis *listener) OnNewBlock(blk *BlockInfo) error {
	conf := lis.GetConfig()
	//获取矿工账号
	acc := Miner.GetMiner()
	if acc == nil {
		return fmt.Errorf("miner set miss")
	}
	//设置base out script
	//创建coinbase tx
	tx := NewTx()
	txt := time.Now().Format("2006-01-02 15:04:05") + " " + conf.TcpIp
	//base tx
	in := &TxIn{}
	in.Script = blk.CoinbaseScript([]byte(txt))
	tx.Ins = []*TxIn{in}
	//
	out := &TxOut{}
	out.Value = blk.CoinbaseReward()
	//锁定到矿工账号
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

//当账户没有私钥时调用此方法签名
//singer 签名器
//wits 脚本对象
func (lis *listener) OnSignTx(singer ISigner, wits *WitnessScript) error {
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
	//交易费用处理，添加给矿工
	fee, err := blk.GetFee(lis.bi)
	if err != nil {
		return err
	}
	if fee > 0 {
		tx.Outs[0].Value += fee
	}
	return blk.CheckTxs(lis.bi)
}
