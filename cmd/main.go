package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	gv "github.com/cxuhua/xginx"
)

func createFistBlock() {
	runtime.GOMAXPROCS(4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go gv.CreateGenesisBlock(wg, ctx, cancel)
	}
	wg.Wait()
}

func main() {

	createFistBlock()

	defer gv.Close()
	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	//是否启动tcp节点服务器
	if gv.Server != nil {
		gv.Server.Start(ctx)
	}
	//是否启动矿工结算
	if gv.Miner != nil {
		gv.Miner.Start(ctx)
	}
	//是否启动http服务
	if gv.Http != nil {
		gv.Http.Start(ctx)
	}
	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig
	cancel()
	log.Println("recv sig :", sig, ",system exited")
	if gv.Server != nil {
		gv.Server.Stop()
		gv.Server.Wait()
	}
	if gv.Miner != nil {
		gv.Miner.Stop()
		gv.Miner.Wait()
	}
	if gv.Http != nil {
		gv.Http.Stop()
		gv.Http.Wait()
	}
}
