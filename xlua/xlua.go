package xlua

import "C"
import (
	"fmt"
	"unsafe"
)

//使用:go tool cgo lua.go 生成导出给c调用
//export Panic
func Panic(msg string) {
	panic(fmt.Errorf(msg))
}

//export CallGoFunc
func CallGoFunc(s unsafe.Pointer, f unsafe.Pointer) {
	sp := (*luastate)(s)
	(*(*LuaFunc)(f))(sp)
}

//export LuaOnStep
func LuaOnStep(s unsafe.Pointer) {
	sp := (*luastate)(s)
	sp.OnStep()
}
