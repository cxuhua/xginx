package main

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

var CoinbaseScriptType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CoinbaseScript",
	Fields: graphql.Fields{
		"data": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(xginx.Script)
				return hex.EncodeToString(script.Data()), nil
			},
			Description: "自定义数据",
		},
		"ip": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(xginx.Script)
				ip := net.IP(script.IP())
				return ip.String(), nil
			},
			Description: "生成区块的节点ip",
		},
		"height": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(xginx.Script)
				return int(script.Height()), nil
			},
			Description: "生成区块的节点ip",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(xginx.Script)
		return ok
	},
	Description: "coinbase脚本类型",
})

var TxScriptType = graphql.NewObject(graphql.ObjectConfig{
	Name: "TxScript",
	Fields: graphql.Fields{
		"time": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.TxScript)
				return int(script.ExeTime), nil
			},
			Description: "脚本执行最大时间(ms)",
		},
		"body": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.TxScript)
				return string(script.Exec), nil
			},
			Description: "脚本内容",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.TxScript)
		return ok
	},
	Description: "交易脚本类型",
})

var WitnessScriptType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WitnessScript",
	Fields: graphql.Fields{
		"num": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.WitnessScript)
				return int(script.Num), nil
			},
			Description: "证书数量",
		},
		"less": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.WitnessScript)
				return int(script.Less), nil
			},
			Description: "通过签名的最小数量",
		},
		"arb": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.WitnessScript)
				return script.Arb != xginx.InvalidArb, nil
			},
			Description: "是否启用仲裁证书",
		},
		"pks": {
			Type: graphql.NewList(graphql.NewNonNull(graphql.String)),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.WitnessScript)
				pks := []string{}
				for _, v := range script.Pks {
					str := hex.EncodeToString(v[:])
					if str == "" {
						continue
					}
					pks = append(pks, str)
				}
				return pks, nil
			},
			Description: "多个公钥",
		},
		"sigs": {
			Type: graphql.NewList(graphql.NewNonNull(graphql.String)),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.WitnessScript)
				sigs := []string{}
				for _, v := range script.Sig {
					str := hex.EncodeToString(v[:])
					if str == "" {
						continue
					}
					sigs = append(sigs, str)
				}
				return sigs, nil
			},
			Description: "对应的签名",
		},
		"body": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.WitnessScript)
				return string(script.Exec), nil
			},
			Description: "脚本内容",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(xginx.TxScript)
		return ok
	},
	Description: "输入脚本类型,多重签名证书",
})

var LockedScriptType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LockedScript",
	Fields: graphql.Fields{
		"addr": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.LockedScript)
				return string(script.Address()), nil
			},
			Description: "输出地址",
		},
		"meta": {
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"fmt": {
					Type:         graphql.String,
					DefaultValue: "str",
					Description:  "显示格式:hex-16进制, str-字符串",
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.LockedScript)
				sfmt := p.Args["fmt"].(string)
				if sfmt == "str" {
					return string(script.Meta), nil
				}
				if sfmt == "hex" {
					return hex.EncodeToString(script.Meta), nil
				}
				return nil, fmt.Errorf("fmt error")
			},
			Description: "脚本执行最大时间(ms)",
		},
		"body": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.LockedScript)
				return string(script.Exec), nil
			},
			Description: "脚本内容",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.LockedScript)
		return ok
	},
	Description: "交易脚本类型",
})

var ScriptType = graphql.NewUnion(graphql.UnionConfig{
	Name:  "Script",
	Types: []*graphql.Object{CoinbaseScriptType, TxScriptType, WitnessScriptType, LockedScriptType},
	ResolveType: func(p graphql.ResolveTypeParams) *graphql.Object {
		_, ok := p.Value.(xginx.Script)
		if ok {
			return CoinbaseScriptType
		}
		_, ok = p.Value.(*xginx.TxScript)
		if ok {
			return TxScriptType
		}
		_, ok = p.Value.(*xginx.WitnessScript)
		if ok {
			return WitnessScriptType
		}
		_, ok = p.Value.(*xginx.LockedScript)
		if ok {
			return LockedScriptType
		}
		return nil
	},
	Description: "脚本类型",
})

var TxInType = graphql.NewObject(graphql.ObjectConfig{
	Name: "TxIn",
	Fields: graphql.Fields{
		"outHash": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				in := p.Source.(*xginx.TxIn)
				return in.OutHash, nil
			},
			Description: "对应的输出交易id",
		},
		"outIndex": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				in := p.Source.(*xginx.TxIn)
				return int(in.OutIndex), nil
			},
			Description: "对应的输出所在的索引",
		},
		"script": {
			Type: ScriptType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				in := p.Source.(*xginx.TxIn)
				return in.Script.To()
			},
			Description: "对应的输出所在的索引",
		},
		"sequence": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				in := p.Source.(*xginx.TxIn)
				return int(in.Sequence), nil
			},
			Description: "序列号",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.TxIn)
		return ok
	},
	Description: "交易输入",
})

var TxOutType = graphql.NewObject(graphql.ObjectConfig{
	Name: "TxOut",
	Fields: graphql.Fields{
		"value": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				out := p.Source.(*xginx.TxOut)
				return int(out.Value), nil
			},
			Description: "对应的输出交易id",
		},
		"script": {
			Type: ScriptType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				out := p.Source.(*xginx.TxOut)
				return out.Script.To()
			},
			Description: "对应的输出所在的索引",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.TxOut)
		return ok
	},
	Description: "交易输出",
})

var TXType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Tx",
	Fields: graphql.Fields{
		"ver": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return int(tx.Ver), nil
			},
			Description: "交易版本",
		},
		"id": {
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return tx.ID()
			},
			Description: "交易id",
		},
		"ins": {
			Type: graphql.NewList(graphql.NewNonNull(TxInType)),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return tx.Ins, nil
			},
			Description: "输入",
		},
		"outs": {
			Type: graphql.NewList(graphql.NewNonNull(TxOutType)),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return tx.Outs, nil
			},
			Description: "输出",
		},
		"script": {
			Type: ScriptType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return tx.Script.To()
			},
			Description: "交易脚本",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.TX)
		return ok
	},
	Description: "交易类型",
})
