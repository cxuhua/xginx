package main

import (
	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var createRSA = &graphql.Field{
	Name: "CreateRSA",
	Type: graphql.String,
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		keydb := objs.KeyDB()
		id, err := keydb.NewRSA()
		if err != nil {
			return NewError(1, err)
		}
		return id, nil
	},
	Description: "创建一个RSA密钥",
}

var createPrivateKey = &graphql.Field{
	Name: "CreatePrivateKey",
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
	Name:        "ListPrivateKey",
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

var listRSA = &graphql.Field{
	Name:        "LlistRSA",
	Type:        graphql.NewList(graphql.NewNonNull(graphql.String)),
	Description: "获取RSA私钥ID列表",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		keydb := objs.KeyDB()
		return keydb.ListRSA(), nil
	},
}

var listAccount = &graphql.Field{
	Name:        "ListAccount",
	Type:        graphql.NewList(graphql.NewNonNull(AccountInfoType)),
	Description: "获取私钥列表",
	Args: graphql.FieldConfigArgument{
		"type": {
			Type:         AccountType,
			DefaultValue: 0,
			Description:  "账户地址",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		keydb := objs.KeyDB()
		ids, _ := keydb.ListAddress(0)
		typ := p.Args["type"].(int)
		kas := []*xginx.AccountInfo{}
		for _, id := range ids {
			info, err := keydb.LoadAccountInfo(id)
			if err != nil {
				return NewError(1, "load address %s error %w", id, err)
			}
			if typ > 0 && typ != info.Type {
				continue
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
			Value:       xginx.CoinAccountType,
			Description: "金额账户",
		},
		"TEMP": {
			Value:       xginx.TempAccountType,
			Description: "临时账户",
		},
	},
})

var CreateAccountInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "CreateAccountInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"num": {
			Type:         graphql.NewNonNull(graphql.Int),
			DefaultValue: 1,
			Description:  "密钥数量",
		},
		"less": {
			Type:         graphql.NewNonNull(graphql.Int),
			DefaultValue: 1,
			Description:  "需要通过签名的密钥数量",
		},
		"arb": {
			Type:         graphql.Boolean,
			DefaultValue: true,
			Description:  "启用仲裁",
		},
		"pks": {
			Type:        graphql.NewList(graphql.NewNonNull(graphql.String)),
			Description: "私钥id列表,签名时根据参数确定需要哪些私钥签名",
		},
		"desc": {
			Type:         graphql.String,
			DefaultValue: "",
			Description:  "账户描述信息",
		},
		"type": {
			Type:         graphql.NewNonNull(AccountType),
			DefaultValue: xginx.CoinAccountType,
			Description:  "账户类型",
		},
	},
	Description: "创建一个账号地址输入参数",
})

var createAccount = &graphql.Field{
	Name: "createAccount",
	Args: graphql.FieldConfigArgument{
		"args": {
			Type:        graphql.NewNonNull(CreateAccountInput),
			Description: "账户信息描述",
		},
	},
	Type: graphql.String,
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		ka := &xginx.AccountInfo{}
		err := DecodeArgs(p, ka, "args")
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
