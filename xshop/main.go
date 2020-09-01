package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	//分类db
	typdb ITypeDB
	//graphql http
	gqlhttp *http.Server
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

//是否保存文档
func (lis *shoplistener) isSaveMetaBody(ctx context.Context, meta *MetaBody) bool {
	return true
}

//当收到一个销售数据
func (lis *shoplistener) onSellMeta(ctx context.Context, tx *xginx.TX, idx int, meta *MetaBody, link bool) error {
	id := meta.MustID()
	if !link {
		return lis.docdb.Delete(id)
	}
	has, err := lis.docdb.Has(id)
	if err != nil {
		return err
	}
	//不存在添加到文档数据库,不关心的文档数据可不存入
	if !has && lis.isSaveMetaBody(ctx, meta) {
		doc, err := meta.ToDocument()
		if err != nil {
			return err
		}
		err = lis.docdb.Insert(doc)
		if err != nil {
			return err
		}
	}
	//添加文档和区块交易数据的关联关系 docid -> tx.id+idx
	//即使文档被删除了,这个关系还是存在
	ext := &MetaExt{
		TxID:  tx.MustID(),
		Index: xginx.VarUInt(idx),
	}
	err = lis.docdb.PutObject(id, ext)
	if err != nil {
		return err
	}
	lis.Publish(ctx, "newProduct", func(objs Objects) {
		objs["product"] = meta
	})
	return nil
}

//当收到一个购买数据
func (lis *shoplistener) onPurchaseMeta(ctx context.Context, tx *xginx.TX, idx int, meta *MetaBody, link bool) error {
	return nil
}

//tx[idx] = meta位置
func (lis *shoplistener) onNewScriptMeta(ctx context.Context, tx *xginx.TX, idx int, meta xginx.VarBytes, link bool) error {
	bb, err := MetaCoder.Decode(meta)
	if err != nil {
		return err
	}
	mb, err := ParseMetaBody(bb)
	if err != nil {
		return err
	}
	//如果文档ID存在
	if !mb.HasID() {
		return fmt.Errorf("meta data error, id miss")
	}
	if mb.Type == MetaTypeSell {
		return lis.onSellMeta(ctx, tx, idx, mb, link)
	}
	if mb.Type == MetaTypePurchase {
		return lis.onPurchaseMeta(ctx, tx, idx, mb, link)
	}
	return nil
}

func (lis *shoplistener) OnLinkBlock(blk *xginx.BlockInfo) {
	ctx := xginx.GetContext()
	//处理是否有新产品在区块中
	blk.EachOutScript(func(tx *xginx.TX, idx int, script *xginx.LockedScript) {
		if script.Meta.Len() == 0 {
			return
		}
		err := lis.onNewScriptMeta(ctx, tx, idx, script.Meta, true)
		if err != nil {
			xginx.LogError(err)
		}
	})
	//发送订阅信息
	lis.Publish(ctx, "linkBlock", func(objs Objects) {
		objs["block"] = blk
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
	typ := msg.Type()
	switch typ {
	case xginx.NtBroadInfo:
		m := msg.(*xginx.MsgBroadInfo)
		if m.Action == MsgActionPurchaseTx {
			r := xginx.NewReader(m.Msg)
			tx := &xginx.TX{}
			err := tx.Decode(r)
			if err != nil {
				xginx.LogWarn("recv MsgBroadInfo action=MsgActionPurchaseTx Decode tx error", err)
				return
			}
			mbs := SeparateMetaBodyFromTx(tx)
			if len(mbs) == 0 {
				xginx.LogWarn("recv MsgBroadInfo action=MsgActionPurchaseTx err,miss metabody data")
				return
			}
			//暂时先不检测metabody数据的合法性
			//继续广播数据
			if xginx.Server != nil {
				xginx.Server.BroadMsg(m, c)
			}
		} else if m.Action == MsgActionSellMeta {

		}
		log.Println(m.Action)
	}
}

//TimeNow 当前时间戳获取
func (lis *shoplistener) TimeNow() uint32 {
	return uint32(time.Now().Unix())
}

//OnUnlinkBlock 区块断开
func (lis *shoplistener) OnUnlinkBlock(blk *xginx.BlockInfo) {
	ctx := xginx.GetContext()
	//处理是否有新产品移除
	blk.EachOutScript(func(tx *xginx.TX, idx int, script *xginx.LockedScript) {
		if script.Meta.Len() == 0 {
			return
		}
		_ = lis.onNewScriptMeta(ctx, tx, idx, script.Meta, false)
	})
	lis.Publish(xginx.GetContext(), "unlinkBlock", func(objs Objects) {
		objs["block"] = blk
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
	typdb, err := NewTypeDB(conf.DataDir + "/types")
	if err != nil {
		return err
	}
	lis.typdb = typdb
	return nil
}

var (
	lis = &shoplistener{}
)

const (
	objblockkey = "__objblockkey__"
	objdocdbkey = "__objdocdbkey__"
	objkeydbkey = "__objkeydbkey__"
	objtypdbkey = "__objtypdbkey__"
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

func (objs Objects) TypeDB() ITypeDB {
	v, ok := objs[objtypdbkey].(ITypeDB)
	if !ok {
		panic(fmt.Errorf("type db  miss"))
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
	objs[objtypdbkey] = lis.typdb
	return objs
}

//发布一个消息
func (lis *shoplistener) Publish(ctx context.Context, opt string, setobjs ...func(objs Objects)) {
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
			for _, setobj := range setobjs {
				setobj(objs)
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
	//订阅初始化
	lis.gqlsubmgr = graphqlws.NewSubscriptionManager(lis.gqlschema)
	conf := &handler.Config{
		Schema:     lis.gqlschema,
		Pretty:     true,
		GraphiQL:   true,
		Playground: false,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			return lis.NewObjects()
		},
	}
	lis.gqlhandler = handler.New(conf)
	mux := http.NewServeMux()
	mux.Handle("/subscriptions", graphqlws.NewHandler(graphqlws.HandlerConfig{
		SubscriptionManager: lis.gqlsubmgr,
	}))
	//查询更新初始化
	mux.Handle("/"+urlv.Scheme, lis)
	lis.gqlhttp = &http.Server{
		Addr:    urlv.Host,
		Handler: mux,
		BaseContext: func(listener net.Listener) context.Context {
			return xginx.GetContext()
		},
	}
	lis.wg.Add(1)
	go func() {
		defer lis.wg.Done()
		err := lis.gqlhttp.ListenAndServe()
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
	if lis.gqlhttp != nil {
		ctx, cancel := context.WithTimeout(xginx.GetContext(), time.Second*30)
		defer cancel()
		_ = lis.gqlhttp.Shutdown(ctx)
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
	if lis.typdb != nil {
		lis.typdb.Close()
	}
}

func main() {
	flag.Parse()
	xginx.Run(lis)
}
