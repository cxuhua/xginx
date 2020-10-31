package xlua

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestArray(t *testing.T) {
	l := NewLuaState(context.Background(), time.Second*3)
	defer l.Close()
	l.OpenLibs()

	assert.Equal(t, l.GetTop(), 0)
	l.NewTable()
	l.PushStr("11")
	assert.Equal(t, l.GetTop(), 2)
	l.SetArray(-2, 1, "11")
	l.SetArray(-2, 2, "22")
	l.SetArray(-2, 3, "33")
	assert.Equal(t, l.GetTop(), 2)
	ll, ok := l.IsArray(-2)
	assert.True(t, ok)
	assert.Equal(t, ll, 3)
	assert.Equal(t, l.GetTop(), 2)
	l.Pop(1)
	l.SetGlobal("tbl")
	assert.Equal(t, l.GetTop(), 0)
	err := l.Exec([]byte(`return tbl`))
	if err != nil {
		panic(err)
	}
	assert.Equal(t, l.GetTop(), 1)
	ll, ok = l.IsArray(-1)
	assert.True(t, ok)
	assert.Equal(t, ll, 3)
	assert.Equal(t, l.GetArray(-1, 3), "33")
	assert.Equal(t, l.GetArray(-1, 1), "11")
	assert.Equal(t, l.GetArray(-1, 2), "22")
	assert.Equal(t, l.GetTop(), 1)
	//test table
	tbl := l.ToTable(-1)
	vars := []string{}
	tbl.ForEach(func() bool {
		vars = append(vars, l.ToStr(-1))
		return true
	})
	assert.Equal(t, vars, []string{"11", "22", "33"})
	assert.Equal(t, l.GetTop(), 1)
	assert.Equal(t, tbl.Get(3), "33")
	assert.Equal(t, tbl.Get(1), "11")
	assert.Equal(t, tbl.Get(2), "22")
	tbl.Set(4, 4)
	assert.Equal(t, l.GetTop(), 1)
	assert.Equal(t, tbl.Get(4), int64(4))
}

func TestForEach(t *testing.T) {
	l := NewLuaState(context.Background(), time.Second*3)
	defer l.Close()
	l.OpenLibs()

	assert.Equal(t, l.GetTop(), 0)
	l.NewTable()
	assert.Equal(t, l.GetTop(), 1)
	l.SetValue(-1, "1", "11")
	l.SetValue(-1, "2", "22")
	l.SetValue(-1, "3", "33")

	l.SetGlobal("tbl")

	assert.Equal(t, l.GetTop(), 0)
	err := l.Exec([]byte(`return tbl`))
	if err != nil {
		panic(err)
	}
	assert.Equal(t, l.GetTop(), 1)
	ll, ok := l.IsArray(-1)
	assert.False(t, ok)
	assert.Equal(t, ll, 0)
	assert.Equal(t, l.GetTop(), 1)

	i := 0
	l.ForEach(func() bool {
		//key
		assert.Equal(t, l.GetType(-2), TSTRING)
		//value
		assert.Equal(t, l.GetType(-1), TSTRING)
		i++
		return true
	})
	assert.Equal(t, i, 3)
	assert.Equal(t, l.GetTop(), 1)
}

func TestTable(t *testing.T) {
	l := NewLuaState(context.Background(), time.Second*3)
	defer l.Close()
	l.OpenLibs()
	assert.Equal(t, l.GetTop(), 0)
	l.NewTable()
	assert.Equal(t, l.GetTop(), 1)
	l.SetValue(-1, "skey", "123")
	l.SetValue(-1, "mskey", "456")
	l.SetValue(-1, "fn", LuaFunc(func(l ILuaState) int {
		l.PushValue(11)
		return 1
	}))
	assert.Equal(t, l.GetTop(), 1)
	l.SetGlobal("tbl")
	assert.Equal(t, l.GetTop(), 0)
	err := l.Exec([]byte(`local fr = tbl.fn() return tbl,fr`))
	if err != nil {
		panic(err)
	}
	assert.Equal(t, l.GetTop(), 2)
	assert.True(t, l.IsTable(1))
	assert.True(t, l.IsInt(2))
	assert.Equal(t, l.GetTop(), 2)
	assert.Equal(t, l.GetValue(1, "skey"), "123")
	assert.Equal(t, l.GetValue(1, "mskey"), "456")
	assert.Equal(t, l.ToInt(2), int64(11))
	assert.Equal(t, l.GetTop(), 2)
}

func TestGetStack(t *testing.T) {
	l := NewLuaState(context.Background(), time.Second*3)
	defer l.Close()
	l.OpenLibs()
	err := l.Exec([]byte(`return true,1,1.1,'xx'`))
	if err != nil {
		panic(err)
	}
	require.Equal(t, l.GetTop(), 4)

	assert.True(t, l.IsBool(1))
	assert.Equal(t, l.ToBool(1), true)

	assert.True(t, l.IsInt(2))
	assert.Equal(t, l.ToInt(2), int64(1))

	assert.True(t, l.IsFloat(3))
	assert.Equal(t, l.ToFloat(3), float64(1.1))

	assert.True(t, l.IsStr(4))
	assert.Equal(t, l.ToStr(4), "xx")

	l.SetTop(0)
	require.Equal(t, l.GetTop(), 0)
	l.PushBool(false)
	l.PushInt(4)
	l.PushFloat(1.3)
	l.PushStr("vv")
	require.Equal(t, l.GetTop(), 4)

	assert.True(t, l.IsBool(1))
	assert.Equal(t, l.ToBool(1), false)

	assert.True(t, l.IsInt(2))
	assert.Equal(t, l.ToInt(2), int64(4))

	assert.True(t, l.IsFloat(3))
	assert.Equal(t, l.ToFloat(3), float64(1.3))

	assert.True(t, l.IsStr(4))
	assert.Equal(t, l.ToStr(4), "vv")

	l.SetTop(0)
	require.Equal(t, l.GetTop(), 0)
	//table test
	l.NewTable()
	assert.Equal(t, l.GetType(1), TTABLE)
}

func TestWhileLimit(t *testing.T) {
	l := NewLuaState(context.Background(), time.Second*3)
	l.SetLimit(100)
	defer l.Close()
	l.OpenLibs()
	err := l.Exec([]byte(`while true do end`))
	if !errors.Is(err, ErrStepLimit) {
		t.Fatal("ErrStepLimit error")
	}
	log.Println(l.GetStep())
}

func TestContext(t *testing.T) {
	m := map[string]interface{}{}
	m["11"] = "ok"
	ctx := context.WithValue(context.Background(), "11", m)
	l := NewLuaState(ctx, time.Second*3)
	defer l.Close()
	l.OpenLibs()
	l.SetFunc("test", func(l ILuaState) int {
		time.Sleep(time.Second)
		mm := l.Context().Value("11").(map[string]interface{})
		mm["tt"] = 222
		return 0
	})
	err := l.Exec([]byte(`test()`))
	assert.NoError(t, err)
	assert.Equal(t, m["tt"], 222)
}

func TestStepLimitErr(t *testing.T) {
	ctx := context.WithValue(context.Background(), "11", "22")
	l := NewLuaState(ctx, time.Second*3)
	l.SetLimit(3)
	defer l.Close()
	l.OpenLibs()
	l.SetFunc("test", func(l ILuaState) int {
		time.Sleep(time.Second)
		return 0
	})
	l.SetFunc("test2", func(l ILuaState) int {
		time.Sleep(time.Second)
		log.Println(l.Context().Value("11"))
		return 0
	})
	err := l.Exec([]byte(`for i=2,1,-1 do test("hellow") test2("22") end`))
	if !errors.Is(err, ErrStepLimit) {
		t.Fatal("ErrStepLimit error")
	}
}

func TestTimeout(t *testing.T) {
	l := NewLuaState(context.Background(), time.Second*3)
	defer l.Close()
	l.OpenLibs()
	l.SetFunc("test", func(l ILuaState) int {
		time.Sleep(time.Second * 1)
		return 0
	})
	l.SetFunc("test2", func(l ILuaState) int {
		time.Sleep(time.Second * 1)
		return 0
	})
	err := l.Exec([]byte(`for i=5,1,-1 do test("hellow") test2("22") end`))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("DeadlineExceeded error")
	}
}
