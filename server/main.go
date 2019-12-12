package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	. "github.com/cxuhua/xginx"
)

func main() {
	flag.Parse()

	conf := InitConfig()
	defer conf.Close()

	ps := GetPubSub()
	defer ps.Shutdown()

	lis := newListener(conf)

	bi := InitBlockIndex(lis)
	defer bi.Close()

	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	Server.Start(ctx, lis)

	Miner.Start(ctx, lis)

	time.Sleep(time.Millisecond * 300)
	lis.OnStartup()

	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig

	cancel()
	LogInfo("recv sig :", sig, ",system start exit")

	Server.Stop()
	LogInfo("wait server stop")
	Server.Wait()

	Miner.Stop()
	LogInfo("wait miner stop")
	Miner.Wait()

	LogInfo("system exited")
}
