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
	"fmt"
	"log"
	"runtime"
	"unsafe"
)

type ILuaState interface {
	//关闭状态机
	Close()
	//打开所有标准库
	OpenLibs()
	//加载执行脚本
	Exec(script []byte) error
	//获取环境
	Context() context.Context
	//注册全局函数
	SetFunc(name string, fn LuaFunc)
}

type luastate struct {
	step int64
	ctx  context.Context
	ptr  *C.lua_State
}

func (l *luastate) Context() context.Context {
	return l.ctx
}

func (l *luastate) Close() {
	if l.ptr != nil {
		C.lua_close(l.ptr)
		l.ptr = nil
	}
}

func (l *luastate) Top() int {
	return int(C.lua_gettop(l.ptr))
}

func (l *luastate) ToStr(idx int) string {
	sl := C.size_t(0)
	str := C.lua_tolstring(l.ptr, C.int(idx), &sl)
	return C.GoString(str)
}

func (l *luastate) ToBytes(idx int) []byte {
	sl := C.size_t(0)
	src := C.lua_tolstring(l.ptr, C.int(idx), &sl)
	ret := make([]byte, int(sl))
	dst := unsafe.Pointer(&ret[0])
	C.memcpy(dst, unsafe.Pointer(src), sl)
	return ret
}

func (l *luastate) Exec(script []byte) error {
	if len(script) == 0 {
		return nil
	}
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
		if err := l.ctx.Err(); err != nil {
			panic(err)
		}
	default:
		l.step++
		log.Println(l.step)
	}
}

func (l *luastate) OpenLibs() {
	C.luaL_openlibs(l.ptr)
}

func (l *luastate) SetFunc(name string, fn LuaFunc) {
	str := C.CString(name)
	C.go_lua_set_func(l.ptr, str, unsafe.Pointer(&fn))
	C.free(unsafe.Pointer(str))
}

type LuaFunc func(l ILuaState)

func NewLuaState(ctx context.Context) ILuaState {
	l := &luastate{}
	l.ptr = C.luaL_newstate()
	if l.ptr == nil {
		panic(fmt.Errorf("new lua state error"))
	}
	C.go_lua_state_init(l.ptr, unsafe.Pointer(l))
	runtime.SetFinalizer(l, func(obj interface{}) {
		l := obj.(ILuaState)
		l.Close()
	})
	l.ctx = ctx
	return l
}
