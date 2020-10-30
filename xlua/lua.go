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
	TNIL     = C.LUA_TNIL
	TBOOLEAN = C.LUA_TBOOLEAN
	TNUMBER  = C.LUA_TNUMBER
	TSTRING  = C.LUA_TSTRING
	TTABLE   = C.LUA_TTABLE
)

type ITable interface {
}

type table struct {
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
	//获取环境
	Context() context.Context
	//注册全局函数,函数不能引用其他go指针数据
	SetFunc(name string, fn LuaFunc)
	//设置执行步限制
	SetLimit(limit int)
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
	ToValue(idx int) interface{}
	IsStr(idx int) bool
	IsInt(idx int) bool
	IsFloat(idx int) bool
	IsBool(idx int) bool
	IsTable(idx int) bool
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
	ForEach(fn func() bool)
	//是否是数组,并返回长度
	IsArray(idx int) (int, bool)
	Next(idx int) int
	TableLen(idx int) int
	SetTable(idx int)
	SetField(idx int, key string)
	GetTable(idx int) int
	GetField(idx int, key string) int
	//table opt
	SetValue(idx int, k string, v interface{})
	GetValue(idx int, k string) interface{}
	//
	SetArray(idx int, k int, v interface{})
	GetArray(idx int, k int) interface{}
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

func (l *luastate) ToValue(idx int) interface{} {
	if l.IsStr(idx) {
		return l.ToStr(idx)
	}
	if l.IsBool(idx) {
		return l.ToBool(idx)
	}
	if l.IsFloat(idx) {
		return l.ToFloat(idx)
	}
	if l.IsInt(idx) {
		return l.ToInt(idx)
	}
	panic(fmt.Errorf("type error"))
}

func (l *luastate) ToBool(idx int) bool {
	return C.lua_toboolean(l.ptr, C.int(idx)) != 0
}

func (l *luastate) IsStr(idx int) bool {
	return C.lua_isstring(l.ptr, C.int(idx)) != 0
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

func (l *luastate) Exec(script []byte) (err error) {
	defer func() {
		if rerr, ok := recover().(error); ok {
			err = rerr
		}
		l.cancel()
	}()
	var bptr *C.char = (*C.char)(unsafe.Pointer(&script[0]))
	var bsiz C.size_t = C.size_t(len(script))
	ret := C.luaL_loadbufferx(l.ptr, bptr, bsiz, nil, nil)
	if ret != C.LUA_OK {
		return fmt.Errorf("load error : %s", l.ToStr(1))
	}
	ret = C.lua_pcallk(l.ptr, 0, C.LUA_MULTRET, 0, 0, nil)
	if ret != C.LUA_OK {
		return fmt.Errorf("call error : %s", l.ToStr(1))
	}
	return nil
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

func (l *luastate) SetLimit(limit int) {
	l.step = 0
	l.limit = limit
}

func (l *luastate) OpenLibs() {
	C.luaL_openlibs(l.ptr)
}

func (l *luastate) SetFunc(name string, fn LuaFunc) {
	str := tocharptr(name)
	C.go_lua_set_global_func(l.ptr, str, unsafe.Pointer(&fn))
}

type LuaFunc func(l ILuaState) int

func NewLuaState(ctx context.Context, exp time.Duration) ILuaState {
	if ctx == nil {
		ctx = context.Background()
	}
	l := &luastate{}
	l.ptr = C.luaL_newstate()
	l.ptr.goptr = unsafe.Pointer(l)
	l.ctx, l.cancel = context.WithTimeout(ctx, exp)
	C.go_lua_state_init(l.ptr)
	runtime.SetFinalizer(l, func(obj interface{}) {
		l := obj.(ILuaState)
		l.Close()
	})
	return l
}
