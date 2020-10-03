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
		"id": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.ID()
			},
			Description: "id",
		},
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
			Description: "元素",
		},
		"txId": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.TxID, nil
			},
			Description: "交易ID",
		},
		"index": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.Index.ToInt(), nil
			},
			Description: "输出索引",
		},
		"next": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.Next, nil
			},
			Description: "下个文档",
		},
		"prev": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb := p.Source.(*MetaBody)
				return mb.Prev, nil
			},
			Description: "上个文档",
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
		"UUID": {
			Value:       MetaEleUUID,
			Description: "文本元素",
		},
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
		"KID": {
			Value:       MetaEleKID,
			Description: "私钥ID,公钥hashid",
		},
	},
	Description: "meta元素类型",
})

var BodyType = graphql.NewEnum(graphql.EnumConfig{
	Name: "BodyType",
	Values: graphql.EnumValueConfigMap{
		"SELL": {
			Value:       MetaTypeSell,
			Description: "出售,卖家发出,并发布的链",
		},
		"PURCHASE": {
			Value:       MetaTypePurchase,
			Description: "购买,买家购买",
		},
		"FINISH": {
			Value:       MetaTypeFinish,
			Description: "完成交易",
		},
	},
	Description: "meta类型",
})
