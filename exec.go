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

const (
	//OptPushTxPool 当交易进入交易池前
	OptPushTxPool = 1
	//OptAddToBlock 当交易加入区块前
	OptAddToBlock = 2
	//OptPublishTx 发布交易到网络前
	OptPublishTx = 3
)

var (
	//脚本环境设定
	//error = 'msg' 全局错误信息
	//verify_addr() 验证消费地址 与输入地址hash是否一致
	//verify_sign() 验证签名是否正确
	//timestamp('2001-02-03 11:00:00') 返回指定时间的时间戳
	//best_height 当前区块链高度
	//best_time 最高的区块时间
	//sys_time 当前系统时间戳
	//tx_id 相关的交易id
	//tx_ver 交易版本号
	//in_index 签名验证时输入在交易中的索引
	//coin_value 相关的金额
	//coin_pool 金额是否在交易池
	//coin_height 金额所在的区块高度
	//tx_opt 交易脚本操作类型 对应 OptPushTxPool  OptAddToBlock OptPublishTx =0
	//http.post 网络post 暂时不考虑使用
	//http.get 网络get 暂时不考虑使用
	//out_address 输出地址(指定谁消费),最终谁可以消费主要看脚本执行情况
	//in_address 输入地址(谁来消费对应的输出)

	//DebugScript 是否调式脚本
	DebugScript = false
	//DefaultTxScript 默认交易脚本
	DefaultTxScript = []byte(`return true`)
	//DefaultInputScript 默认输入脚本
	DefaultInputScript = []byte(`return true`)
	//DefaultLockedScript 默认锁定脚本
	DefaultLockedScript = []byte(`return verify_addr() and verify_sign()`)
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

//post json到url，返回也必须是json格式
//http_post(url,{a=1,b='aa'}) -> tbl,err
func httpPost(l *lua.LState) int {
	if l.GetTop() != 2 {
		return returnHTTPError(l, errors.New("args num error"))
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
	tbl := l.NewTable()
	tbl.RawSet(lua.LString("post"), l.NewFunction(httpPost))
	tbl.RawSet(lua.LString("get"), l.NewFunction(httpGet))
	l.SetGlobal("http", tbl)
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

//获取错误信息
func getError(l *lua.LState) string {
	v := l.GetGlobal("error")
	return lua.LVAsString(v)
}

//初始化基本函数
func initLuaMethodEnv(l *lua.LState) {
	//获取字符串表示的时间戳
	l.SetGlobal("timestamp", l.NewFunction(unixTimestamp))
}

//检测输入hash和锁定hash是否一致
func verifyAddr(l *lua.LState) int {
	signer := getEnvSinger(l)
	if signer == nil {
		l.RaiseError("checkHash signer nil")
		return 0
	}
	err := signer.VerifyAddr()
	l.Push(lua.LBool(err == nil))
	return 1
}

//检测签名是否正确
func verifySign(l *lua.LState) int {
	signer := getEnvSinger(l)
	if signer == nil {
		l.RaiseError("checkSign signer nil")
		return 0
	}
	err := signer.VerifySign()
	l.Push(lua.LBool(err == nil))
	return 1
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
		//锁定脚本对应地址
		l.SetGlobal("out_address", lua.LString(signer.GetOutAddress()))
		//获取输入消费地址
		l.SetGlobal("in_address", lua.LString(signer.GetInAddress()))
		// 金额信息
		l.SetGlobal("coin_value", lua.LNumber(coin.Value))
		l.SetGlobal("coin_pool", lua.LBool(coin.IsPool()))
		l.SetGlobal("coin_height", lua.LNumber(coin.Height))
		//验证函数 如果hash一致返回true
		l.SetGlobal("verify_addr", l.NewFunction(verifyAddr))
		//签名正确返回 true
		l.SetGlobal("verify_sign", l.NewFunction(verifySign))
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
	if buf.Len() == 0 {
		return nil
	}
	fn, err := l.Load(buf, "<"+name+">")
	if err != nil {
		return err
	}
	l.Push(fn)
	if err := l.PCall(0, 1, nil); err != nil {
		return fmt.Errorf("call script error %w", err)
	} else if result := l.Get(-1); result.Type() != lua.LTBool {
		return fmt.Errorf("script result type error")
	} else if bok := lua.LVAsBool(result); !bok {
		return fmt.Errorf("script result error : %s", getError(l))
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
		//无脚本不执行
		return nil
	} else if slen > MaxExecSize {
		//脚本不能太大
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
		//无脚本不执行
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
	//消费地址hash，锁定
	//编译输入脚本 执行错误返回
	err = compileExecScript(l, "input_main", wits.Exec)
	if err != nil {
		return err
	}
	//编译输出脚本
	return compileExecScript(l, "out_main", lcks.Exec)
}
