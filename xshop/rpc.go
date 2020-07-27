package main

import (
	"context"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"

	pool "github.com/jolestar/go-commons-pool/v2"
)

type ApiType map[string]interface{}

type ShopApi struct {
	lis *shoplistener
}

func (api *ShopApi) Invoke(req ApiType, res *ApiType) error {
	(*res)["xx"] = 111
	return nil
}

//链接池子
func NewApiClientPool(ctx context.Context, addr string) *pool.ObjectPool {
	factory := pool.NewPooledObjectFactory(
		func(context.Context) (interface{}, error) {
			dialer := net.Dialer{}
			cctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()
			conn, err := dialer.DialContext(cctx, "tcp", addr)
			if err != nil {
				log.Println("connect json rpc error", addr)
				return nil, err
			}
			return jsonrpc.NewClient(conn), nil
		},
		func(ctx context.Context, object *pool.PooledObject) error {
			c := object.Object.(*rpc.Client)
			_ = c.Close()
			log.Println("close json rpc client")
			return nil
		}, nil, nil, nil)
	return pool.NewObjectPoolWithDefaultConfig(ctx, factory)
}

func CallApi(pool *pool.ObjectPool, method string, req ApiType) (*ApiType, error) {
	//获取链接从连接池
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	obj, err := pool.BorrowObject(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.ReturnObject(ctx, obj)
	//调用通用方法
	c := (obj).(*rpc.Client)
	ret := &ApiType{}
	err = c.Call("ShopApi."+method, req, ret)
	return ret, err
}
