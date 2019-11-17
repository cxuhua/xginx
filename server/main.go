package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	. "github.com/cxuhua/xginx"
)

func main() {

	pubsub := GetPubSub()
	defer pubsub.Shutdown()

	conf := InitConfig("v10000.json")
	defer conf.Close()

	lis := newListener(conf.WalletDir)

	bi := InitBlockIndex(lis)
	defer bi.Close()

	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//是否启动tcp节点服务器
	if Server != nil {
		Server.Start(ctx)
	}
	//是否启动矿工
	if Miner != nil {
		Miner.Start(ctx)
	}
	//启动http服务
	if Http != nil {
		Http.Start(ctx)
	}
	//
	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig
	cancel()
	LogInfo("recv sig :", sig, ",system exited")
	if Server != nil {
		Server.Stop()
		Server.Wait()
	}
	if Miner != nil {
		Miner.Stop()
		Miner.Wait()
	}
}
