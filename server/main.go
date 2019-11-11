package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	gv "github.com/cxuhua/xginx"
)

func main() {
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
}
