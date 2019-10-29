package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cxuhua/xginx"
)

func main() {
	csig := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := xginx.NewServer(ctx, xginx.NodeConfig())
	if err != nil {
		panic(err)
	}
	s.Run()

	time.Sleep(time.Second * 1)

	d := xginx.NetAddrForm("127.0.0.1:9333")
	c := s.NewClient()
	err = c.Open(d)

	log.Println(err)

	c.Loop()
	//
	//time.Sleep(time.Second * 5)
	//
	//c.Close()

	signal.Notify(csig, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	sig := <-csig
	log.Println("recv sig :", sig, ",system exited")
	s.Stop()
	s.Wait()
}
