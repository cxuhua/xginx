package main

import (
	"encoding/hex"
	"net"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

//交易签名处理接口
type ShopTransListener interface {
	xginx.ITransListener
	xginx.ISignTx
}

//转账接口实现
type eshoptranslistener struct {
	bi    *xginx.BlockIndex //链指针
	keydb xginx.IKeysDB     //账户数据库指针
}

func (lis *eshoptranslistener) NewWitnessScript(ckv *xginx.CoinKeyValue) (*xginx.WitnessScript, error) {
	addr := ckv.GetAddress()
	return lis.keydb.NewWitnessScript(addr, xginx.DefaultInputScript)
}

//创建一个转账处理器,使用默认的输入输出脚本
func (obj Objects) NewTransListener() ShopTransListener {
	bi := obj.BlockIndex()
	keydb := obj.KeyDB()
	return &eshoptranslistener{bi: bi, keydb: keydb}
}

//签名交易
func (lis *eshoptranslistener) SignTx(singer xginx.ISigner, pass ...string) error {
	//获取签名信息
	_, in, out, _ := singer.GetObjs()
	//获取签名hash
	hash, err := singer.GetSigHash()
	if err != nil {
		return err
	}
	////从输入获取签名脚本
	wits, err := in.Script.ToWitness()
	if err != nil {
		return err
	}
	//获取输入引用的输出地址
	addr, err := out.Script.GetAddress()
	if err != nil {
		return err
	}
	//账户信息
	info, err := lis.keydb.LoadAccountInfo(addr)
	if err != nil {
		return err
	}
	//转换未为签名结构
	acc, err := info.ToAccount(lis.keydb)
	if err != nil {
		return err
	}
	sigs, err := acc.SignAll(hash)
	if err != nil {
		return err
	}
	wits.Pks = acc.GetPks()
	wits.Sig = sigs
	script, err := wits.Final()
	if err != nil {
		return err
	}
	in.Script = script
	return nil
}

//获取使用的金额列表
func (lis *eshoptranslistener) GetCoins(amt xginx.Amount) xginx.Coins {
	//获取所有的金额账户
	ret := xginx.Coins{}
	//每次获取10个
	for addrs, lkey := lis.keydb.ListAddress(10); len(addrs) > 0; {
		for _, addr := range addrs {
			acc, err := lis.keydb.LoadAccountInfo(addr)
			if err != nil {
				panic(err)
			}
			if acc.Type != xginx.CoinAccountType {
				continue
			}
			coins, err := lis.bi.ListCoins(addr)
			if err != nil {
				panic(err)
			}
			ret = append(ret, coins.Coins...)
			if ret.Balance() >= amt {
				return ret
			}
		}
		//继续获取后面的地址
		addrs, lkey = lis.keydb.ListAddress(10, lkey)
	}
	return ret
}

//获取找零地址
func (lis *eshoptranslistener) GetKeep() xginx.Address {
	return xginx.EmptyAddress
}

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
		"exec": {
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.TxScript)
				return string(script.Exec), nil
			},
			Description: "执行脚本内容",
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
		"exec": {
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
			Type: MetaBodyType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				script := p.Source.(*xginx.LockedScript)
				return ParseMetaBody(script.Meta)
			},
			Description: "相关数据",
		},
		"exec": {
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
	Description: "锁定脚本",
})

//输入脚本类型可能是coins脚本或者是签名脚本
var TxInScriptType = graphql.NewUnion(graphql.UnionConfig{
	Name:  "TxInScript",
	Types: []*graphql.Object{CoinbaseScriptType, WitnessScriptType},
	ResolveType: func(p graphql.ResolveTypeParams) *graphql.Object {
		_, ok := p.Value.(xginx.Script)
		if ok {
			return CoinbaseScriptType
		}
		_, ok = p.Value.(*xginx.WitnessScript)
		if ok {
			return WitnessScriptType
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
			Type: TxInScriptType,
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
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				out := p.Source.(*xginx.TxOut)
				return int(out.Value), nil
			},
			Description: "对应的输出金额",
		},
		"script": {
			Type: LockedScriptType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				out := p.Source.(*xginx.TxOut)
				return out.Script.To()
			},
			Description: "输出脚本",
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
		"verify": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				bi := objs.BlockIndex()
				tx := p.Source.(*xginx.TX)
				return tx.Verify(bi) == nil, nil
			},
			Description: "签名校验",
		},
		"final": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return tx.IsFinal(), nil
			},
			Description: "是否是最终交易,不可替换",
		},
		"confirm": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				objs := GetObjects(p)
				bi := objs.BlockIndex()
				num := bi.GetTxConfirm(tx.MustID())
				return num, nil
			},
			Description: "确认数,确认数到达6交易安全",
		},
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
			Type: TxScriptType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return tx.Script.To()
			},
			Description: "交易脚本",
		},
		"coinbase": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				in := p.Source.(*xginx.TX)
				return in.IsCoinBase(), nil
			},
			Description: "是否是coinbase交易",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*xginx.TX)
		return ok
	},
	Description: "交易类型",
})

var txInfo = &graphql.Field{
	Name: "TxInfo",
	Args: graphql.FieldConfigArgument{
		"id": {
			Type:        graphql.NewNonNull(HashType),
			Description: "交易id",
		},
	},
	Type:        TXType,
	Description: "查询交易信息",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		bi := objs.BlockIndex()
		id, ok := p.Args["id"].(xginx.HASH256)
		if !ok {
			return NewError(1, "id args error")
		}
		return bi.LoadTX(id)
	},
}

type TransferInfo struct {
	Addr   xginx.Address `json:"addr"`
	Amount xginx.Amount  `json:"amount"`
	Meta   string        `json:"meta"`
	Fee    xginx.Amount  `json:"fee"`
}

var TransferInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "TransferInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"addr": {
			Type:        graphql.NewNonNull(graphql.String),
			Description: "转账地址",
		},
		"amount": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "转账金额",
		},
		"meta": {
			Type:         graphql.String,
			DefaultValue: "",
			Description:  "输出meta",
		},
		"fee": {
			Type:         graphql.Int,
			DefaultValue: 0,
			Description:  "交易费",
		},
	},
	Description: "转账到指定地址输入参数",
})

var transfer = &graphql.Field{
	Name: "Transfer",
	Type: graphql.NewNonNull(TXType),
	Args: graphql.FieldConfigArgument{
		"attr": {
			Type:         TransferInput,
			DefaultValue: nil,
			Description:  "输入参数",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		info := &TransferInfo{}
		err := DecodeValidateArgs("attr", p, info)
		if err != nil {
			return NewError(1, err)
		}
		objs := GetObjects(p)
		bi := objs.BlockIndex()
		lis := objs.NewTransListener()
		mi := bi.NewTrans(lis)
		mi.Add(info.Addr, info.Amount)
		mi.Fee = info.Fee
		tx, err := mi.NewTx(xginx.DefaultExeTime, xginx.DefaultTxScript)
		if err != nil {
			return NewError(2, err)
		}
		err = tx.Sign(bi, lis)
		if err != nil {
			return NewError(3, err)
		}
		bp := bi.GetTxPool()
		err = bp.PushTx(bi, tx)
		if err != nil {
			return NewError(4, err)
		}
		return tx, nil
	},
	Description: "转账指定的金额到其他账号,返回交易信息",
}
