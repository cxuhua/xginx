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

	"github.com/graphql-go/graphql"

	"github.com/functionalfoundry/graphqlws"

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
	//文档db
	docdb xginx.IDocSystem
	//密钥db
	keydb xginx.IKeysDB
	//graphql http
	gqlsrv *http.Server
	//graphql接口描述
	gqlschema *graphql.Schema
	//graphqlhttp处理器
	gqlhandler *handler.Handler
	//graphql订阅管理
	gqlsubmgr graphqlws.SubscriptionManager
	//等待所有退出
	wg sync.WaitGroup
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
	lis.Publish(xginx.GetContext(), "linkBlock", func(objs Objects) {
		objs["linkBlock"] = blk
	})
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
	lis.Publish(xginx.GetContext(), "unlinkBlock", func(objs Objects) {
		objs["unlinkBlock"] = blk
	})
}

//MinerAddr 返回矿工地址
func (lis *shoplistener) MinerAddr() xginx.Address {
	bb, err := lis.keydb.GetConfig(MinerAddressKey)
	if err == nil {
		return xginx.Address(bb)
	}
	addr, err := lis.keydb.NewAccountInfo(xginx.CoinAccountType, "miner coin account")
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
	objblockkey = "objblockkey"
	objdocdbkey = "objdocdbkey"
	objkeydbkey = "objkeydbkey"
)

type Objects map[string]interface{}

func (objs Objects) BlockIndex() *xginx.BlockIndex {
	v, ok := objs[objblockkey].(*xginx.BlockIndex)
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
func NewObjects() Objects {
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

//创建全局变量
func (lis *shoplistener) NewObjects() Objects {
	objs := NewObjects()
	objs[objblockkey] = xginx.GetBlockIndex()
	objs[objdocdbkey] = lis.docdb
	objs[objkeydbkey] = lis.keydb
	return objs
}

//发布一个消息
func (lis *shoplistener) Publish(ctx context.Context, opt string, fns ...func(objs Objects)) {
	if lis.gqlsubmgr == nil {
		panic(fmt.Errorf("sub mgr miss"))
	}
	subs := lis.gqlsubmgr.Subscriptions()
	for conn := range subs {
		for _, sub := range subs[conn] {
			if sub.OperationName != opt {
				continue
			}
			//设定发布root对象数据
			objs := lis.NewObjects()
			for _, fn := range fns {
				fn(objs)
			}
			params := graphql.Params{
				Schema:         *lis.gqlschema,
				RequestString:  sub.Query,
				VariableValues: sub.Variables,
				OperationName:  sub.OperationName,
				RootObject:     objs,
				Context:        ctx,
			}
			result := graphql.Do(params)
			//发送数据
			data := &graphqlws.DataMessagePayload{
				Data:   result.Data,
				Errors: graphqlws.ErrorsFromGraphQLErrors(result.Errors),
			}
			sub.SendData(data)
		}
	}
}

func (lis *shoplistener) startgraphql(host string) {
	urlv, err := url.Parse(host)
	if err != nil {
		xginx.LogError(err)
		return
	}
	lis.gqlschema = GetSchema()
	lis.gqlsubmgr = graphqlws.NewSubscriptionManager(lis.gqlschema)
	conf := &handler.Config{
		Schema:     lis.gqlschema,
		Pretty:     true,
		GraphiQL:   false,
		Playground: true,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			return lis.NewObjects()
		},
	}
	lis.gqlhandler = handler.New(conf)
	mux := http.NewServeMux()
	mux.Handle("/subscriptions", graphqlws.NewHandler(graphqlws.HandlerConfig{
		SubscriptionManager: lis.gqlsubmgr,
	}))
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
