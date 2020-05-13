package xginx

import (
	"log"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestLuaExec(t *testing.T) {
	l := initLuaEnv(false)
	//最终结果 ERROR = "" 成功
	l.SetGlobal("ERROR", lua.LString(""))
	//脚本版本
	l.SetGlobal("VER", lua.LNumber(1))
	//当前区块高度
	l.SetGlobal("HEIGHT", lua.LNumber(2))
	//交易操作号
	l.SetGlobal("TX_OPT", lua.LNumber(3))
	//当前时间戳
	l.SetGlobal("TIME", lua.LNumber(4))

	err := l.DoString(`
		lt = 99;
		print(VER);
		print(HEIGHT);
		print(TX_OPT);
		print(TIME);
		ERROR = "e1";
	`)
	log.Println(err)
	log.Println("ERROR1=", l.GetGlobal("ERROR"))
	err = l.DoString(`
		print(lt);
		print(TIME);
		ERROR = "e2";
	`)
	log.Println(err)
	log.Println("ERROR2=", l.GetGlobal("ERROR"))
	defer l.Close()
}
