package main

import (
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

var (
	//默认校验器
	validate = validator.New()
)

//DecodeValidateArgs decode args 到结构体并校验参数
func DecodeValidateArgs(f string, p graphql.ResolveParams, obj interface{}) error {
	err := mapstructure.Decode(p.Args[f], obj)
	if err != nil {
		return err
	}
	v := reflect.Indirect(reflect.ValueOf(obj))
	if v.Kind() == reflect.Struct {
		return validate.StructCtx(p.Context, obj)
	}
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		return fmt.Errorf("obj kind error %v", v.Kind())
	}
	for i := 0; i < v.Len(); i++ {
		iv := v.Index(i)
		err = validate.StructCtx(p.Context, iv.Interface())
		if err != nil {
			return fmt.Errorf("validate array %d error %w", i, err)
		}
	}
	return nil
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
		if len(str) == len(xginx.ZERO160)*2 {
			return xginx.NewHASH160(str)
		}
		if len(str) == len(xginx.ZERO256)*2 {
			return xginx.NewHASH256(str)
		}
	case *string:
		return hashtypesp(*(value).(*string))
	case xginx.HASH160:
		return value.(xginx.HASH160).String()
	case xginx.HASH256:
		return value.(xginx.HASH256).String()
	}
	return nil
}

var HashType = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "Hash",
	Description: "hash256 hash160",
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
		"statusInfo":     statusInfo,
		"listCoin":       listCoin,
		"blockInfo":      blockInfo,
		"txInfo":         txInfo,
		"listPrivateKey": listPrivateKey,
		"listAccount":    listAccount,
		"listTxPool":     listTxPool,
	},
	Description: "数据查询接口",
})

var matation = graphql.NewObject(graphql.ObjectConfig{
	Name: "Mutation",
	Fields: graphql.Fields{
		"transfer":         transfer,
		"createPrivateKey": createPrivateKey,
		"createAccount":    createAccount,
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
					return NewError(100, "linkBlock info miss")
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
			Description: "发送指定的交易信息信息",
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
