package main

import "github.com/graphql-go/graphql"

var MetaEleType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetaEle",
	Fields: graphql.Fields{
		"type": {
			Type: EleType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(MetaEle)
				return mb.Type, nil
			},
			Description: "类型",
		},
		"size": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(MetaEle)
				return mb.Size, nil
			},
			Description: "元素长度",
		},
		"sum": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(MetaEle)
				return mb.Sum, nil
			},
			Description: "校验和hash160",
		},
		"body": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(MetaEle)
				return mb.Body, nil
			},
			Description: "内容",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(MetaEle)
		return ok
	},
	Description: "meta元素描述",
})

var MetaBodyType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetaBody",
	Fields: graphql.Fields{
		"type": {
			Type: BodyType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.Type, nil
			},
			Description: "类型",
		},
		"tags": {
			Type: graphql.NewList(graphql.String),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.Tags, nil
			},
			Description: "标签",
		},
		"eles": {
			Type: graphql.NewList(MetaEleType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.Eles, nil
			},
			Description: "标签",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*MetaBody)
		return ok
	},
	Description: "meta内容描述",
})

var EleType = graphql.NewEnum(graphql.EnumConfig{
	Name: "EleType",
	Values: graphql.EnumValueConfigMap{
		"TEXT": {
			Value:       MetaEleTEXT,
			Description: "文本元素",
		},
		"URL": {
			Value:       MetaEleURL,
			Description: "链接元素 ",
		},
		"HASH": {
			Value:       MetaEleHASH,
			Description: "HASH公钥",
		},
		"RSA": {
			Value:       MetaEleRSA,
			Description: "RSA公钥,用于信息加密",
		},
	},
	Description: "meta元素类型",
})

var BodyType = graphql.NewEnum(graphql.EnumConfig{
	Name: "BodyType",
	Values: graphql.EnumValueConfigMap{
		"SELL": {
			Value:       MetaTypeSell,
			Description: "出售",
		},
		"BUY": {
			Value:       MetaTypeBuy,
			Description: "购买",
		},
		"CONFIRM": {
			Value:       MetaTypeConfirm,
			Description: "确认发货",
		},
	},
	Description: "meta类型",
})