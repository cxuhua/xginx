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

var (
	cfile  = flag.String("conf", "v10000.json", "config file name")
	user   = flag.String("user", "", "admin user")
	pass   = flag.String("pass", "", "admin pass")
	isinit = flag.Bool("init", false, "init admin info")
)

func initdb(conf *Config) {
	wallet, err := NewLevelDBWallet(conf.WalletDir)
	if err != nil {
		panic(err)
	}
	err = wallet.InitAdminInfo(*user, *pass, 0)
	if err != nil {
		panic(err)
	}
	LogInfo("inited admin info")
	addr, err := wallet.NewAccount(3, 2, false)
	if err != nil {
		panic(err)
	}
	err = wallet.SetMiner(addr)
	if err != nil {
		panic(err)
	}
	LogInfo("inited miner account", addr)
	wallet.Close()
}

func main() {
	flag.Parse()
	if *cfile == "" {
		panic("config file miss")
	}
	conf := InitConfig(*cfile)
	defer conf.Close()
	if *isinit {
		initdb(conf)
		return
	}
	pubsub := GetPubSub()
	defer pubsub.Shutdown()

	lis := newListener(conf.WalletDir)

	bi := InitBlockIndex(lis)
	defer bi.Close()

	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	Server.Start(ctx, lis)

	Miner.Start(ctx, lis)

	Http.Start(ctx, lis)

	//延迟回调
	time.Sleep(time.Millisecond * 500)
	lis.OnStartup()

	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig
	cancel()
	LogInfo("recv sig :", sig, ",system exited")

	Server.Stop()
	LogInfo("wait server stop")
	Server.Wait()

	Miner.Stop()
	LogInfo("wait miner stop")
	Miner.Wait()

	Http.Stop()
	LogInfo("wait http stop")
	Http.Wait()
}
