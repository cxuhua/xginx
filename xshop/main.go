package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/graphql-go/handler"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var (
	//rpc设置
	gqlset = flag.String("graphql", "graphql://127.0.0.1:9334", "graphql addr port")
)

//实现自己的监听器
type shoplistener struct {
	//文档处理db
	docdb xginx.IDocSystem
	//密钥db
	keydb xginx.IKeysDB
	//graphql http
	gqlsrv *http.Server

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

type Objects map[string]interface{}

//返回可用的素有对象
func NewObjects(ctx context.Context) Objects {
	return Objects{}
}

func (lis *shoplistener) startgraphql(host string) {
	urlv, err := url.Parse(host)
	if err != nil {
		xginx.LogError(err)
		return
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"version": &graphql.Field{
					Type:        graphql.String,
					Description: "xginx version",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return xginx.Version, nil
					},
				},
			},
		}),
	})
	if err != nil {
		xginx.LogError(err)
		return
	}
	h := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   *xginx.IsDebug,
		GraphiQL: *xginx.IsDebug,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			return NewObjects(ctx)
		},
	})
	mux := http.NewServeMux()
	mux.Handle("/"+urlv.Scheme, h)
	lis.gqlsrv = &http.Server{
		Addr:    urlv.Host,
		Handler: mux,
		BaseContext: func(listener net.Listener) context.Context {
			ctx, _ := context.WithTimeout(xginx.GetContext(), time.Second*30)
			return ctx
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
