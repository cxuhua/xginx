package main

import (
	"fmt"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var ClientType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Client",
	Fields: graphql.Fields{
		"id": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				cli := p.Source.(*xginx.Client)
				return cli.ID(), nil
			},
			Description: "节点id",
		},
		"addr": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				cli := p.Source.(*xginx.Client)
				return cli.Addr.String(), nil
			},
			Description: "链接地址",
		},
		"service": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				cli := p.Source.(*xginx.Client)
				return fmt.Sprintf("%x", cli.Service), nil
			},
			Description: "服务",
		},
		"height": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				cli := p.Source.(*xginx.Client)
				return cli.Height, nil
			},
			Description: "链接地址",
		},
		"ver": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				cli := p.Source.(*xginx.Client)
				return cli.Ver, nil
			},
			Description: "链接地址",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.Client)
		return ok
	},
	Description: "链接信息",
})

var statusInfo = &graphql.Field{
	Type: graphql.NewObject(graphql.ObjectConfig{
		Name: "StatusInfo",
		Fields: graphql.Fields{
			"version": {
				Type:        graphql.Int,
				Description: "运行版本",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					conf := xginx.GetConfig()
					return conf.Ver, nil
				},
			},
			"height": {
				Type:        graphql.Int,
				Description: "区块链高度",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					bi := p.Source.(*xginx.BlockIndex)
					return bi.Height(), nil
				},
			},
			"cache": {
				Type:        graphql.Int,
				Description: "缓存内存大小",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					bi := p.Source.(*xginx.BlockIndex)
					return bi.CacheSize(), nil
				},
			},
			"clients": {
				Type:        graphql.NewList(ClientType),
				Description: "链接客户端",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return xginx.Server.Clients(), nil
				},
			},
		},
	}),
	Description: "获取当前区块链状态",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := Objects(p.Info.RootValue.(map[string]interface{}))
		return objs.BlockIndex(), nil
	},
}
