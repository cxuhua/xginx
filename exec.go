package xginx

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	jsoniter "github.com/json-iterator/go"
	lua "github.com/yuin/gopher-lua"
)

const (
	//OptPushTxPool 当交易进入交易池
	OptPushTxPool = 1
	//OptAddToBlock 当交易加入区块
	OptAddToBlock = 2
	//OptPublishTx 发布交易到网络
	OptPublishTx = 3
	//执行成功返回，脚本总要返回一个字符串
	ExecOK = "OK"
	//错误常量,不确定错误时返回
	ExecErr = "ERROR"
)

var (
	//是否调式脚本
	DebugScript = false
	//成功脚本
	SuccessScript = []byte("return 'OK';")
)

//返回错误
func returnHttpError(l *lua.LState, err error) int {
	l.Push(lua.LNil)
	l.Push(lua.LString(err.Error()))
	return 2
}

//设置一个值
func setAnyValue(l *lua.LState, key string, idx int, v jsoniter.Any, tbl *lua.LTable) {
	if typ := v.ValueType(); typ == jsoniter.BoolValue {
		if key != "" {
			tbl.RawSetString(key, lua.LBool(v.ToBool()))
		} else {
			tbl.RawSetInt(idx, lua.LBool(v.ToBool()))
		}
	} else if typ == jsoniter.NilValue {
		if key != "" {
			tbl.RawSetString(key, lua.LNil)
		} else {
			tbl.RawSetInt(idx, lua.LNil)
		}
	} else if typ == jsoniter.StringValue {
		if key != "" {
			tbl.RawSetString(key, lua.LString(v.ToString()))
		} else {
			tbl.RawSetInt(idx, lua.LString(v.ToString()))
		}
	} else if typ == jsoniter.NumberValue {
		if key != "" {
			tbl.RawSetString(key, lua.LNumber(v.ToFloat64()))
		} else {
			tbl.RawSetInt(idx, lua.LNumber(v.ToFloat64()))
		}
	} else if typ == jsoniter.ArrayValue {
		ntbl := l.NewTable()
		setArrayTable(l, v, ntbl)
		if key != "" {
			tbl.RawSetString(key, ntbl)
		} else {
			tbl.RawSetInt(idx, ntbl)
		}
	} else if typ == jsoniter.ObjectValue {
		ntbl := l.NewTable()
		setObjectTable(l, v, ntbl)
		if key != "" {
			tbl.RawSetString(key, ntbl)
		} else {
			tbl.RawSetInt(idx, ntbl)
		}
	} else {
		panic(fmt.Errorf("json type %d not process", typ))
	}
}

//设置对象表格数据
func setObjectTable(l *lua.LState, any jsoniter.Any, tbl *lua.LTable) {
	for _, key := range any.Keys() {
		v := any.Get(key)
		setAnyValue(l, key, 0, v, tbl)
	}
}

//设置数组表格数据
func setArrayTable(l *lua.LState, any jsoniter.Any, tbl *lua.LTable) {
	for i := 0; i < any.Size(); i++ {
		v := any.Get(i)
		setAnyValue(l, "", i, v, tbl)
	}
}

//json转换为lua table
func jsonToTable(l *lua.LState, jv []byte) (*lua.LTable, error) {
	any := jsoniter.Get(jv)
	tbl := l.NewTable()
	if typ := any.ValueType(); typ == jsoniter.ObjectValue {
		setObjectTable(l, any, tbl)
	} else if typ == jsoniter.ArrayValue {
		setArrayTable(l, any, tbl)
	} else {
		return nil, fmt.Errorf("json type %d not support", typ)
	}
	return tbl, nil
}

//table转换为json数据
func tableToJsoin(l *lua.LState, tbl *lua.LTable) ([]byte, error) {
	return nil, nil
}

//http_post(url,{a=1,b='aa'}) -> tbl,err
func httpPost(l *lua.LState) int {
	ctx := l.Context()
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.WithContext(ctx)
	return 2
}

//http_get(url) -> tbl,err
func httpGet(l *lua.LState) int {
	path := l.ToString(1)
	_, err := url.Parse(path)
	if err != nil {
		return returnHttpError(l, err)
	}
	ctx := l.Context()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return returnHttpError(l, err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	client := http.Client{}
	res, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return returnHttpError(l, err)
	}
	defer res.Body.Close()
	dat, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return returnHttpError(l, err)
	}
	l.Push(lua.LString(dat))
	l.Push(lua.LNil)
	return 2
}

//初始化http库支持方法
func initHttpLuaEnv(l *lua.LState) {
	mod := l.NewTable()
	mod.RawSet(lua.LString("post"), l.NewFunction(httpPost))
	mod.RawSet(lua.LString("get"), l.NewFunction(httpGet))
	l.SetGlobal("http", mod)
}

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
	//成功常量
	l.SetGlobal("ExecOK", lua.LString(ExecOK))
	l.SetGlobal("ExecErr", lua.LString(ExecErr))
	//操作常量
	l.SetGlobal("OptPushTxPool", lua.LNumber(OptPushTxPool))
	l.SetGlobal("OptAddToBlock", lua.LNumber(OptAddToBlock))
	l.SetGlobal("OptPublishTx", lua.LNumber(OptPublishTx))
	//交易id
	id := tx.MustID()
	l.SetGlobal("tx_id", lua.LString(id.String()))
	//交易版本
	l.SetGlobal("tx_ver", lua.LNumber(tx.Ver))
	//当前区块高度和区块生成时间
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
	if err := l.PCall(0, 1, nil); err != nil {
		return err
	} else if result := l.Get(-1); result.Type() != lua.LTString {
		return fmt.Errorf("script result type error")
	} else if str := lua.LVAsString(result); str != ExecOK {
		return fmt.Errorf("script result error %s", str)
	} else {
		return nil
	}
}

//ExecScript 返回错误会不加入交易池或者不进入区块
//执行之前已经校验了签名
func (tx TX) ExecScript(bi *BlockIndex, opt int) error {
	txs, err := tx.Script.ToTxScript()
	if err != nil {
		return err
	}
	if l := txs.Exec.Len(); l == 0 {
		return nil
	} else if l > MaxExecSize {
		return fmt.Errorf("tx exec script too big size = %d", l)
	}
	//交易脚本执行时间为cpu/2
	tv := time.Duration(txs.ExeTime/2) * time.Millisecond
	//
	l, cancel := initLuaEnv(tv, &tx, bi, opt)
	defer cancel()
	defer l.Close()
	//
	//编译脚本
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
	if l := wits.Exec.Len() + lcks.Exec.Len(); l == 0 {
		return nil
	} else if l > MaxExecSize*2 {
		//脚本不能太大
		return fmt.Errorf("witness exec script + locked exec script size too big size = %d", l)
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
	//签名用常量
	//输出金额
	l.SetGlobal("out_value", lua.LNumber(sr.out.Value))
	//编译脚本
	err = compileScript(l, wits.Exec, lcks.Exec)
	if err != nil {
		return err
	}
	return execScript(l)
}
