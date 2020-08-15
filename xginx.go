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
func GetContext() context.Context {
	return ctx
}

//Run 启动区块链服务
func Run(lis IListener) {
	if !flag.Parsed() {
		flag.Parse()
	}
	conf := InitConfig()

	LogInfof("xginx run config name = %s", conf.Name)

	ps := GetPubSub()
	defer ps.Shutdown()

	bi := InitBlockIndex(lis)

	csig := make(chan os.Signal)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	Server.Start(ctx, lis)

	Miner.Start(ctx, lis)

	//延迟回调
	time.Sleep(time.Millisecond * 300)
	lis.OnStart()
	//必定停止

	//等候关闭信号
	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig

	cancel()
	LogInfo("recv sig :", sig, ",system start exit")

	lis.OnStop()
	bi.Close()
	conf.Close()

	Server.Stop()
	LogInfo("wait server stop")
	Server.Wait()

	Miner.Stop()
	LogInfo("wait miner stop")
	Miner.Wait()
}
