package main

import (
	"flag"

	"github.com/cxuhua/xginx"
)

//实现自己的监听器
type mylist struct {
}

func (lis *mylist) OnInit(bi *xginx.BlockIndex) error {
	panic("implement me")
}

func (lis *mylist) TimeNow() uint32 {
	panic("implement me")
}

func (lis *mylist) OnUnlinkBlock(blk *xginx.BlockInfo) {
	panic("implement me")
}

func (lis *mylist) OnLinkBlock(blk *xginx.BlockInfo) {
	panic("implement me")
}

func (lis *mylist) OnNewBlock(blk *xginx.BlockInfo) error {
	panic("implement me")
}

func (lis *mylist) OnFinished(blk *xginx.BlockInfo) error {
	panic("implement me")
}

func (lis *mylist) OnClientMsg(c *xginx.Client, msg xginx.MsgIO) {
	panic("implement me")
}

func (lis *mylist) OnLoadTxs(txs []*xginx.TX) []*xginx.TX {
	panic("implement me")
}

func (lis *mylist) OnClose() {
	panic("implement me")
}

func (lis *mylist) OnTxPool(tx *xginx.TX) error {
	panic("implement me")
}

func (lis *mylist) OnTxPoolRep(old *xginx.TX, new *xginx.TX) {
	panic("implement me")
}

func (lis *mylist) MinerAddr() xginx.Address {
	panic("implement me")
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
