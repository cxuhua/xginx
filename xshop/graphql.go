package main

import (
	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var status = &graphql.Field{
	Type: graphql.NewObject(graphql.ObjectConfig{
		Name: "Status",
		Fields: graphql.Fields{
			"version": &graphql.Field{
				Type:        graphql.Int,
				Description: "区块链高度",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					conf := xginx.GetConfig()
					return conf.Ver, nil
				},
			},
			"height": &graphql.Field{
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

var query = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"status": status,
	},
})

var mutation = graphql.NewObject(graphql.ObjectConfig{
	Name: "Mutation",
	Fields: graphql.Fields{
		"login": &graphql.Field{
			Args: graphql.FieldConfigArgument{
				"name": {
					Type:         graphql.String,
					DefaultValue: "",
					Description:  "用户名称",
				},
				"pass": {
					Type:         graphql.String,
					DefaultValue: "",
					Description:  "用户密码",
				},
			},
			Type:        graphql.Int,
			Description: "xginx version",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return 100, nil
			},
		},
	},
})

func GetSchema() *graphql.Schema {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    query,
		Mutation: mutation,
	})
	if err != nil {
		panic(err)
	}
	return &schema
}
