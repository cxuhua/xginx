package xginx

import (
	"context"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const (
	//OptPushTxPool 当交易进入交易池
	OptPushTxPool = 1
	//OptAddToBlock 当交易加入区块
	OptAddToBlock = 2
	//OptPublishTx 发布交易到网络
	OptPublishTx = 3
	//执行成功返回
	ExecOK = "OK"
)

var (
	//是否调式脚本
	DebugScript = false
	//成功脚本
	SuccessScript = []byte("result='OK';")
)

//初始化脚本状态机
func initLuaEnv(cpu time.Duration, tx *TX, bi *BlockIndex, opt int) (*lua.LState, context.CancelFunc) {
	opts := lua.Options{
		CallStackSize:       64,
		MinimizeStackMemory: true,
		SkipOpenLibs:        !DebugScript,
	}
	ctx, cancel := context.WithTimeout(context.Background(), cpu)
	l := lua.NewState(opts)
	l.SetContext(ctx)
	//默认错误
	l.SetGlobal("result", lua.LString("error"))
	//交易版本
	l.SetGlobal("tx_ver", lua.LNumber(tx.Ver))
	//当前区块高度和世界OK
	l.SetGlobal("best_height", lua.LNumber(bi.Height()))
	l.SetGlobal("best_time", lua.LNumber(bi.Time()))
	//交易操作
	l.SetGlobal("tx_opt", lua.LNumber(opt))
	//当前系统时间
	l.SetGlobal("sys_time", lua.LNumber(bi.lptr.TimeNow()))
	return l, cancel
}

//编译脚本
func compileScript(l *lua.LState, codes ...[]byte) error {
	buf := NewReadWriter()
	for _, vb := range codes {
		buf.WriteFull(vb)
	}
	if DebugScript {
		LogInfo(string(buf.Bytes()))
	}
	fn, err := l.Load(buf, "<string>")
	if err != nil {
		return err
	}
	l.Push(fn)
	return nil
}

//执行脚本
func execScript(l *lua.LState) error {
	err := l.PCall(0, lua.MultRet, nil)
	if err != nil {
		return err
	}
	result := l.GetGlobal("result")
	if result.Type() != lua.LTString {
		return fmt.Errorf("script result type error")
	}
	if str := result.String(); str != ExecOK {
		return fmt.Errorf("script result error %s", str)
	}
	return nil
}

//ExecScript 返回错误会不加入交易池或者不进入区块
//执行之前已经校验了签名
func (tx TX) ExecScript(bi *BlockIndex, opt int) error {
	txs, err := tx.Script.ToTxScript()
	if err != nil {
		return err
	}
	//如果未设置就不执行了
	if txs.Exec.Len() == 0 {
		return nil
	}
	//交易脚本执行时间为cpu/2
	tv := time.Duration(txs.ExeTime/2) * time.Millisecond
	//
	l, cancel := initLuaEnv(tv, &tx, bi, opt)
	defer cancel()
	defer l.Close()
	err = compileScript(l, txs.Exec)
	if err != nil {
		return err
	}
	return execScript(l)
}

//ExecScript 执行签名交易脚本
//执行之前签名已经通过
func (sr mulsigner) ExecScript(bi *BlockIndex, wits WitnessScript, lcks LockedScript) error {
	//如果未设置就不执行了
	if wits.Exec.Len()+lcks.Exec.Len() == 0 {
		return nil
	}
	txs, err := sr.tx.Script.ToTxScript()
	if err != nil {
		return err
	}
	//每个输入的脚本执行时间为一半交易时间的平均数
	tv := time.Duration(int(txs.ExeTime/2)/len(sr.tx.Ins)) * time.Millisecond
	//
	l, cancel := initLuaEnv(tv, sr.tx, bi, 0)
	defer cancel()
	defer l.Close()
	err = compileScript(l, wits.Exec, lcks.Exec)
	if err != nil {
		return err
	}
	return execScript(l)
}
