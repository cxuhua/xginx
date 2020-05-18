package xginx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"time"

	lua "github.com/cxuhua/gopher-lua"
	jsoniter "github.com/json-iterator/go"
)

//交易脚本可用
const (
	//OptPushTxPool 当交易进入交易池前
	OptPushTxPool = 1
	//OptAddToBlock 当交易加入区块前
	OptAddToBlock = 2
	//OptPublishTx 发布交易到网络前
	OptPublishTx = 3
)

//脚本环境设定
//verify_addr() 验证消费地址 与输入地址hash是否一致
//verify_sign() 验证签名是否正确
//timestamp('2001-02-03 11:00:00') 返回指定时间的时间戳,无参数获取当前时间 = sys_time
//best_height 当前区块链高度
//best_time 最高的区块时间
//median_time(h) h高度开始，向前最近11个区块的中间时间,如果参数不存在获取最新的11个区块的中间时间
//sys_time 当前系统时间戳
//tx_id 相关的交易id
//tx_block() 交易所在的区块，返回一个table, b.height b.time
//tx_ver 交易版本号
//in_index 签名验证时输入在交易中的索引
//in_size 输入数量
//out_block() 引用的输出所在区块，返回table b.height b.time
//coin_value 相关的金额
//coin_pool 金额是否在交易池
//coin_height 金额所在的区块高度
//out_size 交易中的输出数量
//tx_opt 交易脚本操作类型 对应 OptPushTxPool  OptAddToBlock OptPublishTx =0
//http_post 网络post 交易脚本可用 如果配置中启用了
//http_get 网络get 交易脚本可用 如果配置中启用了
//out_address 输出地址(指定谁消费),最终谁可以消费主要看脚本执行情况
//in_address 输入地址(谁来消费对应的输出)
//map_set 输入脚本中设置一个值，在输出脚本中可以用map_get获取到
//map_get 获取输入脚本中设置的值

var (

	//DefaultTxScript 默认交易脚本
	DefaultTxScript = []byte(`return true`)

	//DefaultInputScript 默认输入脚本
	DefaultInputScript = []byte(`return true`)

	//DefaultLockedScript 默认锁定脚本
	//验证地址和签名
	DefaultLockedScript = []byte(`return verify_addr() and verify_sign()`)
)

//创建执行环境
func newScriptEnv(ctx context.Context) *lua.LState {
	opts := lua.Options{
		CallStackSize:   16,
		RegistrySize:    128,
		RegistryMaxSize: 0,
		SkipOpenLibs:    !*IsDebug,
	}
	l := lua.NewState(opts)
	l.SetContext(ctx)
	return l
}

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
	l.SetGlobal("http_post", l.NewFunction(httpPost))
	l.SetGlobal("http_get", l.NewFunction(httpGet))
}

//用于将输入数据传递到输出
//只支持四种类似 int float bool string
type transOutMap struct {
	ctx context.Context
	kvs map[string]interface{}
}

func (tm *transOutMap) getValue(k string) (interface{}, bool) {
	v, b := tm.kvs[k]
	return v, b
}

func (tm *transOutMap) setInt(k string, v int64) {
	if len(tm.kvs) >= mapMaxSize {
		panic(errors.New("trans map element too many > mapMaxSize"))
	}
	tm.kvs[k] = v
}

func (tm *transOutMap) setFloat(k string, v float64) {
	if len(tm.kvs) >= mapMaxSize {
		panic(errors.New("trans map element too many > mapMaxSize"))
	}
	tm.kvs[k] = v
}

func (tm *transOutMap) setString(k string, v string) {
	if len(tm.kvs) >= mapMaxSize {
		panic(errors.New("trans map element too many > mapMaxSize"))
	}
	tm.kvs[k] = v
}

func (tm *transOutMap) setBool(k string, v bool) {
	if len(tm.kvs) >= mapMaxSize {
		panic(errors.New("trans map element too many > mapMaxSize"))
	}
	tm.kvs[k] = v
}

func newTransOutMap(ctx context.Context) *transOutMap {
	return &transOutMap{ctx: ctx, kvs: map[string]interface{}{}}
}

var (
	blockKey   = &BlockIndex{}
	txKey      = &TX{}
	signerKey  = &mulsigner{}
	transKey   = &transOutMap{}
	mapMaxSize = 32
)

//返回map
func getEnvTransMap(ctx context.Context) *transOutMap {
	vptr, ok := ctx.Value(transKey).(*transOutMap)
	if !ok {
		return nil
	}
	return vptr
}

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
	l := newScriptEnv(context.Background())
	defer l.Close()
	buf := NewReadWriter()
	for _, vb := range codes {
		buf.WriteFull(vb)
	}
	if buf.Len() > MaxExecSize {
		return fmt.Errorf("script %s ,too big > %d", string(buf.Bytes()), MaxExecSize)
	}
	_, err := l.Load(buf, "<main>")
	if err != nil {
		return fmt.Errorf("check script error, %w", err)
	}
	return err
}

//转换时间戳
//timestamp('2006-01-02 15:04:05')
func unixTimestamp(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.Push(lua.LNumber(time.Now().Unix()))
		return 1
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

//如果无参数就获取最近11个区块的中间时间
//median_time() median_time(10)
func blockMedianTime(l *lua.LState) int {
	bi := getEnvBlockIndex(l.Context())
	if bi == nil {
		l.RaiseError("block index env miss")
		return 0
	}
	h := bi.Height()
	if l.GetTop() == 1 {
		h = uint32(l.ToInt(1))
	}
	t := bi.GetMedianTime(h)
	l.Push(lua.LNumber(t))
	return 1
}

//初始化基本函数
func initLuaMethodEnv(l *lua.LState, typ int) {
	//获取字符串表示的时间戳
	l.SetGlobal("timestamp", l.NewFunction(unixTimestamp))
	//获取最近11个区块中间时间，如果指定了参数就从指定区块向前推送
	l.SetGlobal("median_time", l.NewFunction(blockMedianTime))
}

//检测输入hash和锁定hash是否一致
func verifyAddr(l *lua.LState) int {
	signer := getEnvSigner(l.Context())
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
	signer := getEnvSigner(l.Context())
	if signer == nil {
		l.RaiseError("checkSign signer nil")
		return 0
	}
	err := signer.VerifySign()
	l.Push(lua.LBool(err == nil))
	return 1
}

//如果是整形返回整形，true
func luaNumberIsInt(v lua.LNumber) (int64, bool) {
	i, b := math.Modf(float64(v))
	return int64(i), b == 0
}

//初始化传递api口,只用于输入脚本
//map_set(k, v)
func transMapValueSet(l *lua.LState) int {
	if l.GetTop() != 2 {
		l.RaiseError("args num error")
		return 0
	}
	k := l.Get(1)
	if k.Type() != lua.LTString {
		l.RaiseError("args 1 type error")
		return 0
	}
	key := lua.LVAsString(k)
	if key == "" {
		l.RaiseError("args 1 empty error")
		return 0
	}
	tmap := getEnvTransMap(l.Context())
	if tmap == nil {
		l.RaiseError("trans map miss")
		return 0
	}
	v := l.Get(2)
	typ := v.Type()
	if typ == lua.LTNumber {
		val := lua.LVAsNumber(v)
		iv, ok := luaNumberIsInt(val)
		if ok {
			tmap.setInt(key, iv)
		} else {
			tmap.setFloat(key, float64(val))
		}
	} else if typ == lua.LTBool {
		tmap.setBool(key, lua.LVAsBool(v))
	} else if typ == lua.LTString {
		tmap.setString(key, lua.LVAsString(v))
	} else {
		l.RaiseError("args 2 type error")
		return 0
	}
	return 0
}

//初始化传递api口,只用于输出脚本
//map_get(k) -> v
func transMapValueGet(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.RaiseError("args num error")
		return 0
	}
	k := l.Get(1)
	if k.Type() != lua.LTString {
		l.RaiseError("args 1 type error")
		return 0
	}
	key := lua.LVAsString(k)
	if key == "" {
		l.RaiseError("args 1 empty error")
		return 0
	}
	tmap := getEnvTransMap(l.Context())
	if tmap == nil {
		l.RaiseError("trans map miss")
		return 0
	}
	v, b := tmap.getValue(key)
	if !b {
		l.Push(lua.LNil)
	} else {
		switch v.(type) {
		case int64:
			l.Push(lua.LNumber(v.(int64)))
		case float64:
			l.Push(lua.LNumber(v.(float64)))
		case bool:
			l.Push(lua.LBool(v.(bool)))
		case string:
			l.Push(lua.LString(v.(string)))
		default:
			l.Push(lua.LNil)
		}
	}
	l.Push(lua.LBool(b))
	return 2
}

//设置属性字段
func setBlockTable(l *lua.LState, tbl *lua.LTable, bi *BlockIndex, id HASH256) {
	//查询交易所在的区块
	v, err := bi.LoadTxValue(id)
	//如果查找不到使用下个区块高度和当前时间
	if err != nil {
		tbl.RawSetString("height", lua.LNumber(bi.NextHeight()))
		tbl.RawSetString("time", lua.LNumber(bi.lptr.TimeNow()))
	} else if blk, err := bi.LoadBlock(v.BlkID); err != nil {
		tbl.RawSetString("height", lua.LNumber(bi.NextHeight()))
		tbl.RawSetString("time", lua.LNumber(bi.lptr.TimeNow()))
	} else {
		tbl.RawSetString("height", lua.LNumber(blk.Meta.Height))
		tbl.RawSetString("time", lua.LNumber(blk.Meta.Time))
	}
}

//获取交易所在的区块信息
func txBlockMethod(l *lua.LState) int {
	ctx := l.Context()
	signer := getEnvSigner(ctx)
	if signer == nil {
		l.RaiseError("signer miss")
		return 0
	}
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
		return 0
	}
	tbl := l.NewTable()
	id := signer.GetTxID()
	setBlockTable(l, tbl, bi, id)
	l.Push(tbl)
	return 1
}

//获取输出所在的区块信息
func outblockMethod(l *lua.LState) int {
	ctx := l.Context()
	signer := getEnvSigner(ctx)
	if signer == nil {
		l.RaiseError("signer miss")
		return 0
	}
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
		return 0
	}
	//输出所在交易就是输入的outhash对应的交易
	_, in, _, _ := signer.GetObjs()
	tbl := l.NewTable()
	setBlockTable(l, tbl, bi, in.OutHash)
	l.Push(tbl)
	return 1
}

//初始化交易可用方法
func initLuaTxMethod(l *lua.LState) {
	l.SetGlobal("tx_block", l.NewFunction(txBlockMethod))
	l.SetGlobal("out_block", l.NewFunction(outblockMethod))
}

//编译脚本
//typ = 0,tx script
//typ = 1,input script
//typ = 2,out script
func compileExecScript(ctx context.Context, name string, opt int, typ int, codes ...[]byte) error {
	//检测必须的环境变量
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		return fmt.Errorf("lua env miss blockindex ")
	}
	tx := getEnvTx(ctx)
	if tx == nil {
		return fmt.Errorf("lua env miss tx ")
	}
	//初始化脚本环境
	l := newScriptEnv(ctx)
	defer l.Close()
	//交易操作
	l.SetGlobal("tx_opt", lua.LNumber(opt))
	//操作常量
	if typ == 0 {
		l.SetGlobal("OptPushTxPool", lua.LNumber(OptPushTxPool))
		l.SetGlobal("OptAddToBlock", lua.LNumber(OptAddToBlock))
		l.SetGlobal("OptPublishTx", lua.LNumber(OptPublishTx))
	}
	//是否在交易脚本中启用http 接扣
	l.SetGlobal("has_http", lua.LBool(conf.HTTPAPI))
	//可用方法
	initLuaMethodEnv(l, typ)
	if typ == 0 && conf.HTTPAPI {
		initHTTPLuaEnv(l)
	} else if typ == 1 {
		//输入脚本可用方法
		l.SetGlobal("map_set", l.NewFunction(transMapValueSet))
	} else if typ == 2 {
		//输出脚本可用方法
		l.SetGlobal("map_get", l.NewFunction(transMapValueGet))
	}
	//当前区块高度和区块时间
	l.SetGlobal("best_height", lua.LNumber(bi.Height()))
	l.SetGlobal("best_time", lua.LNumber(bi.Time()))
	//当前系统时间戳
	l.SetGlobal("sys_time", lua.LNumber(bi.lptr.TimeNow()))
	//交易id
	l.SetGlobal("tx_id", lua.LString(tx.MustID().String()))
	//交易版本
	l.SetGlobal("tx_ver", lua.LNumber(tx.Ver))
	//如果有签名环境
	if signer := getEnvSigner(ctx); signer != nil {
		//获得签名对象
		stx, in, out, idx := signer.GetObjs()
		//必须是同一个交易
		if stx != tx {
			panic(errors.New("tx error"))
		}
		//输入总数
		l.SetGlobal("in_size", lua.LNumber(len(tx.Ins)))
		//输入总数
		l.SetGlobal("out_size", lua.LNumber(len(tx.Outs)))
		//输入在交易中的索引 0开始
		l.SetGlobal("in_index", lua.LNumber(idx))
		//获取输出金额信息
		coin, err := out.GetCoin(in, bi)
		//签名后执行这里应该不会出错，肯定存在coin
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
		//交易可用方法
		initLuaTxMethod(l)
	}
	//拼接代码
	buf := NewReadWriter()
	for _, vb := range codes {
		buf.WriteFull(vb)
	}
	if *IsDebug {
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
	//只能有一个返回值 true 或者 false
	if err := l.PCall(0, 1, nil); err != nil {
		return fmt.Errorf("call script error %w", err)
	} else if result := l.Get(-1); result.Type() != lua.LTBool {
		return fmt.Errorf("script result type error")
	} else if bok := lua.LVAsBool(result); !bok {
		return fmt.Errorf("script result error")
	}
	return nil
}

//ExecScript 返回错误会不加入交易池或者不进入区块
//执行之前已经校验了签名
func (tx *TX) ExecScript(bi *BlockIndex, opt int) error {
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
	//限制时间
	ctx, cancel := context.WithTimeout(context.Background(), exectime)
	defer cancel()
	ctx = context.WithValue(ctx, blockKey, bi)
	ctx = context.WithValue(ctx, txKey, tx)
	//编译脚本
	return compileExecScript(ctx, "tx_main", opt, 0, txs.Exec)
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
	//每个输入的脚本执行时间为一半交易时间/输入总数
	exectime := time.Duration(int(txs.ExeTime/2)/len(sr.tx.Ins)) * time.Millisecond
	//限制时间
	ctx, cancel := context.WithTimeout(context.Background(), exectime)
	defer cancel()
	//
	ctx = context.WithValue(ctx, blockKey, bi)
	ctx = context.WithValue(ctx, txKey, sr.tx)
	ctx = context.WithValue(ctx, signerKey, sr)
	//输入和输出锁定脚本在两个不同的环境中执行，使用这个map传递数据
	//只用于签名脚本输入输出
	ctx = context.WithValue(ctx, transKey, newTransOutMap(ctx))
	//编译输入脚本 执行错误返回
	if err := compileExecScript(ctx, "input_main", 0, 1, wits.Exec); err != nil {
		return err
	}
	//编译输出脚本
	if err := compileExecScript(ctx, "out_main", 0, 2, lcks.Exec); err != nil {
		return err
	}
	return nil
}
