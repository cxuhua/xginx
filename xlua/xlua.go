package xlua

import "C"
import (
	"fmt"
	"unsafe"
)

//使用:go tool cgo xlua.go 生成导出给c调用
//export Panic
func Panic(msg string) {
	panic(fmt.Errorf(msg))
}

//export CallGoFunc
func CallGoFunc(s unsafe.Pointer, f unsafe.Pointer) int {
	sp := (*luastate)(s)
	return (*(*LuaFunc)(f))(sp)
}

//export LuaOnStep
func LuaOnStep(s unsafe.Pointer) {
	sp := (*luastate)(s)
	sp.OnStep()
}
