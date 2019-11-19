package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	. "github.com/cxuhua/xginx"
)

func connect() {
	c := Server.NewClient()
	err := c.Open(NetAddrForm("192.168.31.178:9333"))
	if err == nil {
		c.Loop()
	}
}

func main() {
	conf := InitConfig("v10000.json")
	defer conf.Close()

	pubsub := GetPubSub()
	defer pubsub.Shutdown()

	lis := newListener(conf.WalletDir)

	bi := InitBlockIndex(lis)
	defer bi.Close()

	//bi.UnlinkLast()

	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//是否启动tcp节点服务器
	if Server != nil {
		Server.Start(ctx)
		time.Sleep(time.Second)
		connect()
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
