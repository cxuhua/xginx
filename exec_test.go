package xginx

import (
	"context"
	"log"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func init() {
	//测试模式下开启
	DebugScript = true
	SuccessScript = []byte(`
		print(out_value);
		print(tx_opt == OptPushTxPool);
		return ExecOK;
	`)
}

func TestLuaExec(t *testing.T) {
	opts := lua.Options{
		CallStackSize:       64,
		MinimizeStackMemory: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	l := lua.NewState(opts)
	l.SetContext(ctx)
	defer cancel()
	defer l.Close()
	initHttpLuaEnv(l)
	//最终结果 ERROR = "" 成功
	l.SetGlobal("result", lua.LString("error"))
	//交易版本
	l.SetGlobal("tx_ver", lua.LNumber(1))
	//当前区块高度和世界OK
	l.SetGlobal("best_height", lua.LNumber(2))
	l.SetGlobal("best_time", lua.LNumber(201))
	//交易操作
	l.SetGlobal("tx_opt", lua.LNumber(3))
	//当前系统时间
	l.SetGlobal("sys_time", lua.LNumber(4))
	var err error = nil
	log.Println(time.Now())
	fn, err := l.LoadString(`
		local obj,err = http.get('https://www.baidu.com');
		if err ~= nil then return err; end
		print(obj);
		print('tx_ver=',tx_ver);
		print('best_height=',best_height);
		print('best_time=',best_time);
		print('tx_opt=',tx_opt);
		print('sys_time=',sys_time);
		result = "OK";
	`)
	if err != nil {
		panic(err)
	}
	l.Push(fn)

	err = execScript(l)
	log.Println(err)
}
