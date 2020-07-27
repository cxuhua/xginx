package main

import (
	"flag"

	"github.com/cxuhua/xginx"
)

//实现自己的监听器
type mylist struct {
	xginx.Listener
}

func (lis *mylist) OnStart() {
	xginx.LogInfo("my system start")
}

func (lis *mylist) OnStop() {

}

func main() {
	flag.Parse()
	xginx.Run(&mylist{})
}
