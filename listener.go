package xginx

import (
	"errors"
	"os"
	"time"
)

//IListener 所有回调可能来自不同的协程
type IListener interface {
	//首次初始化时
	OnInit(bi *BlockIndex) error
	//时间戳发生器
	TimeNow() uint32
	//当一个区块断开后
	OnUnlinkBlock(blk *BlockInfo)
	//更新区块数据成功时
	OnLinkBlock(blk *BlockInfo)
	//当块创建时，可以添加，修改块内信息
	OnNewBlock(blk *BlockInfo) error
	//完成区块，当检测完成调用,设置merkle之前
	OnFinished(blk *BlockInfo) error
	//当收到网络数据时,数据包根据类型转换成需要的包
	OnClientMsg(c *Client, msg MsgIO)
	//链关闭时
	OnClose()
	//当服务启动后会调用一次
	OnStart()
	//系统结束时
	OnStop(sig os.Signal)
	//当交易进入交易池之前，返回错误不会进入交易池
	OnTxPool(tx *TX) error
	//当交易池的交易因为seq设置被替换时
	OnTxPoolRep(old *TX, new *TX)
}

//Listener 默认监听器
type Listener struct {
}

//OnTxPool 当交易进入交易池之前，返回错误不会进入交易池
func (lis *Listener) OnTxPool(tx *TX) error {
	return nil
}

//OnTxPoolRep 当交易被替换
func (lis *Listener) OnTxPoolRep(old *TX, new *TX) {

}

//OnInit 首次启动初始化
func (lis *Listener) OnInit(bi *BlockIndex) error {
	LogInfo("MinerAddr =", conf.MinerAddr)
	if bv := bi.GetBestValue(); !bv.IsValid() {
		bi.WriteGenesis()
	}
	return nil
}

//OnLinkBlock 区块链入时
func (lis *Listener) OnLinkBlock(blk *BlockInfo) {

}

//OnClientMsg 收到网络信息
func (lis *Listener) OnClientMsg(c *Client, msg MsgIO) {
	//LogInfo(msg.Type())
}

//TimeNow 当前时间戳获取
func (lis *Listener) TimeNow() uint32 {
	return uint32(time.Now().Unix())
}

//OnUnlinkBlock 区块断开
func (lis *Listener) OnUnlinkBlock(blk *BlockInfo) {

}

//OnStart 启动时
func (lis *Listener) OnStart() {
	LogInfo("xginx start")
}

//OnStop 停止
func (lis *Listener) OnStop(sig os.Signal) {
	LogInfo("xginx stop sig=", sig)
}

//OnSignTx 当账户没有私钥时调用此方法签名
//singer 签名器
func (lis *Listener) OnSignTx(signer ISigner) error {
	return errors.New("not imp OnSignTx")
}

//OnClose 区块链关闭
func (lis *Listener) OnClose() {
	LogInfo("xginx block index close")
}

//OnNewBlock 当块创建完毕
//默认创建coinbase交易加入区块为区块第一个交易
func (lis *Listener) OnNewBlock(blk *BlockInfo) error {
	conf := GetConfig()
	//设置base out script
	//创建coinbase tx
	tx := NewTx(0)
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
	script, err := NewLockedScript(pkh, DefaultLockedScript)
	if err != nil {
		return err
	}
	out.Script = script
	tx.Outs = []*TxOut{out}
	blk.Txs = []*TX{tx}
	return nil
}

//OnFinished 完成区块
//交易加入完成，在计算难度前
func (lis *Listener) OnFinished(blk *BlockInfo) error {
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
