package main

import (
	"flag"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"net/url"
	"time"

	"github.com/cxuhua/xginx"
)

var (
	//rpc设置
	xrpc = flag.String("rpc", "tcp://127.0.0.1:9334", "rpc addr port")
)

//实现自己的监听器
type shoplistener struct {
	//文档处理db
	docdb xginx.IDocSystem
	//密钥db
	keydb xginx.IKeysDB
	//rpc listener
	rpclis net.Listener
}

//OnTxPool 当交易进入交易池之前，返回错误不会进入交易池
func (lis *shoplistener) OnTxPool(tx *xginx.TX) error {
	return nil
}

//OnLoadTxs 当加载交易时,在AddTxs之前
func (lis *shoplistener) OnLoadTxs(txs []*xginx.TX) []*xginx.TX {
	return txs
}

//OnTxPoolRep 当交易被替换
func (lis *shoplistener) OnTxPoolRep(old *xginx.TX, new *xginx.TX) {
	xginx.LogInfof("TX = %v Replace %v", new.MustID(), old.MustID())
}

func (lis *shoplistener) OnLinkBlock(blk *xginx.BlockInfo) {

}

func (lis *shoplistener) OnNewBlock(blk *xginx.BlockInfo) error {
	return xginx.DefaultNewBlock(lis, blk)
}

//完成区块，当检测完成调用,设置merkle之前
func (lis *shoplistener) OnFinished(blk *xginx.BlockInfo) error {
	return xginx.DefaultkFinished(blk)
}

//OnClientMsg 收到网络信息
func (lis *shoplistener) OnClientMsg(c *xginx.Client, msg xginx.MsgIO) {
	//LogInfo(msg.Type())
}

//TimeNow 当前时间戳获取
func (lis *shoplistener) TimeNow() uint32 {
	return uint32(time.Now().Unix())
}

//OnUnlinkBlock 区块断开
func (lis *shoplistener) OnUnlinkBlock(blk *xginx.BlockInfo) {

}

//MinerAddr 返回矿工地址
func (lis shoplistener) MinerAddr() xginx.Address {
	bb, err := lis.keydb.GetConfig(MinerAddressKey)
	if err == nil {
		return xginx.Address(bb)
	}
	addr, err := lis.keydb.NewAddressInfo("miner account")
	if err != nil {
		panic(err)
	}
	aid, err := addr.ID()
	if err != nil {
		panic(err)
	}
	err = lis.keydb.PutConfig(MinerAddressKey, []byte(aid))
	if err != nil {
		panic(err)
	}

	return aid
}

//OnInit 首次启动初始化
func (lis *shoplistener) OnInit(bi *xginx.BlockIndex) error {
	conf := xginx.GetConfig()
	docs, err := xginx.OpenDocSystem(conf.DataDir + "/docs")
	if err != nil {
		return err
	}
	lis.docdb = docs
	keys, err := xginx.OpenKeysDB(conf.DataDir + "/keys")
	if err != nil {
		return err
	}
	lis.keydb = keys
	xginx.LogInfo("miner address:", lis.MinerAddr())
	return nil
}

var (
	lis = &shoplistener{}
)

//这里注册rpc接口
func (lis *shoplistener) registerrpc() {
	err := rpc.Register(&ShopApi{lis: lis})
	if err != nil {
		panic(err)
	}
}

func (lis *shoplistener) startrpc(conn string) {
	urlv, err := url.Parse(conn)
	if err != nil {
		panic(err)
	}
	lis.registerrpc()
	rpclis, err := net.Listen("tcp", urlv.Host)
	if err != nil {
		panic(err)
	}
	lis.rpclis = rpclis
	go xginx.ListenerLoopAccept(lis.rpclis, func(conn net.Conn) error {
		go jsonrpc.ServeConn(conn)
		return nil
	}, func(err error) {
		xginx.LogError(err)
	})
}

func (lis *shoplistener) OnStart() {
	//启动rpc
	if *xrpc != "" {
		lis.startrpc(*xrpc)
	}
}

func (lis *shoplistener) OnStop() {

}

func (lis *shoplistener) OnClose() {
	if lis.docdb != nil {
		lis.docdb.Close()
	}
	if lis.keydb != nil {
		lis.keydb.Close()
	}
	if lis.rpclis != nil {
		_ = lis.rpclis.Close()
	}
}

func main() {
	flag.Parse()
	xginx.Run(lis)
}
