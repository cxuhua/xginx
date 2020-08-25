package main

import (
	"fmt"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

//金额状态
var CoinState = graphql.NewEnum(graphql.EnumConfig{
	Name: "CoinState",
	Values: graphql.EnumValueConfigMap{
		"ALL": {
			Value:       0,
			Description: "所有状态",
		},
		"LOCKED": {
			Value:       1,
			Description: "当前区块高度下锁定的",
		},
		"AVAILABLE": {
			Value:       2,
			Description: "当前区块高度下可消费的",
		},
	},
})

var CoinType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Coin",
	Fields: graphql.Fields{
		"ispool": {
			Type:        graphql.Boolean,
			Description: "是否在交易池",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				return coin.IsPool(), nil
			},
		},
		"txId": {
			Type:        HashType,
			Description: "金额所在的交易ID",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				return coin.TxID, nil
			},
		},
		"txInfo": {
			Type:        TXType,
			Description: "交易信息",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				objs := GetObjects(p)
				bi := objs.BlockIndex()
				return bi.LoadTX(coin.TxID)
			},
		},
		"index": {
			Type:        graphql.Int,
			Description: "金额锁在的交易输出索引",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				return int(coin.Index), nil
			},
		},
		"amount": {
			Type:        graphql.Int,
			Description: "金额大小",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				return int64(coin.Value), nil
			},
		},
		"coinbase": {
			Type:        graphql.Boolean,
			Description: "是否来自coinbase交易",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				return coin.Base > 0, nil
			},
		},
		"height": {
			Type:        graphql.Int,
			Description: "所在区块的高度",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				return int(coin.Height), nil
			},
		},
		"state": {
			Type:        CoinState,
			Description: "状态",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				coin := p.Source.(*xginx.CoinKeyValue)
				objs := Objects(p.Info.RootValue.(map[string]interface{}))
				bi := objs.BlockIndex()
				spent := bi.NextHeight()
				if coin.IsMatured(spent) {
					return 2, nil
				} else {
					return 1, nil
				}
			},
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.CoinKeyValue)
		return ok
	},
	Description: "单笔金额信息",
})

var listCoin = &graphql.Field{
	Type: graphql.NewObject(graphql.ObjectConfig{
		Name: "ListCoin",
		Fields: graphql.Fields{
			"list": &graphql.Field{
				Args: graphql.FieldConfigArgument{
					"addr": {
						Type:         graphql.NewNonNull(graphql.String),
						DefaultValue: nil,
						Description:  "账户地址",
					},
					"state": {
						Type:         CoinState,
						DefaultValue: 0,
						Description:  "按状态查询",
					},
					"fee": {
						Type:         graphql.Int,
						DefaultValue: 0,
						Description:  "按金额返回可用的金额",
					},
				},
				Type:        graphql.NewList(CoinType),
				Description: "获取地址金额信息",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					args := struct {
						Addr  xginx.Address
						State int
						Fee   xginx.Amount
					}{}
					err := DecodeArgs(p, &args)
					if err != nil {
						return NewError(100, err)
					}
					bi := p.Source.(*xginx.BlockIndex)
					if args.Fee > 0 {
						coins := xginx.Coins{}
						spent := bi.NextHeight()
						err := bi.ListCoinsWithCB(args.Addr, func(ckv *xginx.CoinKeyValue) bool {
							//不成熟的忽略
							if !ckv.IsMatured(spent) {
								return true
							}
							coins = append(coins, ckv)
							args.Fee -= ckv.Value
							return args.Fee > 0
						})
						return coins, err
					}
					coins, err := bi.ListCoins(args.Addr)
					if err != nil {
						return nil, err
					}
					if args.State == 0 {
						return coins.All.Sort(), nil
					}
					if args.State == 1 {
						return coins.Locks.Sort(), nil
					}
					if args.State == 2 {
						return coins.Coins.Sort(), nil
					}
					return nil, fmt.Errorf("state args %d error", args.State)
				},
			},
		},
	}),
	Description: "查询金额相关信息",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := Objects(p.Info.RootValue.(map[string]interface{}))
		return objs.BlockIndex(), nil
	},
}
