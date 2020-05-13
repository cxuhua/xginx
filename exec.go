package xginx

import "log"

const (
	//OptPushTxPool 当交易进入交易池
	OptPushTxPool = 1
	//OptAddToBlock 当交易加入区块
	OptAddToBlock = 2
	//OptPublishTx 发布交易到网络
	OptPublishTx = 3
)

//ExecScript 返回错误会不加入交易池或者不进入区块
//执行之前已经校验了签名
func (tx TX) ExecScript(bi *BlockIndex, opt int) error {
	id, _ := tx.ID()
	log.Println(id, "ExecScript = ", opt)
	return nil
}

//ExecScript 执行签名交易脚本
//执行之前签名已经通过
func (sr mulsigner) ExecScript(wits WitnessScript, lcks LockedScript) error {
	id, _ := sr.tx.ID()
	log.Println(id, "ExecScript Verify Sign")
	return nil
}
