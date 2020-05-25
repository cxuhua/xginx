package xginx

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	ctx    context.Context
	cancel context.CancelFunc
)

//GetContext 获取区块链服务context
func GetContext() (context.Context, context.CancelFunc) {
	return ctx, cancel
}

//Run 启动区块链服务
func Run(lis IListener) {
	if !flag.Parsed() {
		flag.Parse()
	}
	conf := InitConfig()
	defer conf.Close()

	LogInfof("xginx run config name = %s debug=%b", conf.Name, *IsDebug)

	ps := GetPubSub()
	defer ps.Shutdown()

	bi := InitBlockIndex(lis)
	defer bi.Close()

	csig := make(chan os.Signal)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	Server.Start(ctx, lis)

	Miner.Start(ctx, lis)

	//延迟回调
	time.Sleep(time.Millisecond * 300)
	lis.OnStart()

	//等候关闭信号
	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig

	lis.OnStop(sig)

	cancel()
	LogInfo("recv sig :", sig, ",system start exit")

	Server.Stop()
	LogInfo("wait server stop")
	Server.Wait()

	Miner.Stop()
	LogInfo("wait miner stop")
	Miner.Wait()
}
