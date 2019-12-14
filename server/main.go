package main

import (
	"flag"
	"os"

	"github.com/cxuhua/xginx"
)

//实现自己的监听器
type mylist struct {
	xginx.Listener
}

func (lis *mylist) OnStart() {
	xginx.LogInfo("my system start")
}

func (lis *mylist) OnStop(sig os.Signal) {
	xginx.LogInfo("my system stop sig=", sig)
}

func main() {
	flag.Parse()
	xginx.Run(&mylist{})
}
