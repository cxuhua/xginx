package main

import (
	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

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
	Description: "hash256 hash160 graph",
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
		"status": status,
		"coin":   coin,
		"block":  block,
	},
})

func GetSchema() *graphql.Schema {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: query,
	})
	if err != nil {
		panic(err)
	}
	return &schema
}
