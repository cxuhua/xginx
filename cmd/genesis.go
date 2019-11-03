package cmd

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
	_ = gv.GetStore().UseSession(ctx, func(db gv.DBImp) error {
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go gv.CreateGenesisBlock(db, wg, ctx, cancel)
		}
		return nil
	})
	wg.Wait()
}
