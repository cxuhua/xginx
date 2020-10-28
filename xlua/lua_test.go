package xlua

import (
	"context"
	"log"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	go func() {
		defer func() {
			err := recover()
			if err != nil {
				log.Println(err)
			}
		}()
		l := NewLuaState(ctx)
		defer l.Close()

		l.OpenLibs()

		l.SetFunc("test", func(l ILuaState) {
			time.Sleep(time.Second)
			log.Println("sleep")
		})
		ret := l.Exec([]byte(`for i=10,1,-1 do test() end`))
		log.Println("ret=", ret)
		cancel()
	}()
	<-ctx.Done()
	log.Println(ctx.Err())
	time.Sleep(time.Second * 30)
}
