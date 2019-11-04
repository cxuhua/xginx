package main

import (
	"context"
	"runtime"
	"sync"

	gv "github.com/cxuhua/xginx"
)

func main() {
	runtime.GOMAXPROCS(4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			_ = gv.GetStore().UseSession(ctx, func(db gv.DBImp) error {
				return gv.CreateGenesisBlock(db, wg, ctx, cancel)
			})
		}()
	}
	wg.Wait()
}
