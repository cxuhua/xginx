package xginx

import (
	"errors"
	"time"
)

//IListener 所有回调可能来自不同的协程
type IListener interface {
	//首次初始化时在加载区块链之前
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
	//当加载交易列表到区块时可用此方法过滤不加入区块的
	//调用 AddTxs 时会触发
	OnLoadTxs(txs []*TX) []*TX
	//链关闭时
	OnClose()
	//当服务启动后会调用一次,区块链,节点服务启动后
	OnStart()
	//系统结束时
	OnStop()
	//当交易进入交易池之前，返回错误不会进入交易池
	OnTxPool(tx *TX) error
	//当交易池的交易被替换时
	OnTxPoolRep(old *TX, new *TX)
	//返回矿工账号，如果返回将优先使用这个地址
	MinerAddr() Address
}

//OnNewBlock 当块创建完毕
//默认创建coinbase交易加入区块为区块第一个交易
func DefaultNewBlock(lis IListener, blk *BlockInfo) error {
	conf := GetConfig()
	//设置base out script
	//创建coinbase tx
	tx := NewTx(0)
	txt := time.Now().Format("2006-01-02 15:04:05")
	addr := conf.GetNetAddr()
	//base tx
	in := NewTxIn()
	script, err := blk.CoinbaseScript(addr.IP(), []byte(txt))
	if err != nil {
		return err
	}
	in.Script = script
	tx.Ins = []*TxIn{in}
	//
	out := &TxOut{}
	out.Value = blk.CoinbaseReward()
	//锁定到矿工账号
	pkh, err := lis.MinerAddr().GetPkh()
	if err != nil {
		return err
	}
	lcks, err := NewLockedScript(pkh, nil, DefaultLockedScript)
	if err != nil {
		return err
	}
	script, err = lcks.ToScript()
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
func DefaultkFinished(blk *BlockInfo) error {
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
