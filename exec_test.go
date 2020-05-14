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

func TestJsonTable(t *testing.T) {
	opts := lua.Options{
		CallStackSize:       64,
		MinimizeStackMemory: true,
	}
	l := lua.NewState(opts)
	jv := `{"a":1,"b":"22","c":true,"d":1.1,"arr":[1,2,3,4,5,6]}`
	tbl, err := jsonToTable(l, []byte(jv))
	if err != nil {
		panic(err)
	}
	if tableIsArray(tbl) {
		log.Println("isarray")
	} else {
		log.Println("not isarray")
	}
	jvv, err := tableToJSON(tbl)
	if err != nil {
		panic(err)
	}
	log.Println(string(jvv))
}

func TestLuaExec(t *testing.T) {
	opts := lua.Options{
		CallStackSize:       64,
		MinimizeStackMemory: true,
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	l := lua.NewState(opts)
	l.SetContext(ctx)
	defer cancel()
	defer l.Close()
	initHTTPLuaEnv(l)
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
		local obj,err = http.post('http://127.0.0.1/manager/api/getCounter?id=test&sign=8830404b92f0f0fa2677cf53ece6b906&ts=1589423155&type=1',{a=1});
		if err ~= nil then return err; end
		print(obj.code,obj.msg);
		print('tx_ver=',tx_ver);
		print('best_height=',best_height);
		print('best_time=',best_time);
		print('tx_opt=',tx_opt);
		print('sys_time=',sys_time);
		return 'OK';
	`)
	if err != nil {
		panic(err)
	}
	l.Push(fn)

	err = execScript(l)
	log.Println(err)
}
