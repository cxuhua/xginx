package xginx

import (
	"context"
	"fmt"

	"github.com/cxuhua/xginx/xlua"
)

//脚本环境设定
//verify_addr() 验证消费地址 与输入地址hash是否一致
//verify_sign() 验证签名是否正确

//脚本类型
const (
	//交易脚本
	ExecTypeTxMain = "TxMain"
	//输\入脚本
	ExecTypeInMain = "InMain"
	//输出脚本
	ExecTypeOutMain = "OutMain"
)

var (
	//DefaultTxScript 默认交易脚本 控制是否能进入区块
	DefaultTxScript = []byte(`return true`)
	//DefaultInputScript 默认输入脚本 控制是否能消费
	DefaultInputScript = []byte(`return true`)
	//DefaultLockedScript 默认锁定脚本 控制消费输出需要的条件
	//验证地址和签名
	DefaultLockedScript = []byte(`return verify_addr() and verify_sign()`)
)

var (
	blockKey  = &BlockIndex{}
	txKey     = &TX{}
	signerKey = &mulsigner{}
)

//返回主链对象
func getEnvBlockIndex(ctx context.Context) *BlockIndex {
	vptr, ok := ctx.Value(blockKey).(*BlockIndex)
	if !ok {
		return nil
	}
	return vptr
}

//返回当前交易对象
func getEnvTx(ctx context.Context) *TX {
	vptr, ok := ctx.Value(txKey).(*TX)
	if !ok {
		return nil
	}
	return vptr
}

//返回签名对象
func getEnvSigner(ctx context.Context) ISigner {
	vptr, ok := ctx.Value(signerKey).(ISigner)
	if !ok {
		return nil
	}
	return vptr
}

//CheckScript 检测脚本是否有错
func CheckScript(codes ...[]byte) error {
	return fmt.Errorf("not imp")
}

//检测输入hash和锁定hash是否一致
func verifyAddr(l xlua.ILuaState) int {
	signer := getEnvSigner(l.Context())
	if signer == nil {
		panic(fmt.Errorf("signer miss"))
	}
	err := signer.VerifyAddr()
	l.PushBool(err == nil)
	return 1
}

//检测签名是否正确
func verifySign(l xlua.ILuaState) int {
	signer := getEnvSigner(l.Context())
	if signer == nil {
		panic(fmt.Errorf("signer miss"))
	}
	err := signer.VerifySign()
	l.PushBool(err == nil)
	return 1
}

//编译脚本
//typ = 0,tx script
//typ = 1,input script
//typ = 2,out script
func compileExecScript(ctx context.Context, limit uint32, name string, typ int, codes ...[]byte) error {
	buf := NewReadWriter()
	for _, vb := range codes {
		_ = buf.WriteFull(vb)
	}
	if *IsDebug {
		LogInfo(string(buf.Bytes()))
	}
	if buf.Len() == 0 {
		return nil
	}
	//获取必须的环境变量
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		return fmt.Errorf("lua env miss blockindex ")
	}
	tx := getEnvTx(ctx)
	if tx == nil {
		return fmt.Errorf("lua env miss tx ")
	}
	attr := 0
	if typ == 1 {
		//输入脚本允许设置数据map传递到输出脚本
		attr = xlua.AttrMapSet
	} else if typ == 2 {
		attr = xlua.AttrMapGet
	} else {
		attr = 0
	}
	lstep, ltime := GetExeLimit(limit)
	l := xlua.NewLuaState(ctx, ltime, attr).SetLimit(lstep)
	//测试模式下可使用标准库
	if *IsDebug {
		l.OpenLibs()
	}
	if typ == 2 {
		l.SetFunc("verify_addr", verifyAddr)
		l.SetFunc("verify_sign", verifySign)
	}
	err := l.Exec(buf.Bytes())
	if err != nil {
		return err
	}
	if l.GetTop() != 1 || !l.IsBool(-1) {
		return fmt.Errorf("return stack error")
	}
	if !l.ToBool(-1) {
		return fmt.Errorf("script result error")
	}
	return nil
}

//ExecScript 返回错误交易不进入区块
//执行之前已经校验了签名
//AddTxs LinkBlk 时会执行这个交易脚本检测
func (tx *TX) ExecScript(bi *BlockIndex) error {
	txs, err := tx.Script.ToTxScript()
	if err != nil {
		return err
	}
	if txs.Exec.Len() == 0 {
		return err
	}
	//附加变量
	ctx := context.Background()
	ctx = context.WithValue(ctx, blockKey, bi)
	ctx = context.WithValue(ctx, txKey, tx)
	//编译脚本
	return compileExecScript(ctx, txs.ExeLimit, ExecTypeTxMain, 0, txs.Exec)
}

//ExecScript 执行签名交易脚本
//执行之前签名已经通过
func (sr *mulsigner) ExecScript(bi *BlockIndex, wits *WitnessScript, lcks *LockedScript) error {
	//脚本肯定存在
	if slen := wits.Exec.Len() + lcks.Exec.Len(); slen == 0 {
		return fmt.Errorf("script size == 0")
	}
	//获取交易脚本的执行限制时间
	txs, err := sr.tx.Script.ToTxScript()
	if err != nil {
		return err
	}
	//附加变量
	ctx := xlua.NewMapContext()
	ctx = context.WithValue(ctx, blockKey, bi)
	ctx = context.WithValue(ctx, txKey, sr.tx)
	ctx = context.WithValue(ctx, signerKey, sr)
	//编译执行输入脚本
	if err := compileExecScript(ctx, txs.ExeLimit, ExecTypeInMain, 1, wits.Exec); err != nil {
		return err
	}
	//编译执行输出脚本
	if err := compileExecScript(ctx, txs.ExeLimit, ExecTypeOutMain, 2, lcks.Exec); err != nil {
		return err
	}
	return nil
}
