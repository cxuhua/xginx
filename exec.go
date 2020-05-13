package xginx

import (
	"log"

	lua "github.com/yuin/gopher-lua"
)

const (
	//OptPushTxPool 当交易进入交易池
	OptPushTxPool = 1
	//OptAddToBlock 当交易加入区块
	OptAddToBlock = 2
	//OptPublishTx 发布交易到网络
	OptPublishTx = 3
)

//初始化脚本状态机
func initLuaEnv(slibs ...bool) *lua.LState {
	opts := lua.Options{
		CallStackSize:       64,
		MinimizeStackMemory: true,
		SkipOpenLibs:        len(slibs) == 0 || slibs[0],
	}
	l := lua.NewState(opts)
	return l
}

//ExecScript 返回错误会不加入交易池或者不进入区块
//执行之前已经校验了签名
func (tx TX) ExecScript(bi *BlockIndex, opt int) error {
	l := initLuaEnv()
	defer l.Close()
	id, _ := tx.ID()
	log.Println(id, "ExecScript = ", opt, bi.Height())
	return nil
}

//ExecScript 执行签名交易脚本
//执行之前签名已经通过
func (sr mulsigner) ExecScript(bi *BlockIndex, wits WitnessScript, lcks LockedScript) error {
	l := initLuaEnv()
	defer l.Close()
	id, _ := sr.tx.ID()
	log.Println(id, "ExecScript Verify Sign", bi.Height())
	return nil
}
