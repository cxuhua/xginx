package main

import (
	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var createPrivateKey = &graphql.Field{
	Name: "createPrivateKey",
	Type: graphql.String,
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		keydb := objs.KeyDB()
		id, err := keydb.NewPrivateKey()
		if err != nil {
			return NewError(1, err)
		}
		return id, nil
	},
	Description: "创建一个私钥,返回私钥id",
}

var listPrivateKey = &graphql.Field{
	Name:        "listPrivateKey",
	Type:        graphql.NewList(graphql.NewNonNull(graphql.String)),
	Description: "获取私钥列表",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		keydb := objs.KeyDB()
		ids, _ := keydb.ListPrivate(0)
		return ids, nil
	},
}
var AccountInfoType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AccountInfo",
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.AccountInfo)
		return ok
	},
	Description: "账户信息",
	Fields: graphql.Fields{
		"num": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ka := p.Source.(*xginx.AccountInfo)
				return int(ka.Num), nil
			},
			Description: "证书数量",
		},
		"less": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ka := p.Source.(*xginx.AccountInfo)
				return int(ka.Less), nil
			},
			Description: "需要签名的数量",
		},
		"arb": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ka := p.Source.(*xginx.AccountInfo)
				return ka.Arb, nil
			},
			Description: "是否启用仲裁",
		},
		"pks": {
			Type: graphql.NewList(graphql.String),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ka := p.Source.(*xginx.AccountInfo)
				return ka.Pks, nil
			},
			Description: "包含的密钥id",
		},
		"addr": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ka := p.Source.(*xginx.AccountInfo)
				id, err := ka.ID()
				return string(id), err
			},
			Description: "账户地址信息",
		},
		"desc": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ka := p.Source.(*xginx.AccountInfo)
				return ka.Desc, nil
			},
			Description: "描述信息",
		},
		"type": {
			Type: AccountType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				ka := p.Source.(*xginx.AccountInfo)
				return ka.Type, nil
			},
			Description: "账户地址信息",
		},
	},
})

var listAccount = &graphql.Field{
	Name:        "listAccount",
	Type:        graphql.NewList(graphql.NewNonNull(AccountInfoType)),
	Description: "获取私钥列表",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		keydb := objs.KeyDB()
		ids, _ := keydb.ListAddress(0)
		kas := []*xginx.AccountInfo{}
		for _, id := range ids {
			info, err := keydb.LoadAccountInfo(id)
			if err != nil {
				return NewError(1, "load address %s error %w", id, err)
			}
			kas = append(kas, info)
		}
		return kas, nil
	},
}

//账户类型
var AccountType = graphql.NewEnum(graphql.EnumConfig{
	Name: "AccountType",
	Values: graphql.EnumValueConfigMap{
		"COIN": {
			Value:       1,
			Description: "金额账户",
		},
		"TEMP": {
			Value:       2,
			Description: "临时账户",
		},
	},
})

var CreateAccountInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "CreateAccountInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"num": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "密钥数量",
		},
		"less": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "需要通过签名的密钥数量",
		},
		"arb": {
			Type:         graphql.Boolean,
			DefaultValue: true,
			Description:  "启用仲裁",
		},
		"pks": {
			Type:        graphql.NewList(graphql.NewNonNull(graphql.String)),
			Description: "私钥id列表",
		},
		"desc": {
			Type:         graphql.String,
			DefaultValue: "",
			Description:  "账户描述信息",
		},
		"type": {
			Type:         graphql.NewNonNull(AccountType),
			DefaultValue: 1,
			Description:  "账户类型",
		},
	},
	Description: "创建一个账号地址输入参数",
})

var createAccount = &graphql.Field{
	Name: "createAccount",
	Args: graphql.FieldConfigArgument{
		"info": {
			Type:        graphql.NewNonNull(CreateAccountInput),
			Description: "账户信息描述",
		},
	},
	Type: graphql.String,
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		ka := &xginx.AccountInfo{}
		err := DecodeValidateArgs("info", p, ka)
		if err != nil {
			return NewError(1, err)
		}
		if err := ka.Check(); err != nil {
			return NewError(2, err)
		}
		objs := GetObjects(p)
		keydb := objs.KeyDB()
		addr, err := keydb.SaveAccountInfo(ka)
		if err != nil {
			return NewError(3, err)
		}
		return string(addr), nil
	},
	Description: "创建一个账户,返回账户地址",
}
