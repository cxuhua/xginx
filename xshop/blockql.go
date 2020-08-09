package main

import (
	"fmt"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var BlockType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Block",
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.BlockInfo)
		return ok
	},
	Description: "区块类型",
	Fields: graphql.Fields{
		"id": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return blk.ID()
			},
			Description: "区块id",
		},
		"ver": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return blk.Header.Ver, nil
			},
			Description: "版本",
		},
		"prev": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return blk.Header.Prev, nil
			},
			Description: "上个区块的id",
		},
		"merkle": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return blk.Header.Merkle, nil
			},
			Description: "默克尔树id",
		},
		"time": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return blk.Header.Time, nil
			},
			Description: "生成时间戳",
		},
		"bits": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return fmt.Sprintf("%08x", blk.Header.Bits), nil
			},
			Description: "难度",
		},
		"nonce": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return fmt.Sprintf("%08x", blk.Header.Nonce), nil
			},
			Description: "随机数",
		},
		"txs": {
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(TXType))),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				blk := p.Source.(*xginx.BlockInfo)
				return blk.Txs, nil
			},
			Description: "交易列表",
		},
	},
})

func GetObjects(p graphql.ResolveParams) Objects {
	return p.Info.RootValue.(map[string]interface{})
}

var blockInfo = &graphql.Field{
	Name: "BlockInfo",
	Args: graphql.FieldConfigArgument{
		"id": {
			Type:         HashType,
			DefaultValue: nil,
			Description:  "根据区块id查询",
		},
		"height": {
			Type:         graphql.Int,
			DefaultValue: nil,
			Description:  "根据区块id查询",
		},
	},
	Type:        BlockType,
	Description: "查询区块信息",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		bi := objs.BlockIndex()
		id, ok := p.Args["id"].(xginx.HASH256)
		if ok {
			return bi.LoadBlock(id)
		}
		if h, has := p.Args["height"]; has {
			return bi.LoadBlockWithH(h.(int))
		}
		//无参数返回最后一个区块
		bv := bi.GetBestValue()
		if !bv.IsValid() {
			return nil, fmt.Errorf("block chain empty")
		}
		return bi.LoadBlock(bv.ID)
	},
}
