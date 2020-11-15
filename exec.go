package xginx

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"

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

//获取当前时间戳
func timestamp(l xlua.ILuaState) int {
	bi := getEnvBlockIndex(l.Context())
	if bi == nil {
		panic(fmt.Errorf("bi miss"))
	}
	v := bi.lptr.TimeNow()
	l.PushInt(int64(v))
	return 1
}

//设置脚本
func setScript(l xlua.ILuaState, script Script) {
	tbl := l.NewTable()
	//脚本长度
	tbl.Set("size", len(script))
	typ := script.GetType()
	tbl.Set("type", typ)
	tbl.Set("raw", hex.EncodeToString(script))
	switch typ {
	case ScriptTxType:
		s, err := script.ToTxScript()
		if err != nil {
			panic(err)
		}
		obj := l.NewTable()
		obj.Set("exec", s.Exec.String())
		obj.Set("limit", s.ExeLimit)
		l.SetField(-2, "tx")
	case ScriptWitnessType:
		s, err := script.ToWitness()
		if err != nil {
			panic(err)
		}
		obj := l.NewTable()
		obj.Set("exec", s.Exec.String())
		obj.Set("num", s.Num)
		obj.Set("less", s.Less)
		obj.Set("arb", s.Arb)
		obj.Set("address", string(s.Address()))
		l.SetField(-2, "witness")
	case ScriptLockedType:
		s, err := script.ToLocked()
		if err != nil {
			panic(err)
		}
		obj := l.NewTable()
		obj.Set("exec", s.Exec.String())
		obj.Set("meta", s.Meta.String())
		obj.Set("address", string(s.Address()))
		l.SetField(-2, "locked")
	case ScriptCoinbaseType:
		obj := l.NewTable()
		obj.Set("height", script.Height())
		obj.Set("ip", net.IP(script.IP()).String())
		obj.Set("data", hex.EncodeToString(script.Data()))
		l.SetField(-2, "coinbase")
	default:
		panic(fmt.Errorf("script type error"))
	}
	l.SetField(-2, "script")
}

//设置输入信息
func setTxIn(l xlua.ILuaState, in *TxIn) {
	tbl := l.NewTable()
	tbl.Set("out_hash", in.OutHash.String())
	tbl.Set("out_index", in.OutIndex.ToUInt32())
	tbl.Set("seq", in.Sequence.ToUInt32())
	setScript(l, in.Script)
}

//设置输出信息
func setTxOut(l xlua.ILuaState, out *TxOut) {
	tbl := l.NewTable()
	tbl.Set("value", int64(out.Value))
	setScript(l, out.Script)
}

//设置交易信息到栈顶
func setTxInfo(l xlua.ILuaState, tx *TX) int {
	tbl := l.NewTable()
	//版本
	tbl.Set("ver", tx.Ver.ToInt())
	tbl.Set("id", tx.MustID().String())
	//输入
	l.NewTable()
	for i, in := range tx.Ins {
		l.PushInt(int64(i + 1))
		setTxIn(l, in)
		l.SetTable(-3)
	}
	l.SetField(-2, "ins")
	//输出
	l.NewTable()
	for i, out := range tx.Outs {
		l.PushInt(int64(i + 1))
		setTxOut(l, out)
		l.SetTable(-3)
	}
	l.SetField(-2, "outs")
	//设置脚本信息
	setScript(l, tx.Script)
	return 1
}

//获取当前交易信息(0 1 2可用)
func txInfo(l xlua.ILuaState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		panic(fmt.Errorf("bi miss"))
	}
	tx := getEnvTx(ctx)
	if tx == nil {
		panic(fmt.Errorf("tx miss"))
	}
	return setTxInfo(l, tx)
}

//获取当前输入信息,返回4个信息,交易,当前输入,当前输出,输入索引
func getSigner(l xlua.ILuaState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		panic(fmt.Errorf("bi miss"))
	}
	signer := getEnvSigner(ctx)
	if signer == nil {
		panic(fmt.Errorf("signer miss"))
	}
	tx, in, out, idx := signer.GetObjs()
	setTxInfo(l, tx)
	setTxIn(l, in)
	setTxOut(l, out)
	l.PushInt(int64(idx))
	return 4
}

//获取当前输入引用的交易
func getOutTx(l xlua.ILuaState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		panic(fmt.Errorf("bi miss"))
	}
	signer := getEnvSigner(ctx)
	if signer == nil {
		panic(fmt.Errorf("signer miss"))
	}
	_, in, _, _ := signer.GetObjs()
	tx, err := bi.LoadTX(in.OutHash)
	if err != nil {
		panic(err)
	}
	return setTxInfo(l, tx)
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
	attr := 0
	if typ == 1 {
		//输入脚本允许设置数据map传递到输出脚本
		attr = xlua.AttrMapSet
	} else if typ == 2 {
		attr = xlua.AttrMapGet
	} else {
		attr = 0
	}
	//时间执行和步数限制
	lstep, ltime := GetExeLimit(limit)
	l := xlua.NewLuaState(ctx, ltime, attr).SetLimit(lstep)
	//测试模式下可使用标准库
	if *IsDebug {
		l.OpenLibs()
	}
	//通用属性
	l.SetGlobalValue("ScriptCoinbaseType", ScriptCoinbaseType)
	l.SetGlobalValue("ScriptLockedType", ScriptLockedType)
	l.SetGlobalValue("ScriptWitnessType", ScriptWitnessType)
	l.SetGlobalValue("ScriptTxType", ScriptTxType)
	//通用方法
	l.SetFunc("timestamp", timestamp) //获取当前时间
	if typ == 1 {
		//执行输入脚本
		l.SetFunc("get_signer", getSigner) //获取当前交签名器
		l.SetFunc("get_outtx", getOutTx)   //获取引用所在的交易
	} else if typ == 2 {
		//执行输出脚本
		l.SetFunc("get_signer", getSigner)   //获取当前交签名器
		l.SetFunc("get_outtx", getOutTx)     //获取引用所在的交易
		l.SetFunc("verify_addr", verifyAddr) //校验地址
		l.SetFunc("verify_sign", verifySign) //校验签名
	} else {
		//执行交易脚本
		l.SetFunc("tx_info", txInfo) //获取当前交易信息
	}
	err := l.Exec(buf.Bytes())
	if err != nil {
		log.Println(err)
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
