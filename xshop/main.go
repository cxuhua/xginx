package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/graphql-go/handler"

	"github.com/cxuhua/xginx"
)

var (
	//rpc设置
	gqlset = flag.String("graphql", "graphql://127.0.0.1:9334", "graphql addr port")
)

const (
	MinerAddressKey = "__miner_address_key__"
)

//实现自己的监听器
type shoplistener struct {
	//文档处理db
	docdb xginx.IDocSystem
	//密钥db
	keydb xginx.IKeysDB
	//graphql http
	gqlsrv *http.Server
	//
	gqlhandler *handler.Handler
	wg         sync.WaitGroup
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
func (lis *shoplistener) MinerAddr() xginx.Address {
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

const (
	objkeyblockkey = "objkeyblockindex"
	objdocdbkey    = "objdocdbkey"
	objkeydbkey    = "objkeydbkey"
)

type Objects map[string]interface{}

func (objs Objects) BlockIndex() *xginx.BlockIndex {
	v, ok := objs[objkeyblockkey].(*xginx.BlockIndex)
	if !ok {
		panic(fmt.Errorf("block index miss"))
	}
	return v
}

func (objs Objects) KeyDB() xginx.IKeysDB {
	v, ok := objs[objkeydbkey].(xginx.IKeysDB)
	if !ok {
		panic(fmt.Errorf("key db miss"))
	}
	return v
}

func (objs Objects) DocDB() xginx.IDocSystem {
	v, ok := objs[objdocdbkey].(xginx.IDocSystem)
	if !ok {
		panic(fmt.Errorf("doc db  miss"))
	}
	return v
}

//返回可用的素有对象
func NewObjects(ctx context.Context) Objects {
	return Objects{}
}

//http Handler
func (lis *shoplistener) ServeHTTP(rw http.ResponseWriter, q *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			_, _ = fmt.Fprintf(rw, "recover : %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}()
	lis.gqlhandler.ServeHTTP(rw, q)
}

func (lis *shoplistener) startgraphql(host string) {
	urlv, err := url.Parse(host)
	if err != nil {
		xginx.LogError(err)
		return
	}
	lis.gqlhandler = handler.New(&handler.Config{
		Schema:   GetSchema(),
		Pretty:   *xginx.IsDebug,
		GraphiQL: *xginx.IsDebug,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			objs := NewObjects(ctx)
			objs[objkeyblockkey] = xginx.GetBlockIndex()
			objs[objdocdbkey] = lis.docdb
			objs[objkeydbkey] = lis.keydb
			return objs
		},
	})
	mux := http.NewServeMux()
	mux.Handle("/"+urlv.Scheme, lis)
	lis.gqlsrv = &http.Server{
		Addr:    urlv.Host,
		Handler: mux,
		BaseContext: func(listener net.Listener) context.Context {
			return xginx.GetContext()
		},
	}
	lis.wg.Add(1)
	go func() {
		defer lis.wg.Done()
		err := lis.gqlsrv.ListenAndServe()
		if err != nil {
			xginx.LogError(err)
		}
	}()
}

func (lis *shoplistener) OnStart() {
	//启动graphql服务
	lis.startgraphql(*gqlset)
}

func (lis *shoplistener) OnStop() {
	//停止graphql服务
	if lis.gqlsrv != nil {
		ctx, cancel := context.WithTimeout(xginx.GetContext(), time.Second*30)
		defer cancel()
		_ = lis.gqlsrv.Shutdown(ctx)
	}
	//等待结束
	lis.wg.Wait()
}

func (lis *shoplistener) OnClose() {
	if lis.docdb != nil {
		lis.docdb.Close()
	}
	if lis.keydb != nil {
		lis.keydb.Close()
	}
}

func main() {
	flag.Parse()
	xginx.Run(lis)
}
