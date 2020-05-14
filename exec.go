package xginx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	jsoniter "github.com/json-iterator/go"
	lua "github.com/yuin/gopher-lua"
)

//如何时候脚本返回 如果不是ExecOK代表执行失败
const (
	//OptPushTxPool 当交易进入交易池前
	OptPushTxPool = 1
	//OptAddToBlock 当交易加入区块前
	OptAddToBlock = 2
	//OptPublishTx 发布交易到网络前
	OptPublishTx = 3
	//ExecOK 执行成功返回，脚本总要返回一个字符串
	ExecOK = "OK"
	//ExecErr 错误常量,不确定错误时返回
	ExecErr = "ERR"
)

var (
	//DebugScript 是否调式脚本
	DebugScript = false
	//SuccessScript 成功脚本
	SuccessScript = []byte("return ExecOK;")
)

//返回错误
func returnHTTPError(l *lua.LState, err error) int {
	l.Push(lua.LNil)
	l.Push(lua.LString(err.Error()))
	return 2
}

//设置一个值
func setAnyValue(l *lua.LState, key string, v jsoniter.Any, tbl *lua.LTable) {
	if typ := v.ValueType(); typ == jsoniter.BoolValue {
		if key != "" {
			tbl.RawSetString(key, lua.LBool(v.ToBool()))
		} else {
			tbl.Append(lua.LBool(v.ToBool()))
		}
	} else if typ == jsoniter.NilValue {
		if key != "" {
			tbl.RawSetString(key, lua.LNil)
		} else {
			tbl.Append(lua.LNil)
		}
	} else if typ == jsoniter.StringValue {
		if key != "" {
			tbl.RawSetString(key, lua.LString(v.ToString()))
		} else {
			tbl.Append(lua.LString(v.ToString()))
		}
	} else if typ == jsoniter.NumberValue {
		if key != "" {
			tbl.RawSetString(key, lua.LNumber(v.ToFloat64()))
		} else {
			tbl.Append(lua.LNumber(v.ToFloat64()))
		}
	} else if typ == jsoniter.ArrayValue {
		ntbl := l.NewTable()
		setArrayTable(l, v, ntbl)
		if key != "" {
			tbl.RawSetString(key, ntbl)
		} else {
			tbl.Append(ntbl)
		}
	} else if typ == jsoniter.ObjectValue {
		ntbl := l.NewTable()
		setObjectTable(l, v, ntbl)
		if key != "" {
			tbl.RawSetString(key, ntbl)
		} else {
			tbl.Append(ntbl)
		}
	} else {
		LogErrorf("json type %d not process", typ)
	}
}

//设置对象表格数据
func setObjectTable(l *lua.LState, any jsoniter.Any, tbl *lua.LTable) {
	for _, key := range any.Keys() {
		v := any.Get(key)
		setAnyValue(l, key, v, tbl)
	}
}

//设置数组表格数据
func setArrayTable(l *lua.LState, any jsoniter.Any, tbl *lua.LTable) {
	for i := 0; i < any.Size(); i++ {
		setAnyValue(l, "", any.Get(i), tbl)
	}
}

//对象转换为table
func objectToTable(l *lua.LState, v interface{}) (*lua.LTable, error) {
	jv, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return jsonToTable(l, jv)
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

//如果table是一个非空数组
func tableIsArray(tbl *lua.LTable) bool {
	max := tbl.MaxN()
	return max > 0 && max == tbl.Len()
}

//获取
func getTableValue(v lua.LValue) interface{} {
	typ := v.Type()
	if typ == lua.LTBool {
		return lua.LVAsBool(v)
	} else if typ == lua.LTString {
		return lua.LVAsString(v)
	} else if typ == lua.LTNumber {
		return lua.LVAsNumber(v)
	} else if typ == lua.LTTable {
		return getTableJSON(v.(*lua.LTable))
	} else {
		return nil
	}
}

//转换tbl到数组
func getTableJSON(tbl *lua.LTable) interface{} {
	if tableIsArray(tbl) {
		arr := []interface{}{}
		tbl.ForEach(func(k, v lua.LValue) {
			vv := getTableValue(v)
			if vv == nil {
				return
			}
			arr = append(arr, vv)
		})
		return arr
	}
	arr := map[string]interface{}{}
	tbl.ForEach(func(k, v lua.LValue) {
		if k.Type() != lua.LTString {
			return
		}
		kk := lua.LVAsString(k)
		vv := getTableValue(v)
		if vv == nil {
			return
		}
		arr[kk] = vv
	})
	return arr
}

//table转换为json数据
func tableToJSON(tbl *lua.LTable) ([]byte, error) {
	arr := getTableJSON(tbl)
	return json.Marshal(arr)
}

//http_post(url,{a=1,b='aa'}) -> tbl,err
func httpPost(l *lua.LState) int {
	if l.GetTop() != 2 {
		return returnHTTPError(l, errors.New("args error"))
	}
	path := l.ToString(1)
	_, err := url.Parse(path)
	if err != nil {
		return returnHTTPError(l, err)
	}
	tbl := l.Get(2)
	if tbl.Type() != lua.LTTable {
		return returnHTTPError(l, errors.New("args 2 type error"))
	}
	jv, err := tableToJSON(tbl.(*lua.LTable))
	if err != nil {
		return returnHTTPError(l, err)
	}
	ctx := l.Context()
	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(jv))
	if err != nil {
		return returnHTTPError(l, err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	client := http.Client{}
	res, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return returnHTTPError(l, err)
	}
	defer res.Body.Close()
	dat, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return returnHTTPError(l, err)
	}
	tbl, err = jsonToTable(l, dat)
	if err != nil {
		return returnHTTPError(l, err)
	}
	l.Push(tbl)
	l.Push(lua.LNil)
	return 2
}

//http_get(url) -> tbl,err
func httpGet(l *lua.LState) int {
	if l.GetTop() != 1 {
		return returnHTTPError(l, errors.New("args error"))
	}
	path := l.ToString(1)
	_, err := url.Parse(path)
	if err != nil {
		return returnHTTPError(l, err)
	}
	ctx := l.Context()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return returnHTTPError(l, err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	client := http.Client{}
	res, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return returnHTTPError(l, err)
	}
	defer res.Body.Close()
	dat, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return returnHTTPError(l, err)
	}
	tbl, err := jsonToTable(l, dat)
	if err != nil {
		return returnHTTPError(l, err)
	}
	l.Push(tbl)
	l.Push(lua.LNil)
	return 2
}

//初始化http库支持方法
func initHTTPLuaEnv(l *lua.LState) {
	mod := l.NewTable()
	mod.RawSet(lua.LString("post"), l.NewFunction(httpPost))
	mod.RawSet(lua.LString("get"), l.NewFunction(httpGet))
	l.SetGlobal("http", mod)
}

var (
	blockKey  = struct{}{}
	txKey     = struct{}{}
	signerKey = struct{}{}
)

//返回主链对象
func getEnvBlockIndex(l *lua.LState) *BlockIndex {
	vptr, ok := l.Context().Value(blockKey).(*BlockIndex)
	if !ok {
		return nil
	}
	return vptr
}

//返回当前交易对象
func getEnvTx(l *lua.LState) *TX {
	vptr, ok := l.Context().Value(txKey).(*TX)
	if !ok {
		return nil
	}
	return vptr
}

//返回签名对象
func getEnvSinger(l *lua.LState) ISigner {
	vptr, ok := l.Context().Value(signerKey).(ISigner)
	if !ok {
		return nil
	}
	return vptr
}

//CheckScript 检测脚本是否有错
func CheckScript(codes ...[]byte) error {
	opts := lua.Options{
		CallStackSize:       64,
		MinimizeStackMemory: true,
		SkipOpenLibs:        !DebugScript,
	}
	l := lua.NewState(opts)
	defer l.Close()
	buf := NewReadWriter()
	for _, vb := range codes {
		buf.WriteFull(vb)
	}
	_, err := l.Load(buf, "<main>")
	if err != nil {
		return fmt.Errorf("check script error %w", err)
	}
	return err
}

//转换时间戳
//Timestamp('2006-01-02 15:04:05')
func unixTimestamp(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.RaiseError("args error")
		return 0
	}
	sfmt := "2006-01-02 15:04:05"
	str := l.ToString(1)
	if len(str) < len(sfmt) {
		sfmt = "2006-01-02"
	}
	tv, err := time.ParseInLocation(sfmt, str, time.Local)
	if err != nil {
		l.RaiseError(err.Error())
		return 0
	}
	l.Push(lua.LNumber(tv.Unix()))
	return 1
}

//初始化基本函数
func initLuaMethodEnv(l *lua.LState) {
	//获取字符串表示的时间戳
	l.SetGlobal("Timestamp", l.NewFunction(unixTimestamp))
}

//初始化脚本状态机
func initLuaEnv(exectime time.Duration, tx *TX, bi *BlockIndex, signer ISigner, opt int) (*lua.LState, context.CancelFunc) {
	opts := lua.Options{
		CallStackSize:       64,
		MinimizeStackMemory: true,
		SkipOpenLibs:        !DebugScript,
	}
	ctx, cancel := context.WithTimeout(context.Background(), exectime)
	l := lua.NewState(opts)
	//成功常量
	l.SetGlobal("ExecOK", lua.LString(ExecOK))
	l.SetGlobal("ExecErr", lua.LString(ExecErr))
	//操作常量
	l.SetGlobal("OptPushTxPool", lua.LNumber(OptPushTxPool))
	l.SetGlobal("OptAddToBlock", lua.LNumber(OptAddToBlock))
	l.SetGlobal("OptPublishTx", lua.LNumber(OptPublishTx))
	//可用方法
	initLuaMethodEnv(l)
	//
	if bi != nil {
		ctx = context.WithValue(ctx, blockKey, bi)
		//当前区块高度和区块时间
		l.SetGlobal("best_height", lua.LNumber(bi.Height()))
		l.SetGlobal("best_time", lua.LNumber(bi.Time()))
		//当前系统时间戳
		l.SetGlobal("sys_time", lua.LNumber(bi.lptr.TimeNow()))
	}
	if tx != nil {
		ctx = context.WithValue(ctx, txKey, tx)
		//交易id
		l.SetGlobal("tx_id", lua.LString(tx.MustID().String()))
		//交易版本
		l.SetGlobal("tx_ver", lua.LNumber(tx.Ver))
	}
	if signer != nil {
		ctx = context.WithValue(ctx, signerKey, signer)
		//获得签名对象
		_, in, out, idx := signer.GetObjs()
		//输入在交易中的索引 0开始
		l.SetGlobal("in_index", lua.LNumber(idx))
		//获取输出金额信息
		coin, err := out.GetCoin(in, bi)
		//签名后执行这里应该不会出错
		if err != nil {
			panic(err)
		}
		// 金额信息
		l.SetGlobal("coin_value", lua.LNumber(coin.Value))
		l.SetGlobal("coin_pool", lua.LBool(coin.IsPool()))
		l.SetGlobal("coin_height", lua.LNumber(coin.Height))
	}
	//交易操作
	l.SetGlobal("tx_opt", lua.LNumber(opt))
	l.SetContext(ctx)
	return l, cancel
}

//编译脚本
func compileExecScript(l *lua.LState, name string, codes ...[]byte) error {
	buf := NewReadWriter()
	for _, vb := range codes {
		buf.WriteFull(vb)
	}
	if DebugScript {
		LogInfo(string(buf.Bytes()))
	}
	fn, err := l.Load(buf, "<"+name+">")
	if err != nil {
		return err
	}
	l.Push(fn)
	if err := l.PCall(0, 1, nil); err != nil {
		return err
	} else if result := l.Get(-1); result.Type() != lua.LTString {
		return fmt.Errorf("script result type error")
	} else if str := lua.LVAsString(result); str != ExecOK {
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
	if slen := txs.Exec.Len(); slen == 0 {
		return nil
	} else if slen > MaxExecSize {
		return fmt.Errorf("tx exec script too big , size = %d", slen)
	}
	//交易脚本执行时间为cpu/2
	exectime := time.Duration(txs.ExeTime/2) * time.Millisecond
	//初始化执行环境
	l, cancel := initLuaEnv(exectime, &tx, bi, nil, opt)
	defer cancel()
	defer l.Close()
	//编译脚本
	return compileExecScript(l, "tx_main", txs.Exec)
}

//ExecScript 执行签名交易脚本
//执行之前签名已经通过
func (sr *mulsigner) ExecScript(bi *BlockIndex, wits WitnessScript, lcks LockedScript) error {
	//如果未设置就不执行了
	if slen := wits.Exec.Len() + lcks.Exec.Len(); slen == 0 {
		return nil
	} else if slen > MaxExecSize {
		//脚本不能太大
		return fmt.Errorf("witness exec script + locked exec script size too big ,size = %d", slen)
	}
	txs, err := sr.tx.Script.ToTxScript()
	if err != nil {
		return err
	}
	//每个输入的脚本执行时间为一半交易时间的平均数
	exectime := time.Duration(int(txs.ExeTime/2)/len(sr.tx.Ins)) * time.Millisecond
	//初始化执行环境
	l, cancel := initLuaEnv(exectime, sr.tx, bi, sr, 0)
	defer cancel()
	defer l.Close()
	//编译输入脚本 执行错误返回
	err = compileExecScript(l, "input_main", wits.Exec)
	if err != nil {
		return err
	}
	//编译输出脚本
	return compileExecScript(l, "out_main", lcks.Exec)
}
