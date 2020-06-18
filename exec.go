package xginx

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	lua "github.com/cxuhua/gopher-lua"
	jsoniter "github.com/json-iterator/go"
)

//脚本环境设定
//verify_addr() 验证消费地址 与输入地址hash是否一致
//verify_sign() 验证签名是否正确

//timestamp('2001-02-03 11:00:00') 返回指定时间的时间戳,无参数获取当前时间
//默认使用 2006-01-02 15:04:05 格式，也可以 timestamp('2006-01-02','2001-02-03') 指定格式
//encode(tbl) json编码
//decode(str) json解码
//verify(hash,pub,sig) 校验指定的数据签名,成功返回 true
//map_set 输入脚本中设置一个值，在输出脚本中可以用map_get获取到(输入脚本可用)
//map_has 是否存在指定的key(输出脚本可用)
//map_get 获取输入脚本中设置的值(输出脚本可用)

//best_height 当前区块链高度
//best_time 最高的区块时间

//签名环境下获取当前引用的交易信息
//get_rtx()
//相当于 get_tx(get_tx().sign_in.out_hash)

//获取当前交易信息
//get_tx() 获取当前环境交易对象
//get_tx('txid') 获取指定交易信息

//get_blk('blkid') 获取指定区块的信息
//get_blk() 获取最后一个区块信息

//local tx = get_tx()
//tx.id 交易id hex编码
//tx.blk.id 区块id
//tx.blk.ver 区块版本
//tx.blk.miss =true表示区块不在链上
//tx.blk.height 所在区块高度
//tx.blk.time 所在区块时间
//tx.blk.bits 区块难度
//tx.blk.prev 上一个区块
//tx.blk.merkle 默克尔树id
//tx.cbb 是否是coinbase
//tx.fee 交易费,如果是coinbase，这个返回coinbase输出金额
//tx.ver 交易版本
//tx.ninv 输入总数
//tx.nout 输出总数
//tx:inv(idx) 获取指定输入
//tx:out(idx) 获取指定输出
//tx.sign 当前签名数据信息
//tx.sign.idx 签名输入位置 签名检测环境可用
//tx.sign.inv 签名输入 签名检测环境可用
//tx.sign.out 签名输入引用的输出 签名检测环境可用
//tx.sign.hash 签名hash hex编码 签名检测环境可用
//tx.wits 当前证书信息
//tx.wits.num 公钥数量
//tx.wits.less 最小成功签名数量
//tx.wits.arb 是否启用仲裁 != 255 表示启用,表示哪个公钥是仲裁公钥
//tx.wits.npub 公钥数量
//tx.wits.nsig 签名数量
//tx.wits.pub(idx)  获取公钥 返回->string hex编码
//tx.wits.sig(idx) 获取签名返回 ->string hex编码
//tx.wits.verify(idx) 返回验证符合某个签名的公钥索引 返回-1表示没有符合签名的公钥

//in 的属性方法
//in.out_hash 引用交易hash
//in.out_index 输出索引
//in.sequence 序列号
//in.address 输入地址

//out 属性方法
//out.value 输出金额
//out.address 输出地址

var (
	//DefaultTxScript 默认交易脚本 控制是否能进入区块
	DefaultTxScript = []byte(`return true`)
	//DefaultInputScript 默认输入脚本 控制是否能消费
	DefaultInputScript = []byte(`return true`)
	//DefaultLockedScript 默认锁定脚本 控制消费输出需要的条件
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

//用于将输入数据传递到输出
//只支持 int float bool string object
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

func (tm *transOutMap) setTable(k string, v *lua.LTable) {
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
	blockKey  = &BlockIndex{}
	txKey     = &TX{}
	signerKey = &mulsigner{}
	transKey  = &transOutMap{}
	//transOutMap 最大可存储的数量
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
//timestamp() 返回当前系统时间戳
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
	//如果指定了格式
	if top >= 2 {
		sfmt = l.ToString(1)
		str = l.ToString(2)
	} else {
		str = l.ToString(1)
	}
	if str == "" {
		l.RaiseError("args miss")
	}
	tv, err := time.ParseInLocation(sfmt, str, time.Local)
	if err != nil {
		l.RaiseError(err.Error())
	}
	l.Push(lua.LNumber(tv.Unix()))
	return 1
}

//检测输入hash和锁定hash是否一致
func verifyAddr(l *lua.LState) int {
	signer := getEnvSigner(l.Context())
	if signer == nil {
		l.RaiseError("tx script env can't use")
	}
	err := signer.VerifyAddr()
	l.Push(lua.LBool(err == nil))
	return 1
}

//检测签名是否正确
func verifySign(l *lua.LState) int {
	signer := getEnvSigner(l.Context())
	if signer == nil {
		l.RaiseError("tx script env can't use")
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
	}
	k := l.Get(1)
	if k.Type() != lua.LTString {
		l.RaiseError("args 1 type error")
	}
	key := lua.LVAsString(k)
	if key == "" {
		l.RaiseError("args 1 empty error")
	}
	tmap := getEnvTransMap(l.Context())
	if tmap == nil {
		l.RaiseError("trans map miss")
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
	} else if typ == lua.LTTable {
		tbl := v.(*lua.LTable)
		tmap.setTable(key, tbl)
	} else {
		l.RaiseError("args 2 type error")
	}
	return 0
}

//key是否存在
func transMapValueHas(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.RaiseError("args num error")
	}
	k := l.Get(1)
	if k.Type() != lua.LTString {
		l.RaiseError("args 1 type error")
	}
	key := lua.LVAsString(k)
	if key == "" {
		l.RaiseError("args 1 empty error")
	}
	tmap := getEnvTransMap(l.Context())
	if tmap == nil {
		l.RaiseError("trans map miss")
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
	}
	k := l.Get(1)
	if k.Type() != lua.LTString {
		l.RaiseError("args 1 type error")
	}
	key := lua.LVAsString(k)
	if key == "" {
		l.RaiseError("args 1 empty error")
	}
	tmap := getEnvTransMap(l.Context())
	if tmap == nil {
		l.RaiseError("trans map miss")
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
		case *lua.LTable:
			l.Push(v.(*lua.LTable))
		default:
			l.Push(lua.LNil)
		}
	}
	return 1
}

//设置区块头属性
func setBlockAttr(tbl *lua.LTable, blk *BlockInfo) {
	kvs := tbl.RawSetString
	kvs("miss", lua.LBool(false))
	kvs("ver", lua.LNumber(blk.Meta.Ver))
	kvs("id", lua.LString(blk.MustID().String()))
	kvs("height", lua.LNumber(blk.Meta.Height))
	kvs("time", lua.LNumber(blk.Meta.Time))
	kvs("bits", lua.LNumber(blk.Meta.Bits))
	kvs("prev", lua.LString(blk.Meta.Prev.String()))
	kvs("merkle", lua.LString(blk.Meta.Merkle.String()))
}

//获取区块头信息
func getBlockMethod(l *lua.LState) int {
	bi := getEnvBlockIndex(l.Context())
	if bi == nil {
		l.RaiseError("block env miss")
	}
	var id HASH256
	if l.GetTop() == 0 {
		id = bi.Last().MustID()
	} else {
		id = NewHASH256(l.ToString(1))
	}
	if id.IsZero() {
		l.RaiseError("id %v error", id)
	}
	lblk := l.NewTable()
	kvs := lblk.RawSetString
	blk, err := bi.LoadBlock(id)
	if err != nil {
		kvs("miss", lua.LBool(true))
		kvs("id", lua.LString(id.String()))
	} else {
		setBlockAttr(lblk, blk)
	}
	l.Push(lblk)
	return 1
}

//创建自定义数据
func newUserData(l *lua.LState, vptr interface{}) *lua.LUserData {
	uptr := l.NewUserData()
	uptr.Value = vptr
	return uptr
}

//设置属性字段
//获取交易id
func setTxBlockTable(l *lua.LState, tbl *lua.LTable, bi *BlockIndex, tx *TX) error {
	id, err := tx.ID()
	if err != nil {
		return err
	}
	kvs := tbl.RawSetString
	//交易id
	kvs("id", lua.LString(id.String()))
	//查询交易所在的区块信息
	v, err := bi.LoadTxValue(id)
	//如果查找不到使用下个区块高度和当前时间
	lblk := l.NewTable()
	lkvs := lblk.RawSetString
	if err != nil {
		lkvs("miss", lua.LBool(true))
	} else if blk, err := bi.LoadBlock(v.BlkID); err != nil {
		lkvs("miss", lua.LBool(true))
		lkvs("id", lua.LString(v.BlkID.String()))
	} else {
		setBlockAttr(lblk, blk)
	}
	kvs("blk", lblk)
	//是否是coinbase
	kvs("cbb", lua.LBool(tx.IsCoinBase()))
	//交易费,如果是coinbase，这个返回coinbase输出金额
	fee, err := tx.GetTransFee(bi)
	if err != nil {
		return err
	}
	kvs("fee", lua.LNumber(fee))
	//交易版本
	kvs("ver", lua.LNumber(tx.Ver))
	//输入总数
	kvs("ninv", lua.LNumber(len(tx.Ins)))
	//输出总数
	kvs("nout", lua.LNumber(len(tx.Outs)))
	//传递当前tx到闭合函数
	uptr := newUserData(l, tx)
	kvs("inv", l.NewClosure(txGetInMethod, uptr))
	kvs("out", l.NewClosure(txGetOutMethod, uptr))
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

//获取upvalue wits对象
func getScriptWits(l *lua.LState) (*WitnessScript, error) {
	up := l.Get(lua.GlobalsIndex - 1)
	if up.Type() != lua.LTUserData {
		return nil, fmt.Errorf("upvalue miss")
	}
	wits, ok := up.(*lua.LUserData).Value.(*WitnessScript)
	if !ok {
		return nil, fmt.Errorf("upvalue type error")
	}
	return wits, nil
}

//获取输入证书公钥
func getWitsPubMethod(l *lua.LState) int {
	//参数1 是self
	if l.GetTop() != 2 {
		l.RaiseError("args error")
	}
	wits, err := getScriptWits(l)
	if err != nil {
		l.RaiseError(err.Error())
	}
	idx := l.ToInt(2)
	if idx < 0 || idx >= len(wits.Pks) {
		l.RaiseError("idx outbound")
	}
	str := hex.EncodeToString(wits.Pks[idx][:])
	l.Push(lua.LString(str))
	return 1
}

//获取输入证书签名
func getWitsSigMethod(l *lua.LState) int {
	//参数1 是self
	if l.GetTop() != 2 {
		l.RaiseError("args error")
	}
	wits, err := getScriptWits(l)
	if err != nil {
		l.RaiseError(err.Error())
	}
	idx := l.ToInt(2)
	if idx < 0 || idx >= len(wits.Sig) {
		l.RaiseError("idx outbound")
	}
	str := hex.EncodeToString(wits.Sig[idx][:])
	l.Push(lua.LString(str))
	return 1
}

//验证第几个签名，并返回符合的公钥数量 idx,bool
func getWitsVerityMethod(l *lua.LState) int {
	signer := getEnvSigner(l.Context())
	if signer == nil {
		l.RaiseError("env miss signer")
	}
	//参数1 是self
	if l.GetTop() != 2 {
		l.RaiseError("args error")
	}
	wits, err := getScriptWits(l)
	if err != nil {
		l.RaiseError(err.Error())
	}
	idx := l.ToInt(2)
	if idx < 0 || idx >= len(wits.Sig) {
		l.RaiseError("idx outbound")
	}
	sigb := wits.Sig[idx]
	sig, err := NewSigValue(sigb.Bytes())
	if err != nil {
		l.RaiseError(err.Error())
	}
	//获取签名hash
	hash, err := signer.GetSigHash()
	if err != nil {
		l.RaiseError(err.Error())
	}
	//获取符合签名的公钥
	for i := 0; i < len(wits.Pks); i++ {
		pub, err := NewPublicKey(wits.Pks[i].Bytes())
		if err != nil {
			l.RaiseError(err.Error())
		}
		if pub.Verify(hash, sig) {
			l.Push(lua.LNumber(i))
			return 1
		}
	}
	l.Push(lua.LNumber(-1))
	return 1
}

//设置输入属性
func setInTable(l *lua.LState, tbl *lua.LTable, in *TxIn) error {
	kvs := tbl.RawSetString
	kvs("out_index", lua.LNumber(in.OutIndex))
	kvs("out_hash", lua.LString(in.OutHash.String()))
	kvs("sequence", lua.LNumber(in.Sequence))
	//获取输入地址
	wits, err := in.Script.ToWitness()
	if err == nil {
		kvs("address", lua.LString(wits.Address()))
	}
	return nil
}

//必须指定参数
func txGetInMethod(l *lua.LState) int {
	top := l.GetTop()
	if top != 2 {
		l.RaiseError("args num error")
	}
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
	}
	tbl := l.NewTable()
	//从upvalue获取交易信息
	tx, err := getUpValueTx(l)
	if err != nil {
		l.RaiseError("upvalue tx miss")
	}
	iv := l.Get(2)
	if iv.Type() != lua.LTNumber {
		l.RaiseError("args 2 type error")
	}
	idx := int(lua.LVAsNumber(iv))
	if idx < 0 || idx >= len(tx.Ins) {
		l.RaiseError("args 1 index out bound")
	}
	in := tx.Ins[idx]
	err = setInTable(l, tbl, in)
	if err != nil {
		l.RaiseError("set in table error %s", err.Error())
	}
	l.Push(tbl)
	return 1
}

//设置输出属性
func setOutTable(l *lua.LState, tbl *lua.LTable, out *TxOut) error {
	kvs := tbl.RawSetString
	addr, err := out.Script.GetAddress()
	if err != nil {
		return err
	}
	kvs("value", lua.LNumber(out.Value))
	kvs("address", lua.LString(addr))
	return nil
}

//获取输出信息
func txGetOutMethod(l *lua.LState) int {
	top := l.GetTop()
	if top != 2 {
		l.RaiseError("args num error")
	}
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
	}
	tbl := l.NewTable()
	//从upvalue获取交易信息
	tx, err := getUpValueTx(l)
	if err != nil {
		l.RaiseError("upvalue tx miss")
	}
	a1 := l.Get(2)
	if a1.Type() != lua.LTNumber {
		l.RaiseError("args 2 type error")
	}
	idx := int(lua.LVAsNumber(a1))
	if idx < 0 || idx >= len(tx.Outs) {
		l.RaiseError("args 1 index out bound")
	}
	out := tx.Outs[idx]
	err = setOutTable(l, tbl, out)
	if err != nil {
		l.RaiseError("set out table error %s", err.Error())
	}
	l.Push(tbl)
	return 1
}

//设置交易信息
func setTxLuaAttr(l *lua.LState, bi *BlockIndex, tx *TX, tbl *lua.LTable) error {
	ctx := l.Context()
	kvs := tbl.RawSetString
	//设置交易所在的区块信息和交易信息
	err := setTxBlockTable(l, tbl, bi, tx)
	if err != nil {
		return err
	}
	//如果是在签名环境中
	signer := getEnvSigner(ctx)
	if signer == nil {
		return nil
	}
	//以下在签名环境下可用
	_, in, out, idx := signer.GetObjs()
	stbl := l.NewTable()
	skvs := stbl.RawSetString
	//sign_idx 签名输入位置
	skvs("idx", lua.LNumber(idx))
	//sign_in 当前签名输入
	itbl := l.NewTable()
	err = setInTable(l, itbl, in)
	if err != nil {
		return err
	}
	skvs("inv", itbl)
	//sign_out 签名输入引用的输出
	otbl := l.NewTable()
	err = setOutTable(l, otbl, out)
	if err != nil {
		return err
	}
	skvs("out", otbl)
	//sign_hash 签名hash
	hash, err := signer.GetSigHash()
	if err != nil {
		return err
	}
	skvs("hash", lua.LString(hex.EncodeToString(hash)))
	kvs("sign", stbl)
	wits, err := in.Script.ToWitness()
	if err != nil {
		return err
	}
	upptr := newUserData(l, wits)
	wtbl := l.NewTable()
	wkvs := wtbl.RawSetString
	//wits_num int
	wkvs("num", lua.LNumber(wits.Num))
	//wits_less int
	wkvs("less", lua.LNumber(wits.Less))
	//wits_arb bool
	wkvs("arb", lua.LNumber(wits.Arb))
	//pub_size
	wkvs("npub", lua.LNumber(len(wits.Pks)))
	//sig_size
	wkvs("nsig", lua.LNumber(len(wits.Sig)))
	//get_pub(idx) ->string hex编码
	wkvs("pub", l.NewClosure(getWitsPubMethod, upptr))
	//get_sig(idx) ->string hex编码
	wkvs("sig", l.NewClosure(getWitsSigMethod, upptr))
	//verity(sig idx) 验证签名,返回 符合的pub 索引
	wkvs("verify", l.NewClosure(getWitsVerityMethod, upptr))
	kvs("wits", wtbl)
	return nil
}

//获取当前输入引用的交易信息
func getTxInRefMethod(l *lua.LState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("blockindex env miss")
	}
	//如果是在签名环境中
	signer := getEnvSigner(ctx)
	if signer == nil {
		l.RaiseError("signer env miss")
	}
	_, in, _, _ := signer.GetObjs()
	tx, err := bi.LoadTX(in.OutHash)
	if signer == nil {
		l.RaiseError(err.Error())
	}
	tbl := l.NewTable()
	err = setTxBlockTable(l, tbl, bi, tx)
	if err != nil {
		l.RaiseError(err.Error())
	}
	l.Push(tbl)
	return 1
}

//
func txBlockMethod(l *lua.LState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
	}
	top := l.GetTop()
	var tx *TX = nil
	//如果指定了交易id
	if top == 1 {
		id := NewHASH256(l.ToString(1))
		qtx, err := bi.LoadTX(id)
		if err == nil {
			tx = qtx
		}
	} else {
		//获取当前环境的交易
		tx = getEnvTx(ctx)
	}
	//返回nil表示没有交易信息
	if tx == nil {
		l.Push(lua.LNil)
		return 1
	}
	tbl := l.NewTable()
	//设置交易所在的区块信息和交易信息
	err := setTxLuaAttr(l, bi, tx, tbl)
	if err != nil {
		l.RaiseError(err.Error())
	}
	l.Push(tbl)
	return 1
}

//json_encode
func jsonLuaEncode(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.RaiseError("args num error")
	}
	tbl := l.ToTable(1)
	if tbl == nil {
		l.Push(lua.LNil)
		return 1
	}
	bv, err := tableToJSON(tbl)
	if err != nil {
		l.RaiseError(err.Error())
	}
	l.Push(lua.LString(bv))
	return 1
}

func jsonLuaDecode(l *lua.LState) int {
	if l.GetTop() != 1 {
		l.RaiseError("args num error")
	}
	str := l.ToString(1)
	if str == "" {
		l.Push(lua.LNil)
		return 1
	}
	tbl, err := jsonToTable(l, []byte(str))
	if err != nil {
		l.RaiseError(err.Error())
	}
	l.Push(tbl)
	return 1
}

func verifyLuaDecode(l *lua.LState) int {
	if l.GetTop() != 3 {
		l.RaiseError("args num error")
	}
	hb, err := hex.DecodeString(l.ToString(1))
	if err != nil || len(hb) == 0 {
		l.RaiseError(err.Error())
	}
	pb, err := hex.DecodeString(l.ToString(2))
	if err != nil || len(pb) == 0 {
		l.RaiseError(err.Error())
	}
	sb, err := hex.DecodeString(l.ToString(3))
	if err != nil || len(sb) == 0 {
		l.RaiseError(err.Error())
	}
	pub, err := NewPublicKey(pb)
	if err != nil {
		l.RaiseError(err.Error())
	}
	sig, err := NewSigValue(sb)
	if err != nil {
		l.RaiseError(err.Error())
	}
	okb := pub.Verify(hb, sig)
	l.Push(lua.LBool(okb))
	return 1
}

//当前输出
func txOutMethod(l *lua.LState) int {
	ctx := l.Context()
	bi := getEnvBlockIndex(ctx)
	if bi == nil {
		l.RaiseError("block index miss")
	}
	signer := getEnvSigner(ctx)
	if signer == nil {
		l.RaiseError("current signer miss")
	}
	tbl := l.NewTable()
	tx, _, out, _ := signer.GetObjs()
	if l.GetTop() == 1 {
		a1 := l.Get(1)
		if a1.Type() != lua.LTNumber {
			l.RaiseError("args 1 type error")
		}
		idx := int(lua.LVAsNumber(a1))
		if idx < 0 || idx >= len(tx.Outs) {
			l.RaiseError("args 1 index out bound")
		}
		out = tx.Outs[idx]
	}
	tbl.RawSetString("value", lua.LNumber(out.Value))
	addr, err := out.Script.GetAddress()
	if err != nil {
		l.RaiseError("get address error %s", err.Error())
	}
	tbl.RawSetString("address", lua.LString(addr))
	l.Push(tbl)
	return 1
}

//初始化交易可用方法
func initLuaTxMethod(l *lua.LState, bi *BlockIndex, typ int) {
	//输入脚本中
	if typ == 1 {
		//可写 设置一个数据，在输出脚本中可获取到
		l.SetGlobal("map_set", l.NewFunction(transMapValueSet))
	}
	//输出脚本map只读
	if typ == 2 {
		//只读 检测是否有指定的key
		l.SetGlobal("map_has", l.NewFunction(transMapValueHas))
		//只读 获取指定的key的值
		l.SetGlobal("map_get", l.NewFunction(transMapValueGet))
	}
	//签名环境下
	if typ != 0 {
		//验证函数 如果hash一致返回true
		l.SetGlobal("verify_addr", l.NewFunction(verifyAddr))
		//签名正确返回 true
		l.SetGlobal("verify_sign", l.NewFunction(verifySign))
	}
	//获取字符串表示的时间戳
	l.SetGlobal("timestamp", l.NewFunction(unixTimestamp))
	//encode(tbl) -> string json格式
	l.SetGlobal("encode", l.NewFunction(jsonLuaEncode))
	//decode(str) -> tbl json格式
	l.SetGlobal("decode", l.NewFunction(jsonLuaDecode))
	//verify(hash,pub,sig) 参数都是hex编码
	l.SetGlobal("verify", l.NewFunction(verifyLuaDecode))
	//获取交易信息
	l.SetGlobal("get_tx", l.NewFunction(txBlockMethod))
	//获取区块头信息
	l.SetGlobal("get_blk", l.NewFunction(getBlockMethod))
	//获取当前引用的交易信息
	l.SetGlobal("get_rtx", l.NewFunction(getTxInRefMethod))
}

//编译脚本
//typ = 0,tx script
//typ = 1,input script
//typ = 2,out script
func compileExecScript(ctx context.Context, name string, typ int, codes ...[]byte) error {
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
	//获取必须的环境变量
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
	//初始化脚本环境
	initLuaTxMethod(l, bi, typ)
	//加载脚本
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

//ExecScript 返回错误交易不进入区块
//执行之前已经校验了签名
//AddTxs LinkBlk 时会执行这个交易脚本检测
func (tx *TX) ExecScript(bi *BlockIndex) error {
	txs, err := tx.Script.ToTxScript()
	if err != nil {
		return err
	}
	if txs.Exec.Len() == 0 {
		//无脚本不执行
		return nil
	}
	//交易脚本执行时间为cpu/2
	exectime := time.Duration(txs.ExeTime/2) * time.Millisecond
	//限制时间
	ctx, cancel := context.WithTimeout(context.Background(), exectime)
	defer cancel()
	ctx = context.WithValue(ctx, blockKey, bi)
	ctx = context.WithValue(ctx, txKey, tx)
	//编译脚本
	return compileExecScript(ctx, "tx_main", 0, txs.Exec)
}

//ExecScript 执行签名交易脚本
//执行之前签名已经通过
func (sr *mulsigner) ExecScript(bi *BlockIndex, wits *WitnessScript, lcks *LockedScript) error {
	//如果未设置就不执行了
	if slen := wits.Exec.Len() + lcks.Exec.Len(); slen == 0 {
		//无脚本不执行
		return nil
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
	//只用于签名脚本输入输出传递信息
	ctx = context.WithValue(ctx, transKey, newTransOutMap(ctx))
	//编译输入脚本 执行错误返回
	if err := compileExecScript(ctx, "in_main", 1, wits.Exec); err != nil {
		return err
	}
	//编译输出脚本
	if err := compileExecScript(ctx, "out_main", 2, lcks.Exec); err != nil {
		return err
	}
	return nil
}
