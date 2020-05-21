package xginx

import (
	"bytes"
	"context"
	"encoding/hex"
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

//timestamp('2001-02-03 11:00:00') 返回指定时间的时间戳,无参数获取当前时间
//默认使用 2006-01-02 15:04:05 格式，也可以 timestamp('2006-01-02','2001-02-03') 指定格式

//best_height 当前区块链高度
//best_time 最高的区块时间

//get_tx() 获取当前环境交易对象
//get_tx('txid') 获取指定交易信息

//v := get_tx()
//v:get_in(idx)
//v:get_out(idx)

//v.sign_idx 签名输入位置 签名检测环境可用
//v.sign_in 签名输入 签名检测环境可用
//v.sign_out 签名输入引用的输出 签名检测环境可用
//v.sign_hash 签名hash hex编码

//encode(tbl) json编码
//decode(str) json解码

//tx_opt 交易脚本操作类型 对应 OptPushTxPool  OptAddToBlock OptPublishTx =0

//has_http  http接口是否可用
//http_post 网络post 交易脚本可用 如果配置中启用了
//http_get  网络get 交易脚本可用 如果配置中启用了

//map_set 输入脚本中设置一个值，在输出脚本中可以用map_get获取到
//map_has 是否存在
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
	bi := getEnvBlockIndex(l.Context())
	sfmt := "2006-01-02 15:04:05"
	top := l.GetTop()
	//无参数返回当前时间
	if top == 0 {
		l.Push(lua.LNumber(bi.lptr.TimeNow()))
		return 1
	}
	var str string
	//如果指定了，参数
	if top >= 2 {
		sfmt = l.ToString(1)
		str = l.ToString(2)
	} else {
		str = l.ToString(1)
	}
	if str == "" {
		l.RaiseError("args miss")
		return 0
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
func initLuaMethodEnv(l *lua.LState, typ int) {
	//获取字符串表示的时间戳
	l.SetGlobal("timestamp", l.NewFunction(unixTimestamp))
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

//key是否存在
func transMapValueHas(l *lua.LState) int {
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
	_, b := tmap.kvs[key]
	l.Push(lua.LBool(b))
	return 1
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
	return 1
}

//设置属性字段
func setBlockTable(l *lua.LState, tbl *lua.LTable, bi *BlockIndex, tx *TX) error {
	//获取交易id
	id, err := tx.ID()
	if err != nil {
		return err
	}
	//交易id
	tbl.RawSetString("id", lua.LString(id.String()))
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
	//是否是coinbase
	tbl.RawSetString("base", lua.LBool(tx.IsCoinBase()))
	//交易费,如果是coinbase，这个返回coinbase输出金额
	fee, err := tx.GetTransFee(bi)
	if err != nil {
		return err
	}
	tbl.RawSetString("fee", lua.LNumber(fee))
	//交易版本
	tbl.RawSetString("ver", lua.LNumber(tx.Ver))
	//输入总数
	tbl.RawSetString("in_size", lua.LNumber(len(tx.Ins)))
	//输出总数
	tbl.RawSetString("out_size", lua.LNumber(len(tx.Outs)))
	return nil
}

//获取交易所在的信息
//如果指定交易id则查询交易信息
func getUpValueTx(l *lua.LState) (*TX, error) {
	up := l.Get(lua.GlobalsIndex - 1)
	if up.Type() != lua.LTUserData {
		return nil, fmt.Errorf("upvalue miss")
	}
	tx, ok := up.(*lua.LUserData).Value.(*TX)
	if !ok {
		return nil, fmt.Errorf("upvalue type error")
	}
	return tx, nil
}

//设置输入属性
func setInTable(tbl *lua.LTable, in *TxIn) {
	tbl.RawSetString("out_index", lua.LNumber(in.OutIndex))
	tbl.RawSetString("out_hash", lua.LString(in.OutHash.String()))
	tbl.RawSetString("sequence", lua.LNumber(in.Sequence))
}

//必须指定参数
func txGetInMethod(l *lua.LState) int {
	top := l.GetTop()
	if top != 2 {
		l.RaiseError("args num error")
		return 0
	}
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
		return 0
	}
	tbl := l.NewTable()
	tx, err := getUpValueTx(l)
	if err != nil {
		l.RaiseError("upvalue tx miss")
		return 0
	}
	iv := l.Get(2)
	if iv.Type() != lua.LTNumber {
		l.RaiseError("args 2 type error")
		return 0
	}
	idx := int(lua.LVAsNumber(iv))
	if idx < 0 || idx >= len(tx.Ins) {
		l.RaiseError("args 1 index out bound")
		return 0
	}
	in := tx.Ins[idx]
	setInTable(tbl, in)
	l.Push(tbl)
	return 1
}

//设置输出属性
func setOutTable(tbl *lua.LTable, out *TxOut) {
	addr, err := out.Script.GetAddress()
	if err != nil {
		panic(err)
	}
	tbl.RawSetString("value", lua.LNumber(out.Value))
	tbl.RawSetString("address", lua.LString(addr))
}

//获取输出信息
func txGetOutMethod(l *lua.LState) int {
	top := l.GetTop()
	if top != 2 {
		l.RaiseError("args num error")
		return 0
	}
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
		return 0
	}
	tbl := l.NewTable()
	tx, err := getUpValueTx(l)
	if err != nil {
		l.RaiseError("upvalue tx miss")
		return 0
	}
	a1 := l.Get(2)
	if a1.Type() != lua.LTNumber {
		l.RaiseError("args 2 type error")
		return 0
	}
	idx := int(lua.LVAsNumber(a1))
	if idx < 0 || idx >= len(tx.Outs) {
		l.RaiseError("args 1 index out bound")
		return 0
	}
	out := tx.Outs[idx]
	setOutTable(tbl, out)
	l.Push(tbl)
	return 1
}

//
func txBlockMethod(l *lua.LState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
		return 0
	}
	top := l.GetTop()
	var tx *TX = nil
	//如果指定了交易id
	if top == 1 {
		id := NewHASH256(l.ToString(1))
		qtx, err := bi.LoadTX(id)
		if err != nil {
			l.RaiseError("find tx failed %s", err.Error())
			return 0
		}
		tx = qtx
	} else {
		//获取当前环境的交易
		tx = getEnvTx(ctx)
	}
	if tx == nil {
		l.RaiseError("tx miss")
		return 0
	}
	tbl := l.NewTable()
	//设置交易所在的区块信息和交易信息
	err := setBlockTable(l, tbl, bi, tx)
	if err != nil {
		l.RaiseError(err.Error())
		return 0
	}
	//设置方法
	uptr := l.NewUserData()
	uptr.Value = tx
	tbl.RawSetString("get_in", l.NewClosure(txGetInMethod, uptr))
	tbl.RawSetString("get_out", l.NewClosure(txGetOutMethod, uptr))
	//如果是在签名环境中
	if signer := getEnvSigner(ctx); signer != nil {
		_, in, out, idx := signer.GetObjs()
		//sign_idx 签名输入位置
		tbl.RawSetString("sign_idx", lua.LNumber(idx))
		//sign_in 签名输入
		itbl := l.NewTable()
		setInTable(itbl, in)
		tbl.RawSetString("sign_in", itbl)
		//sign_out 签名输入引用的输出
		otbl := l.NewTable()
		setOutTable(otbl, out)
		tbl.RawSetString("sign_out", otbl)
		//sign_hash 签名hash
		hash, err := signer.GetSigHash()
		if err != nil {
			l.RaiseError(err.Error())
			return 0
		}
		tbl.RawSetString("sign_hash", lua.LString(hex.EncodeToString(hash)))
	}
	l.Push(tbl)
	return 1
}

//json_encode
func jsonLuaEncode(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.RaiseError("args num error")
		return 0
	}
	tbl := l.ToTable(1)
	if tbl == nil {
		l.Push(lua.LNil)
		return 1
	}
	bv, err := tableToJSON(tbl)
	if err != nil {
		l.RaiseError(err.Error())
		return 0
	}
	l.Push(lua.LString(bv))
	return 1
}

func jsonLuaDecode(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.RaiseError("args num error")
		return 0
	}
	str := l.ToString(1)
	if str == "" {
		l.Push(lua.LNil)
		return 1
	}
	tbl, err := jsonToTable(l, []byte(str))
	if err != nil {
		l.RaiseError(err.Error())
		return 0
	}
	l.Push(tbl)
	return 1
}

//当前输出
func txOutMethod(l *lua.LState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
		return 0
	}
	signer := getEnvSigner(ctx)
	if signer == nil {
		l.RaiseError("current signer miss")
		return 0
	}
	tbl := l.NewTable()
	tx, _, out, _ := signer.GetObjs()
	if l.GetTop() == 1 {
		a1 := l.Get(1)
		if a1.Type() != lua.LTNumber {
			l.RaiseError("args 1 type error")
			return 0
		}
		idx := int(lua.LVAsNumber(a1))
		if idx < 0 || idx >= len(tx.Outs) {
			l.RaiseError("args 1 index out bound")
			return 0
		}
		out = tx.Outs[idx]
	}
	tbl.RawSetString("value", lua.LNumber(out.Value))
	addr, err := out.Script.GetAddress()
	if err != nil {
		l.RaiseError("get address error %s", err.Error())
		return 0
	}
	tbl.RawSetString("address", lua.LString(addr))
	l.Push(tbl)
	return 1
}

//初始化交易可用方法
func initLuaTxMethod(l *lua.LState) {
	//encode(tbl) -> string
	l.SetGlobal("encode", l.NewFunction(jsonLuaEncode))
	//decode(str) -> tbl
	l.SetGlobal("decode", l.NewFunction(jsonLuaDecode))
	//tx()
	l.SetGlobal("get_tx", l.NewFunction(txBlockMethod))
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
		//可读写
		l.SetGlobal("map_has", l.NewFunction(transMapValueHas))
		l.SetGlobal("map_set", l.NewFunction(transMapValueSet))
		l.SetGlobal("map_get", l.NewFunction(transMapValueGet))
	} else if typ == 2 {
		//只读
		l.SetGlobal("map_has", l.NewFunction(transMapValueHas))
		l.SetGlobal("map_get", l.NewFunction(transMapValueGet))
	}
	//当前区块高度和区块时间
	l.SetGlobal("best_height", lua.LNumber(bi.Height()))
	l.SetGlobal("best_time", lua.LNumber(bi.Time()))
	//如果有签名环境
	if signer := getEnvSigner(ctx); signer != nil {
		//验证函数 如果hash一致返回true
		l.SetGlobal("verify_addr", l.NewFunction(verifyAddr))
		//签名正确返回 true
		l.SetGlobal("verify_sign", l.NewFunction(verifySign))
	}
	//交易可用方法
	initLuaTxMethod(l)
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
