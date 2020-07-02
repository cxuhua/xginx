package xginx

import (
	"context"
	"log"
	"math"
	"testing"
)

func init() {
	//测试模式下开启
	*IsDebug = true

	DefaultTxScript = []byte(`
	return true
`)

	DefaultInputScript = []byte(`
	return true
`)

	DefaultLockedScript = []byte(`
	--获取引用的交易
	local rtx = get_rtx();
	print(encode(rtx));
	--获取当前交易
	local tx = get_tx();
	--获取当前输入引用的输出交易
	local otx = get_tx(tx.sign.inv.out_hash);
	--打印测试
	--print(tx.sign.inv.address,tx.sign.out.address);
	--只有输入地址和输出地址一致并且签名正确才能解锁当前金额
	return verify_addr() and verify_sign();
`)

}

func TestFloatVal(t *testing.T) {
	v := float64(1.000000000)
	i, b := math.Modf(v)
	log.Println(i, b == 0)
}

func TestCheckScript(t *testing.T) {
	err := CheckScript(DefaultInputScript)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckScript([]byte(`&763743`))
	if err == nil {
		t.Fatal("error script ")
	}
}

func TestTransMap(t *testing.T) {
	ctx = context.WithValue(context.Background(), transKey, newTransOutMap(ctx))
	l := newScriptEnv(ctx)
	defer l.Close()

	l.SetGlobal("map_set", l.NewFunction(transMapValueSet))
	l.SetGlobal("map_get", l.NewFunction(transMapValueGet))

	err := l.DoString(`
		map_set('k1',1);
		map_set('k2',true);
		map_set('k3','kstring');
		map_set('k4',1.55);
	`)
	if err != nil {
		t.Fatal(err)
	}
	err = l.DoString(`map_set('k5',{});`)
	if err != nil {
		t.Fatal(err)
	}
	err = l.DoString(`
		print(map_get('k1'));
		print(map_get('k2'));
		print(map_get('k3'));
		print(map_get('k4'));
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJsonTable(t *testing.T) {
	l := newScriptEnv(context.Background())
	defer l.Close()
	jv := `{"a":1,"b":"1234567890","c":true,"d":1.1,"arr":[1,2,3,4,5,6]}`
	tbl, err := jsonToTable(l, []byte(jv))
	if err != nil {
		panic(err)
	}
	if tableIsArray(tbl) {
		t.Fatal("is object")
	}
	jvv, err := tableToJSON(tbl)
	if err != nil {
		t.Fatal(err)
	}
	log.Println(string(jvv))
}