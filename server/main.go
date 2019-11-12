package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	xx "github.com/cxuhua/xginx"
)

func main() {
	defer xx.Close()
	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	//是否启动tcp节点服务器
	if xx.Server != nil {
		xx.Server.Start(ctx)
	}
	//是否启动矿工结算
	if xx.Miner != nil {
		xx.Miner.Start(ctx)
	}
	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig
	cancel()
	log.Println("recv sig :", sig, ",system exited")
	if xx.Server != nil {
		xx.Server.Stop()
		xx.Server.Wait()
	}
	if xx.Miner != nil {
		xx.Miner.Stop()
		xx.Miner.Wait()
	}
}
