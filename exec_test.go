package xginx

import (
	"log"
	"testing"

	lua "github.com/cxuhua/gopher-lua"
)

func init() {
	//测试模式下开启
	DebugScript = true
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

func TestJsonTable(t *testing.T) {
	opts := lua.Options{
		CallStackSize:   16,
		RegistrySize:    128,
		RegistryMaxSize: 0,
		SkipOpenLibs:    !DebugScript,
	}
	l := lua.NewState(opts)

	jv := `{"a":1,"b":"1234567890","c":true,"d":1.1,"arr":[1,2,3,4,5,6]}`
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
