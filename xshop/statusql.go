package main

import (
	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var status = &graphql.Field{
	Type: graphql.NewObject(graphql.ObjectConfig{
		Name: "Status",
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
		},
	}),
	Description: "获取当前区块链状态",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := Objects(p.Info.RootValue.(map[string]interface{}))
		return objs.BlockIndex(), nil
	},
}
