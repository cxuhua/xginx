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
	shopdb *ShopDB
	xginx.Listener
	rpclis net.Listener
}

//MinerAddr 返回矿工地址，默认返回配置中的
func (lis shoplistener) MinerAddr() xginx.Address {
	return lis.shopdb.GetMinerAddr()
}

var (
	lis = &shoplistener{}
)

//GetShopDB 获取商店数据库
func GetShopDB() *ShopDB {
	return lis.shopdb
}

//这里注册rpc接口
func (lis *shoplistener) registerrpc() {
	err := rpc.Register(&ShopApi{lis: lis})
	if err != nil {
		panic(err)
	}
}

func (lis *shoplistener) looprpc() {
	xginx.LogInfo("start json rpc server")
	var delay time.Duration
	for {
		conn, err := lis.rpclis.Accept()
		if err == nil {
			delay = 0
			go jsonrpc.ServeConn(conn)
			continue
		}
		if ne, ok := err.(net.Error); ok && ne.Temporary() {
			if delay == 0 {
				delay = 5 * time.Millisecond
			} else {
				delay *= 2
			}
			if max := 3 * time.Second; delay > max {
				delay = max
			}
			xginx.LogWarn("Accept warn: %v; retrying in %v", err, delay)
			time.Sleep(delay)
			continue
		} else {
			xginx.LogError(err)
			break
		}
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
	go func() {
		lis.looprpc()
	}()
}

func (lis *shoplistener) OnStart() {
	conf := xginx.GetConfig()
	docs, err := xginx.OpenDocSystem(conf.DataDir + "/docs")
	if err != nil {
		panic(err)
	}
	keys, err := xginx.OpenKeysDB(conf.DataDir + "/keys")
	if err != nil {
		panic(err)
	}
	lis.shopdb = &ShopDB{
		DocDB: docs,
		KeyDB: keys,
		bi:    xginx.GetBlockIndex(),
	}
	//启动rpc
	if *xrpc != "" {
		lis.startrpc(*xrpc)
	}
}

func (lis *shoplistener) OnStop() {
	if lis.shopdb != nil {
		lis.shopdb.DocDB.Close()
		lis.shopdb.KeyDB.Close()
	}
	if lis.rpclis != nil {
		_ = lis.rpclis.Close()
	}
}

func main() {
	flag.Parse()
	xginx.Run(lis)
}
