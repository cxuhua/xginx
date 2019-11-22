package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	. "github.com/cxuhua/xginx"
)

func main() {
	conf := InitConfig("v10000.json")
	defer conf.Close()

	pubsub := GetPubSub()
	defer pubsub.Shutdown()

	lis := newListener(conf.WalletDir)

	bi := InitBlockIndex(lis)
	defer bi.Close()

	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//是否启动tcp节点服务器
	if Server != nil {
		Server.Start(ctx, lis)
	}
	//是否启动矿工
	if Miner != nil {
		Miner.Start(ctx, lis)
	}
	//启动http服务
	if Http != nil {
		Http.Start(ctx, lis)
	}
	//延迟回调
	time.Sleep(time.Millisecond * 200)
	lis.OnStartup()

	//测试5秒一个
	//go func() {
	//	for {
	//		ps := GetPubSub()
	//		ps.Pub(MinerAct{
	//			Opt: OptGenBlock,
	//			Arg: uint32(1),
	//		}, NewMinerActTopic)
	//		time.Sleep(time.Second * 5)
	//	}
	//}()
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
