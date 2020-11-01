package xlua

/*
#cgo CFLAGS: -I .
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <lapi.h>
#include <lualib.h>
#include <lauxlib.h>
#include <xlua.h>
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"
)

var (
	ErrStepLimit = errors.New("run step arraive limit")
)

const (
	TNIL      = C.LUA_TNIL
	TBOOLEAN  = C.LUA_TBOOLEAN
	TNUMBER   = C.LUA_TNUMBER
	TSTRING   = C.LUA_TSTRING
	TTABLE    = C.LUA_TTABLE
	TFUNCTION = C.LUA_TFUNCTION
)

type ITable interface {
	Set(k interface{}, v interface{}) ITable
	Get(k interface{}) interface{}
	ForEach(fn func() bool)
	IsArray(lp ...*int) bool
	Append(v interface{}) ITable
}

type table struct {
	l   ILuaState
	idx int
}

//每次获取长度和检测,速度较慢,连续添加使用Set(int,v)
func (tbl *table) Append(v interface{}) ITable {
	ll := 0
	if !tbl.IsArray(&ll) {
		panic(fmt.Errorf("not array table"))
	}
	return tbl.Set(ll+1, v)
}

func (tbl *table) IsArray(lp ...*int) bool {
	ll, ok := tbl.l.IsArray(tbl.idx)
	if len(lp) > 0 && lp[0] != nil {
		*lp[0] = ll
	}
	return ok
}

func (tbl *table) Set(k interface{}, v interface{}) ITable {
	switch kt := k.(type) {
	case int:
		tbl.l.SetArray(tbl.idx, kt, v)
	case *int:
		tbl.l.SetArray(tbl.idx, *kt, v)
	case int16:
		tbl.l.SetArray(tbl.idx, int(kt), v)
	case int32:
		tbl.l.SetArray(tbl.idx, int(kt), v)
	case int64:
		tbl.l.SetArray(tbl.idx, int(kt), v)
	case uint:
		tbl.l.SetArray(tbl.idx, int(kt), v)
	case uint16:
		tbl.l.SetArray(tbl.idx, int(kt), v)
	case uint32:
		tbl.l.SetArray(tbl.idx, int(kt), v)
	case uint64:
		tbl.l.SetArray(tbl.idx, int(kt), v)
	case string:
		tbl.l.SetValue(tbl.idx, kt, v)
	case *string:
		tbl.l.SetValue(tbl.idx, *kt, v)
	default:
		panic(fmt.Errorf("not support key type %v", k))
	}
	return tbl
}

//-2 -> key -1 ->value
func (tbl *table) ForEach(fn func() bool) {
	tbl.l.PushNil()
	for tbl.l.Next(tbl.idx-1) != TNIL {
		if !fn() {
			tbl.l.Pop(2)
			break
		}
		tbl.l.Pop(1)
	}
}

func (tbl *table) Get(k interface{}) interface{} {
	switch kt := k.(type) {
	case int:
		return tbl.l.GetArray(tbl.idx, kt)
	case *int:
		return tbl.l.GetArray(tbl.idx, *kt)
	case int16:
		return tbl.l.GetArray(tbl.idx, int(kt))
	case int32:
		return tbl.l.GetArray(tbl.idx, int(kt))
	case int64:
		return tbl.l.GetArray(tbl.idx, int(kt))
	case uint:
		return tbl.l.GetArray(tbl.idx, int(kt))
	case uint16:
		return tbl.l.GetArray(tbl.idx, int(kt))
	case uint32:
		return tbl.l.GetArray(tbl.idx, int(kt))
	case uint64:
		return tbl.l.GetArray(tbl.idx, int(kt))
	case string:
		return tbl.l.GetValue(tbl.idx, kt)
	case *string:
		return tbl.l.GetValue(tbl.idx, *kt)
	default:
		panic(fmt.Errorf("not support key type %v", k))
	}
}

type ILuaState interface {
	//设置当前步数
	SetStep(step int)
	//关闭状态机
	Close()
	//打开所有标准库
	OpenLibs()
	//加载执行脚本
	Exec(script []byte) error
	//检测脚本语法错误
	Check(script []byte) (err error)
	//获取环境
	Context() context.Context
	//注册全局函数,函数不能引用其他go指针数据
	SetFunc(name string, fn LuaFunc) ILuaState
	//设置执行步限制
	SetLimit(limit int) ILuaState
	//获取当前执行步数
	GetStep() int
	//global
	SetGlobal(name string)
	GetGlobal(name string) int
	GetTop() int
	SetTop(idx int)
	Pop(idx int)
	//get stack
	GetType(idx int) int
	ToStr(idx int) string
	ToInt(idx int) int64
	ToFloat(idx int) float64
	ToBool(idx int) bool
	ToFunc(idx int) LuaFunc
	ToValue(idx int) interface{}
	IsStr(idx int) bool
	IsInt(idx int) bool
	IsFloat(idx int) bool
	IsBool(idx int) bool
	IsTable(idx int) bool
	IsFunc(idx int) bool
	//push stack
	PushNil()
	PushValue(v interface{})
	PushStr(v string)
	PushInt(v int64)
	PushFloat(v float64)
	PushBool(v bool)
	PushFunc(fn LuaFunc)
	//table
	NewTable()
	//each top stack table
	ForEach(fn func() bool)
	//是否是数组,并返回长度
	IsArray(idx int) (int, bool)
	Next(idx int) int
	TableLen(idx int) int
	SetTable(idx int)
	SetField(idx int, key string)
	GetTable(idx int) int
	GetField(idx int, key string) int
	//作为对象设置table值
	SetValue(idx int, k string, v interface{})
	GetValue(idx int, k string) interface{}
	//做为数组设置table值
	SetArray(idx int, k int, v interface{})
	GetArray(idx int, k int) interface{}
	//对idx位置的table操作
	ToTable(idx int) ITable
}

type luastate struct {
	step   int
	limit  int
	ctx    context.Context
	cancel context.CancelFunc
	ptr    *C.lua_State
}

func tocharptr(str string) *C.char {
	btr := []byte(str)
	return (*C.char)(unsafe.Pointer(&btr[0]))
}

func (l *luastate) ToTable(idx int) ITable {
	if !l.IsTable(idx) {
		panic(fmt.Errorf("stack index = %dnot table", idx))
	}
	return &table{
		l:   l,
		idx: idx,
	}
}

//-2 -> key -1 ->value
func (l *luastate) ForEach(fn func() bool) {
	top := l.GetTop()
	l.assert(l.IsTable(-1), "top stack should table")
	l.PushNil()
	for l.Next(top) != TNIL {
		if !fn() {
			l.Pop(2)
			break
		}
		l.Pop(1)
	}
}

//table is array,index range 1-> n
func (l *luastate) IsArray(idx int) (int, bool) {
	ll := 0
	lptr := (*C.int)(unsafe.Pointer(&ll))
	isarr := bool(C.go_lua_is_array(l.ptr, lptr, C.int(idx)))
	return ll, isarr
}

func (l *luastate) Next(idx int) int {
	return int(C.lua_next(l.ptr, C.int(idx)))
}

func (l *luastate) SetStep(step int) {
	l.step = step
}

func (l *luastate) TableLen(idx int) int {
	return int(C.lua_rawlen(l.ptr, C.int(idx)))
}

func (l *luastate) assert(cond bool, sfmt string, arg ...interface{}) {
	if !cond {
		panic(fmt.Errorf(sfmt, arg...))
	}
}

func (l *luastate) SetArray(idx int, k int, v interface{}) {
	_, ok := l.IsArray(idx)
	l.assert(ok, "idx stack should table")
	l.PushInt(int64(k))
	l.PushValue(v)
	l.SetTable(idx - 2)
}

func (l *luastate) GetArray(idx int, k int) interface{} {
	ll, ok := l.IsArray(idx)
	l.assert(ok, "idx stack should table")
	l.assert(k >= 1 && k <= ll, "Index out of bounds")
	l.PushInt(int64(k))
	l.GetTable(idx - 1)
	v := l.ToValue(-1)
	l.Pop(1)
	return v
}

func (l *luastate) SetValue(idx int, k string, v interface{}) {
	l.assert(l.IsTable(idx), "idx stack should table")
	l.PushValue(v)
	l.SetField(idx-1, k)
}

func (l *luastate) GetValue(idx int, k string) interface{} {
	l.assert(l.IsTable(idx), "idx stack should table")
	l.GetField(idx, k)
	v := l.ToValue(-1)
	l.Pop(1)
	return v
}

func (l *luastate) PushValue(v interface{}) {
	switch iv := v.(type) {
	case int:
		l.PushInt(int64(iv))
	case int16:
		l.PushInt(int64(iv))
	case int32:
		l.PushInt(int64(iv))
	case int64:
		l.PushInt(iv)
	case uint:
		l.PushInt(int64(iv))
	case uint16:
		l.PushInt(int64(iv))
	case uint32:
		l.PushInt(int64(iv))
	case uint64:
		l.PushInt(int64(iv))
	case string:
		l.PushStr(iv)
	case float32:
		l.PushFloat(float64(iv))
	case float64:
		l.PushFloat(float64(iv))
	case bool:
		l.PushBool(iv)
	case LuaFunc:
		l.PushFunc(iv)
	default:
		panic(fmt.Errorf("not support push type %v", iv))
	}
}

func (l *luastate) PushFunc(fn LuaFunc) {
	C.go_lua_push_func(l.ptr, unsafe.Pointer(&fn))
}

func (l *luastate) PushNil() {
	C.lua_pushnil(l.ptr)
}

func (l *luastate) Pop(idx int) {
	C.lua_settop(l.ptr, C.int(-idx-1))
}

func (l *luastate) NewTable() {
	C.lua_createtable(l.ptr, 0, 0)
}

func (l *luastate) GetType(idx int) int {
	return int(C.lua_type(l.ptr, C.int(idx)))
}

func (l *luastate) GetTable(idx int) int {
	return int(C.lua_gettable(l.ptr, C.int(idx)))
}

func (l *luastate) GetField(idx int, key string) int {
	str := tocharptr(key)
	return int(C.lua_getfield(l.ptr, C.int(idx), str))
}

func (l *luastate) SetTable(idx int) {
	C.lua_settable(l.ptr, C.int(idx))
}

func (l *luastate) SetField(idx int, key string) {
	str := tocharptr(key)
	C.lua_setfield(l.ptr, C.int(idx), str)
}

func (l *luastate) SetGlobal(name string) {
	str := tocharptr(name)
	C.lua_setglobal(l.ptr, str)
}

func (l *luastate) GetGlobal(name string) int {
	str := tocharptr(name)
	return int(C.lua_getglobal(l.ptr, str))
}

func (l *luastate) Context() context.Context {
	return l.ctx
}

func (l *luastate) GetStep() int {
	return l.step
}

func (l *luastate) Close() {
	if l.ptr != nil {
		C.lua_close(l.ptr)
		l.ptr = nil
	}
}

func (l *luastate) GetTop() int {
	return int(C.lua_gettop(l.ptr))
}
func (l *luastate) SetTop(idx int) {
	C.lua_settop(l.ptr, C.int(idx))
}

func (l *luastate) ToStr(idx int) string {
	sl := C.size_t(0)
	str := C.lua_tolstring(l.ptr, C.int(idx), &sl)
	return C.GoString(str)
}

func (l *luastate) ToInt(idx int) int64 {
	isnum := 0
	ptr := (*C.int)(unsafe.Pointer(&isnum))
	ret := int64(C.lua_tointegerx(l.ptr, C.int(idx), ptr))
	if isnum == 0 {
		panic(fmt.Errorf("idx=%d not integer", idx))
	}
	return ret
}

func (l *luastate) ToFloat(idx int) float64 {
	isnum := 0
	ptr := (*C.int)(unsafe.Pointer(&isnum))
	ret := float64(C.lua_tonumberx(l.ptr, C.int(idx), ptr))
	if isnum == 0 {
		panic(fmt.Errorf("idx=%d not number", idx))
	}
	return ret
}

func (l *luastate) IsTable(idx int) bool {
	return C.lua_type(l.ptr, C.int(idx)) == C.LUA_TTABLE
}

func (l *luastate) IsFunc(idx int) bool {
	return C.lua_iscfunction(l.ptr, C.int(idx)) != 0
}

func (l *luastate) ToFunc(idx int) LuaFunc {
	ptr := unsafe.Pointer(C.lua_tocfunction(l.ptr, C.int(idx)))
	return *(*LuaFunc)(ptr)
}

func (l *luastate) ToValue(idx int) interface{} {
	if l.IsStr(idx) {
		return l.ToStr(idx)
	}
	if l.IsBool(idx) {
		return l.ToBool(idx)
	}
	//优先检测是否是整形
	if l.IsInt(idx) {
		return l.ToInt(idx)
	}
	if l.IsFloat(idx) {
		return l.ToFloat(idx)
	}
	if l.IsFunc(idx) {
		return l.ToFunc(idx)
	}
	if l.IsTable(idx) {
		return l.ToTable(idx)
	}
	panic(fmt.Errorf("type error"))
}

func (l *luastate) ToBool(idx int) bool {
	return C.lua_toboolean(l.ptr, C.int(idx)) != 0
}

func (l *luastate) IsStr(idx int) bool {
	return C.lua_type(l.ptr, C.int(idx)) == C.LUA_TSTRING
}

func (l *luastate) IsInt(idx int) bool {
	return C.lua_isinteger(l.ptr, C.int(idx)) != 0
}

func (l *luastate) IsFloat(idx int) bool {
	return C.lua_isnumber(l.ptr, C.int(idx)) != 0
}

func (l *luastate) IsBool(idx int) bool {
	return C.lua_type(l.ptr, C.int(idx)) == C.LUA_TBOOLEAN
}

func (l *luastate) PushStr(v string) {
	str := tocharptr(v)
	C.lua_pushlstring(l.ptr, str, C.size_t(len(v)))
}

func (l *luastate) PushInt(v int64) {
	C.lua_pushinteger(l.ptr, C.longlong(v))
}

func (l *luastate) PushFloat(v float64) {
	C.lua_pushnumber(l.ptr, C.double(v))
}

func (l *luastate) PushBool(v bool) {
	if v {
		C.lua_pushboolean(l.ptr, 1)
	} else {
		C.lua_pushboolean(l.ptr, 0)
	}
}

func (l *luastate) Check(script []byte) error {
	var err error
	var bptr *C.char = (*C.char)(unsafe.Pointer(&script[0]))
	var bsiz C.size_t = C.size_t(len(script))
	ret := C.luaL_loadbufferx(l.ptr, bptr, bsiz, nil, nil)
	if ret != C.LUA_OK {
		err = fmt.Errorf("load error : %s", l.ToStr(1))
		l.Pop(1)
	}
	return err
}

func (l *luastate) Exec(script []byte) (err error) {
	defer func() {
		if rerr, ok := recover().(error); ok {
			err = rerr
			l.cancel()
		}
	}()
	var bptr *C.char = (*C.char)(unsafe.Pointer(&script[0]))
	var bsiz C.size_t = C.size_t(len(script))
	ret := C.luaL_loadbufferx(l.ptr, bptr, bsiz, nil, nil)
	if ret != C.LUA_OK {
		err = fmt.Errorf("load error : %s", l.ToStr(1))
		l.Pop(1)
		return err
	}
	if err != nil {
		return err
	}
	ret = C.lua_pcallk(l.ptr, 0, C.LUA_MULTRET, 0, 0, nil)
	if ret != C.LUA_OK {
		err = fmt.Errorf("call error : %s", l.ToStr(1))
		l.Pop(1)
	}
	return err
}

func (l *luastate) OnStep() {
	select {
	case <-l.ctx.Done():
		//运行时间限制
		if err := l.ctx.Err(); err != nil {
			panic(err)
		}
	default:
		//运行步数限制
		if l.limit > 0 && l.step >= l.limit {
			panic(fmt.Errorf("%w %d/%d", ErrStepLimit, l.step, l.limit))
		}
		l.step++
	}
}

func (l *luastate) SetLimit(limit int) ILuaState {
	l.step = 0
	l.limit = limit
	return l
}

func (l *luastate) OpenLibs() {
	C.luaL_openlibs(l.ptr)
}

func (l *luastate) SetFunc(name string, fn LuaFunc) ILuaState {
	str := tocharptr(name)
	C.go_lua_set_global_func(l.ptr, str, unsafe.Pointer(&fn))
	return l
}

type LuaFunc func(l ILuaState) int

var (
	mapkey = struct{}{}
)

type ctxmap map[string]interface{}

func mustGetMap(ctx context.Context) ctxmap {
	p := ctx.Value(mapkey)
	if p == nil {
		panic(fmt.Errorf("miss ctxmap 1"))
	}
	v, ok := p.(ctxmap)
	if !ok {
		panic(fmt.Errorf("miss ctxmap 2"))
	}
	return v
}

func NewMapContext() context.Context {
	return context.WithValue(context.Background(), mapkey, ctxmap{})
}

const (
	//限制map元素数量
	MapSizeLimit = 16
)

func map_set(l ILuaState) int {
	ctx := l.Context()
	top := l.GetTop()
	if top != 2 {
		panic(fmt.Errorf("args key value error"))
	}
	if !l.IsStr(1) {
		panic(fmt.Errorf("args key type error"))
	}
	cmap := mustGetMap(ctx)
	if len(cmap) >= MapSizeLimit {
		panic(fmt.Errorf("map size limit %d", MapSizeLimit))
	}
	cmap[l.ToStr(1)] = l.ToValue(2)
	return 0
}

func map_get(l ILuaState) int {
	ctx := l.Context()
	top := l.GetTop()
	if top != 1 {
		panic(fmt.Errorf("args key value error"))
	}
	if !l.IsStr(1) {
		panic(fmt.Errorf("args key type error"))
	}
	cmap := mustGetMap(ctx)
	if len(cmap) >= MapSizeLimit {
		panic(fmt.Errorf("map size limit %d", MapSizeLimit))
	}
	pv, has := cmap[l.ToStr(1)]
	if !has {
		l.PushNil()
		return 1
	}
	l.PushValue(pv)
	return 1
}

func map_has(l ILuaState) int {
	ctx := l.Context()
	top := l.GetTop()
	if top != 1 {
		panic(fmt.Errorf("args key value error"))
	}
	if !l.IsStr(1) {
		panic(fmt.Errorf("args key type error"))
	}
	cmap := mustGetMap(ctx)
	if len(cmap) >= MapSizeLimit {
		panic(fmt.Errorf("map size limit %d", MapSizeLimit))
	}
	_, has := cmap[l.ToStr(1)]
	l.PushBool(has)
	return 1
}

const (
	AttrMapSet = 1 << 0
	AttrMapGet = 1 << 1
)

func NewLuaState(ctx context.Context, exp time.Duration, attr ...int) ILuaState {
	l := &luastate{}
	l.ptr = C.luaL_newstate()
	l.ptr.goptr = unsafe.Pointer(l)
	l.ctx, l.cancel = context.WithTimeout(ctx, exp)
	C.go_lua_state_init(l.ptr)
	runtime.SetFinalizer(l, func(obj interface{}) {
		l := obj.(ILuaState)
		l.Close()
	})
	if len(attr) > 0 && attr[0]&AttrMapSet != 0 {
		l.SetFunc("map_set", map_set)
	}
	if len(attr) > 0 && attr[0]&AttrMapGet != 0 {
		l.SetFunc("map_get", map_get)
		l.SetFunc("map_has", map_has)
	}
	return l
}
