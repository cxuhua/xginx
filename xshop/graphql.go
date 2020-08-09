package main

import (
	"fmt"

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
	return validate.StructCtx(p.Context, obj)
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
		l := len(str)
		if l == len(xginx.ZERO160)*2 {
			return xginx.NewHASH160(str)
		}
		if l == len(xginx.ZERO256)*2 {
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

func GetSchema() *graphql.Schema {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    query,
		Mutation: matation,
	})
	if err != nil {
		panic(err)
	}
	return &schema
}
