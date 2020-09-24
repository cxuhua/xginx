package main

import (
	"fmt"

	"github.com/mitchellh/mapstructure"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

//decode args到结构体
func DecodeArgs(p graphql.ResolveParams, obj interface{}, field ...string) error {
	var sv interface{} = p.Args
	if len(field) > 0 && field[0] != "" {
		sv = p.Args[field[0]]
	}
	return mapstructure.Decode(sv, obj)
}

//错误类型
type Error struct {
	Code int                    `json:"code"`          //错误代码
	Msg  string                 `json:"msg,omitempty"` //错误信息
	Ext  map[string]interface{} `json:"ext,omitempty"` //扩展错误信息
}

func (err Error) Extensions() map[string]interface{} {
	ext := map[string]interface{}{}
	if err.Ext != nil {
		ext = err.Ext
	}
	ext["code"] = err.Code
	return ext
}

func (err Error) Error() string {
	return err.Msg
}

func NewError(code int, sfmt interface{}, v ...interface{}) (interface{}, error) {
	switch sfmt.(type) {
	case error:
		msg := sfmt.(error).Error()
		return nil, Error{Code: code, Msg: msg}
	case string:
		msg := fmt.Sprintf(sfmt.(string), v...)
		return nil, Error{Code: code, Msg: msg}
	default:
		return nil, Error{Code: code}
	}
}

func hashtypesp(value interface{}) interface{} {
	switch value.(type) {
	case string:
		str := value.(string)
		//20*2
		if len(str) == len(xginx.ZERO160)*2 {
			return xginx.NewHASH160(str)
		}
		//32*2
		if len(str) == len(xginx.ZERO256)*2 {
			return xginx.NewHASH256(str)
		}
		//12*2
		if len(str) == len(xginx.NilDocumentID)*2 {
			return xginx.DocumentIDFromHex(str)
		}
		//16*2
		if len(str) == len(xginx.MsgID{})*2 {
			return xginx.MsgIDFromHex(str)
		}
	case *string:
		return hashtypesp(*(value).(*string))
	case xginx.HASH160:
		return value.(xginx.HASH160).String()
	case xginx.HASH256:
		return value.(xginx.HASH256).String()
	case xginx.DocumentID:
		return value.(xginx.DocumentID).String()
	case xginx.MsgID:
		return value.(xginx.MsgID).String()
	}
	return nil
}

var HashType = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "Hash",
	Description: "HASH256 HASH160 DocumentID MsgID",
	Serialize:   hashtypesp,
	ParseValue:  hashtypesp,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			return hashtypesp(valueAST.Value)
		}
		return nil
	},
})

var query = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"statusInfo":      statusInfo,
		"listCoin":        listCoin,
		"blockInfo":       blockInfo,
		"loadTxInfo":      loadTxInfo,
		"listPrivateKey":  listPrivateKey,
		"listAccount":     listAccount,
		"listTxPool":      listTxPool,
		"listRSA":         listRSA,
		"listTempProduct": listTempProduct,
		"loadProduct":     loadProduct,
		"findProduct":     findProduct,
		"loadEncodedTx":   loadEncodedTx,
	},
	Description: "数据查询接口",
})

var matation = graphql.NewObject(graphql.ObjectConfig{
	Name: "Mutation",
	Fields: graphql.Fields{
		"createRSA":        createRSA,
		"transfer":         transfer,
		"createPrivateKey": createPrivateKey,
		"createAccount":    createAccount,
		"newBlock":         newBlock,
		"createTxMeta":     createTxMeta,
		"loadTempProduct":  loadTempProduct,
		"newTempProduct":   newTempProduct,
		"uploadProduct":    uploadProduct,
		"purchaseProduct":  purchaseProduct,
	},
	Description: "数据更新接口",
})

var subscription = graphql.NewObject(graphql.ObjectConfig{
	Name: "Subscription",
	Fields: graphql.Fields{
		"block": {
			Name: "Block",
			Type: graphql.NewNonNull(BlockType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				blk, ok := objs["block"].(*xginx.BlockInfo)
				if !ok {
					return NewError(100, "block info miss")
				}
				return blk, nil
			},
			Description: "发送指定的区块信息",
		},
		"tx": {
			Name: "TX",
			Type: graphql.NewNonNull(TXType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				tx, ok := objs["tx"].(*xginx.TX)
				if !ok {
					return NewError(100, "tx info miss")
				}
				return tx, nil
			},
			Description: "发送指定的交易信息",
		},
		"product": {
			Name: "Product",
			Type: graphql.NewNonNull(MetaBodyType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				mb, ok := objs["product"].(*MetaBody)
				if !ok {
					return NewError(100, "metabody info miss")
				}
				return mb, nil
			},
			Description: "发送新的产品事件",
		},
	},
	Description: "数据订阅接口",
})

func GetSchema() *graphql.Schema {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:        query,
		Mutation:     matation,
		Subscription: subscription,
	})
	if err != nil {
		panic(err)
	}
	return &schema
}
