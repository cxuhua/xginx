package main

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"

	"github.com/cxuhua/xginx"
	"github.com/graphql-go/graphql"
)

//交易签名处理接口
type IShopTrans interface {
	xginx.ISignTx
	//fee交易费
	NewTx(fee xginx.Amount) (*xginx.TX, error)
}

//转账发送地址金额
type SenderInfo struct {
	Addr   xginx.Address //地址
	TxID   xginx.HASH256 //交易id
	Index  xginx.VarUInt //金额索引
	Script string        //解锁脚本
	Keep   bool          //是否是找零地址
}

//转账接收信息
type ReceiverInfo struct {
	Addr   xginx.Address //地址
	Amount xginx.Amount  //金额
	Meta   string        //meta数据
	Script string        //输出脚本
}

func (r ReceiverInfo) ParseMeta() (*MetaBody, error) {
	return ParseMetaBody([]byte(r.Meta))
}

var (
	MetaCoder = xginx.LzmaCoder
)

//输出+信息结构,保存信息和对应的输出,out的meta应该是meta信息
type MetaTxOut struct {
	Tx    *xginx.TX    //对应的区块交易信息
	Out   *xginx.TxOut //所在的输出
	Index int          //输出索引
	Meta  *MetaBody    //解析出的meta信息
}

var MetaTxOutType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetaTxOutType",
	Fields: graphql.Fields{
		"tx": {
			Type: graphql.NewNonNull(TXType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOut)
				return v.Tx, nil
			},
			Description: "交易信息",
		},
		"out": {
			Type: graphql.NewNonNull(TxOutType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOut)
				return v.Out, nil
			},
			Description: "信息meta信息所在的输出",
		},
		"index": {
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOut)
				return v.Index, nil
			},
			Description: "输出的索引位置",
		},
		"meta": {
			Type: graphql.NewNonNull(MetaBodyType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOut)
				return v.Meta, nil
			},
			Description: "meta信息",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*MetaTxOut)
		return ok
	},
	Description: "交易脚本类型",
})

type MetaTxOutPair struct {
	Purchase *MetaTxOut
	Sell     *MetaTxOut
}

var PairAmountType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PairAmountType",
	Fields: graphql.Fields{
		"sell": {
			Type: AmountType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOutPair)
				return v.GetProductAmount()
			},
			Description: "卖价",
		},
		"pay": {
			Type: AmountType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOutPair)
				return v.GetPayAmount()
			},
			Description: "支付的",
		},
		"deposit": {
			Type: AmountType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOutPair)
				return v.GetDepositAmount()
			},
			Description: "抵押金=出价-卖价",
		},
		"trans": {
			Type: AmountType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				v := p.Source.(*MetaTxOutPair)
				return v.GetTransAmount(objs)
			},
			Description: "交易费",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*MetaTxOutPair)
		return ok
	},
	Description: "卖卖交易对相关金额",
})

var MetaTxOutPairType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetaTxOutPairType",
	Fields: graphql.Fields{
		"purchase": {
			Type: graphql.NewNonNull(MetaTxOutType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOutPair)
				return v.Purchase, nil
			},
			Description: "购买信息",
		},
		"sell": {
			Type: graphql.NewNonNull(MetaTxOutType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				v := p.Source.(*MetaTxOutPair)
				return v.Sell, nil
			},
			Description: "出售信息",
		},
		"amount": {
			Type: graphql.NewNonNull(PairAmountType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source, nil
			},
			Description: "相关金额",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		_, ok := p.Value.(*MetaTxOutPair)
		return ok
	},
	Description: "卖卖交易对信息",
})

//获取抵押金额
func (pair *MetaTxOutPair) GetDepositAmount() (xginx.Amount, error) {
	pfee, err := pair.GetPayAmount()
	if err != nil {
		return 0, err
	}
	sfee, err := pair.GetProductAmount()
	if err != nil {
		return 0, err
	}
	deposit := pfee - sfee
	if !deposit.IsRange() {
		return 0, fmt.Errorf("deposit fee error")
	}
	return deposit, nil
}

//获取产品金额
func (pair *MetaTxOutPair) GetProductAmount() (xginx.Amount, error) {
	a := pair.Sell.Out.Value
	if !a.IsRange() {
		return 0, fmt.Errorf("fee value error")
	}
	return a, nil
}

//获取购买支付得金额
func (pair *MetaTxOutPair) GetPayAmount() (xginx.Amount, error) {
	b, err := pair.GetProductAmount()
	if err != nil {
		return 0, err
	}
	a := pair.Purchase.Out.Value - b
	if !a.IsRange() {
		return 0, fmt.Errorf("fee value error")
	}
	return a, nil
}

//获取交易费
func (pair *MetaTxOutPair) GetTransAmount(objs Objects) (xginx.Amount, error) {
	a, err := pair.Purchase.Tx.GetTransAmount(objs.BlockIndex())
	if err != nil {
		return 0, err
	}
	if !a.IsRange() {
		return 0, fmt.Errorf("fee value error")
	}
	return a, nil
}

//创建双发控制账户,此账户密钥被买卖双方控制,当购买交易确认后,需要双发签名
func (pair *MetaTxOutPair) NewAccount() (*xginx.AccountInfo, error) {
	//购买方私钥id
	bbkid, err := pair.Purchase.Meta.GetKID()
	if err != nil {
		return nil, err
	}
	//出售方私钥id
	sbkid, err := pair.Sell.Meta.GetKID()
	if err != nil {
		return nil, err
	}
	return xginx.NewTempAccountInfo(2, 2, false, []string{sbkid, bbkid})
}

//检测控制地址,如果买家收到此交易还要检测sbkid私钥是否是自己的
func (pair *MetaTxOutPair) CheckAddress(objs Objects) error {
	acc, err := pair.NewAccount()
	if err != nil {
		return err
	}
	lcks, err := pair.Purchase.Out.Script.ToLocked()
	if err != nil {
		return err
	}
	if acc.MustAddress() != lcks.Address() {
		return fmt.Errorf("address error")
	}
	return nil
}

//检测买卖交易对是否正确
func (pair *MetaTxOutPair) Check(objs Objects) error {
	bi := objs.BlockIndex()
	//检测购买交易的金额是否存在区块链上
	if !pair.Purchase.Tx.CoinsIsValid(bi) {
		return fmt.Errorf("purchase tx coins not valid")
	}
	//检测出售交易是否合法
	err := pair.Sell.Tx.Verify(bi)
	if err != nil {
		return err
	}
	//检测控制地址
	err = pair.CheckAddress(objs)
	if err != nil {
		return err
	}
	//检查金额
	sfee, err := pair.GetProductAmount()
	if err != nil {
		return err
	}
	bfee, err := pair.GetPayAmount()
	if err != nil {
		return err
	}
	if bfee < sfee {
		return fmt.Errorf("pay fee < product fee")
	}
	return nil
}

//获取出售信息
func (mbt *MetaTxOut) GetSell(objs Objects) (*MetaTxOut, error) {
	ret := &MetaTxOut{}
	bid, err := mbt.Meta.ID()
	if err != nil {
		return nil, err
	}
	//转换为购买id
	sid := bid.To(MetaTypeSell)
	//获取购买的产品信息,当区块确认时如果有销售交易信息肯定已经存入doc系统
	meta, err := GetMetaBody(objs.DocDB(), sid)
	if err != nil {
		return nil, err
	}
	if meta.Type != MetaTypeSell {
		return nil, fmt.Errorf("sell meta type error")
	}
	//当购买时应该可以获取的产品所在的交易信息
	tx, out, err := meta.GetTX(objs.BlockIndex())
	if err != nil {
		return nil, err
	}
	ret.Index = meta.Index.ToInt()
	ret.Meta = meta
	ret.Tx = tx
	ret.Out = out
	return ret, nil
}

//防止重复攻击的uuid,可能返回多个,只要有一个被保存就说明已经提交的交换系统中
func (mbt *MetaTxOut) UUID() ([]string, error) {
	if mbt.Meta.Type == MetaTypeSell {
		id, err := mbt.Tx.ID()
		if err != nil {
			return nil, err
		}
		sid := fmt.Sprintf("%s%.4d", id.String(), mbt.Index)
		return []string{sid}, nil
	}
	if mbt.Meta.Type == MetaTypePurchase {
		ids := []string{}
		for _, in := range mbt.Tx.Ins {
			id := fmt.Sprintf("%s%.4d", in.OutHash.String(), in.OutIndex.ToInt())
			ids = append(ids, id)
		}
		return ids, nil
	}
	return nil, fmt.Errorf("uuid get error")
}

func (mbt *MetaTxOut) NewPair(objs Objects) (*MetaTxOutPair, error) {
	ret := &MetaTxOutPair{}
	ret.Purchase = mbt
	sell, err := mbt.GetSell(objs)
	if err != nil {
		return nil, err
	}
	ret.Sell = sell
	return ret, nil
}

//重新定义TX
type PurchaseTX xginx.TX

//从tx创建
func NewPurchaseTX(tx *xginx.TX) *PurchaseTX {
	stx := PurchaseTX(*tx)
	return &stx
}

//从交易中获取购买信息
func (stx *PurchaseTX) FindMeta() (*MetaTxOut, error) {
	ret := &MetaTxOut{}
	tx := (*xginx.TX)(stx)
	for idx, out := range tx.Outs {
		lcks, err := out.Script.ToLocked()
		if err != nil {
			continue
		}
		mbb, err := MetaCoder.Decode(lcks.Meta)
		if err != nil {
			continue
		}
		mb, err := ParseMetaBody(mbb)
		if err != nil {
			continue
		}
		if mb.Type != MetaTypePurchase {
			continue
		}
		mb.Index = xginx.VarUInt(idx)
		mb.TxID = tx.MustID()
		ret.Tx = tx
		ret.Index = idx
		ret.Out = out
		ret.Meta = mb
		return ret, nil
	}
	return nil, fmt.Errorf("not found")
}

//获取购买交易对
func (stx *PurchaseTX) NewPair(objs Objects) (*MetaTxOutPair, error) {
	//获取购买信息
	pmb, err := stx.FindMeta()
	if err != nil {
		return nil, err
	}
	pair, err := pmb.NewPair(objs)
	if err != nil {
		return nil, err
	}
	err = pair.Check(objs)
	if err != nil {
		return nil, err
	}
	return pair, nil
}

//转账接口实现
type eshoptranslistener struct {
	bi        *xginx.BlockIndex //链指针
	keydb     xginx.IKeysDB     //账户数据库指针
	docdb     xginx.IDocSystem
	senders   []SenderInfo
	receivers []ReceiverInfo
	typ       byte
}

//获取产品信息
func (lis *eshoptranslistener) getMetaBody(meta string) ([]byte, error) {
	//如果是购买,直接返回信息
	if lis.typ == 0 || lis.typ == MetaTypePurchase {
		return []byte(meta), nil
	}
	//如果是出售从文档中获取数据
	mb, err := GetMetaBody(lis.docdb, xginx.DocumentIDFromHex(meta))
	if err != nil {
		return nil, err
	}
	sm, err := mb.To()
	if err != nil {
		return nil, err
	}
	return []byte(sm), nil
}

//fee 交易费
func (lis *eshoptranslistener) NewTx(fee xginx.Amount) (*xginx.TX, error) {
	if !fee.IsRange() {
		return nil, fmt.Errorf("fee %d error", fee)
	}
	tx := xginx.NewTx(xginx.DefaultExeTime, xginx.DefaultTxScript)
	//输出金额总计
	sum := fee
	for _, v := range lis.receivers {
		sum += v.Amount
	}
	//最后一个输入地址默认作为找零地址（如果有零）
	var keepaddr xginx.Address
	//使用哪些金额
	for _, sender := range lis.senders {
		coin, err := lis.bi.GetCoinWithAddress(sender.Addr, sender.TxID, sender.Index)
		if err != nil {
			continue
		}
		//获取消费金额对应的账户
		wits, err := lis.keydb.NewWitnessScript(sender.Addr, []byte(sender.Script))
		if err != nil {
			return nil, err
		}
		//生成待签名的输入
		in, err := coin.NewTxIn(wits)
		if err != nil {
			return nil, err
		}
		tx.Ins = append(tx.Ins, in)
		//保存最后一个地址
		if sender.Keep {
			keepaddr = coin.GetAddress()
		}
		sum -= coin.Value
		if sum <= 0 {
			break
		}
	}
	//没有减完，余额不足
	if sum > 0 {
		return nil, errors.New("insufficient balance")
	}
	//检测找零地址是否正确 sum < 0表示有找零
	if err := keepaddr.Check(); sum < 0 && err != nil {
		return nil, fmt.Errorf("keep address error %w", err)
	}
	//转出到其他账号的输出
	for _, v := range lis.receivers {
		meta, err := lis.getMetaBody(v.Meta)
		if err != nil {
			return nil, err
		}
		//压缩数据
		bmeta, err := MetaCoder.Encode(meta)
		if err != nil {
			return nil, err
		}
		out, err := v.Addr.NewTxOut(v.Amount, bmeta, []byte(v.Script))
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	//剩余的需要找零钱给自己，否则金额就会丢失
	if amt := -sum; amt > 0 {
		//默认找零到最后一个地址
		out, err := keepaddr.NewTxOut(amt, nil, xginx.DefaultLockedScript)
		if err != nil {
			return nil, err
		}
		tx.Outs = append(tx.Outs, out)
	}
	return tx, nil
}

//创建一个处理器仅仅用于签名
func (obj Objects) NewTransForSign(typ byte) IShopTrans {
	lis := &eshoptranslistener{
		bi:        obj.BlockIndex(),
		senders:   []SenderInfo{},
		receivers: []ReceiverInfo{},
		keydb:     obj.KeyDB(),
		docdb:     obj.DocDB(),
		typ:       typ,
	}
	return lis
}

//创建一个转账处理器,使用默认的输入输出脚本
//senders如果指定了可用发送金额
func (obj Objects) NewTrans(senders []SenderInfo, receivers []ReceiverInfo, typ ...byte) IShopTrans {
	lis := &eshoptranslistener{
		bi:        obj.BlockIndex(),
		senders:   senders,
		receivers: receivers,
		keydb:     obj.KeyDB(),
		docdb:     obj.DocDB(),
	}
	if len(typ) > 0 {
		lis.typ = typ[0]
	}
	return lis
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
	//如果没有账户信息,并且是买入,返回忽略错误
	if err != nil && lis.typ == MetaTypePurchase {
		return xginx.ErrIgnoreSignError
	}
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

//获取指定coins
func (lis *eshoptranslistener) getsenderscoins() xginx.Coins {
	ret := xginx.Coins{}
	for _, sender := range lis.senders {
		coin, err := lis.bi.GetCoinWithAddress(sender.Addr, sender.TxID, sender.Index)
		if err != nil {
			continue
		}
		ret = append(ret, coin)
	}
	return ret
}

//获取使用的金额列表
func (lis *eshoptranslistener) GetCoins(amt xginx.Amount) xginx.Coins {
	//如果指定了固定金额
	if len(lis.senders) > 0 {
		return lis.getsenderscoins()
	}
	//获取所有的金额账户
	ret := xginx.Coins{}
	//每次获取10个,直到凑足可用金额
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
		_, ok := p.Value.(*xginx.WitnessScript)
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
				meta, err := MetaCoder.Decode(script.Meta)
				if err != nil {
					return nil, err
				}
				if len(meta) == 0 {
					return nil, nil
				}
				return ParseMetaBody(meta)
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
		"ispool": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				tx := p.Source.(*xginx.TX)
				return tx.IsPool(), nil
			},
			Description: "是否在交易池",
		},
		"verify": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				bi := objs.BlockIndex()
				tx := p.Source.(*xginx.TX)
				return tx.Verify(bi) == nil, nil
			},
			Description: "校验交易是否有效",
		},
		"coinsIsValid": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				bi := objs.BlockIndex()
				tx := p.Source.(*xginx.TX)
				return tx.CoinsIsValid(bi), nil
			},
			Description: "验证输入对应的金额是否可用",
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
		"joinTxPool": {
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				bi := objs.BlockIndex()
				txp := bi.GetTxPool()
				tx := p.Source.(*xginx.TX)
				err := txp.PushTx(bi, tx)
				if err != nil {
					return NewError(100, err)
				}
				return true, nil
			},
			Description: "加入交易池",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		tx, ok := p.Value.(*xginx.TX)
		return ok && tx != nil
	},
	Description: "交易类型",
})

var PageInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "PageInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"skip": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "跳过的数量",
		},
		"limit": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "每页的数量",
		},
	},
	Description: "分页参数输入",
})

var listTxPool = &graphql.Field{
	Name:        "ListTxPool",
	Type:        graphql.NewList(graphql.NewNonNull(TXType)),
	Description: "获取交易池中的交易",
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		bi := objs.BlockIndex()
		tp := bi.GetTxPool()
		return tp.AllTxs(), nil
	},
}

//从交易加载交易对
func LoadPairWithTx(objs Objects, bb []byte) (*MetaTxOutPair, error) {
	r := xginx.NewReader(bb)
	tx := &xginx.TX{}
	err := tx.Decode(r)
	if err != nil {
		return nil, err
	}
	stx := NewPurchaseTX(tx)
	return stx.NewPair(objs)
}

var loadMetaPair = &graphql.Field{
	Name: "loadMetaPair",
	Type: graphql.NewNonNull(MetaTxOutPairType),
	Args: graphql.FieldConfigArgument{
		"data": {
			Type:        graphql.NewNonNull(graphql.String),
			Description: "base64编码的交易数据",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		objs := GetObjects(p)
		args := &EncodedTx{}
		err := DecodeArgs(p, args)
		if err != nil || args.Data == "" {
			return NewError(100, err)
		}
		bb, err := base64.StdEncoding.DecodeString(args.Data)
		if err != nil {
			return NewError(101, err)
		}
		return LoadPairWithTx(objs, bb)
	},
	Description: "从购买交易加载买卖数据对",
}

var loadTxInfo = &graphql.Field{
	Name: "LoadTxInfo",
	Args: graphql.FieldConfigArgument{
		"id": {
			Type:        graphql.NewNonNull(HashType),
			Description: "交易id",
		},
	},
	Type:        graphql.NewNonNull(TXType),
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

var SenderInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SenderInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"addr": {
			Type:        graphql.NewNonNull(graphql.String),
			Description: "金额地址",
		},
		"keep": {
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "是否作为找零地址",
		},
		"txId": {
			Type:        graphql.NewNonNull(HashType),
			Description: "金额交易id",
		},
		"index": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "金额交易索引",
		},
		"script": {
			Type:         graphql.String,
			DefaultValue: string(xginx.DefaultInputScript),
			Description:  "输入脚本",
		},
	},
	Description: "转账到指定地址输入参数",
})

var ReceiverInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "ReceiverInput",
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
			Type:        graphql.String,
			Description: "输出meta数据",
		},
		"script": {
			Type:         graphql.String,
			DefaultValue: string(xginx.DefaultLockedScript),
			Description:  "输出执行脚本",
		},
	},
	Description: "转账到指定地址输入参数",
})

var MetaEleInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetaEleInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type": {
			Type:        graphql.NewNonNull(EleType),
			Description: "元素类型",
		},
		"size": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "元素长度",
		},
		"sum": {
			Type:        graphql.NewNonNull(graphql.String),
			Description: "数据校验和Ripemd160算法,使用小写",
		},
		"body": {
			Type:        graphql.NewNonNull(graphql.String),
			Description: "元素内容",
		},
	},
	Description: "转账到指定地址输入参数",
})

var TxMetaInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "TxMetaInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type": {
			Type:        graphql.NewNonNull(BodyType),
			Description: "数据类型",
		},
		"tags": {
			Type:        graphql.NewList(graphql.NewNonNull(graphql.String)),
			Description: "关键字标签",
		},
		"eles": {
			Type:        graphql.NewList(graphql.NewNonNull(MetaEleInput)),
			Description: "关键字标签",
		},
	},
	Description: "转账到指定地址输入参数",
})

var createTxMeta = &graphql.Field{
	Name: "CreateTxMeta",
	Args: graphql.FieldConfigArgument{
		"meta": {
			Type:        graphql.NewNonNull(TxMetaInput),
			Description: "数据内容",
		},
	},
	Type: MetaBodyType,
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		mb := &MetaBody{}
		err := DecodeArgs(p, mb, "meta")
		if err != nil {
			return NewError(100, err)
		}
		err = mb.Check(p.Context)
		if err != nil {
			return NewError(101, err)
		}
		return mb, err
	},
	Description: "创建一个meta数据",
}

var transfer = &graphql.Field{
	Name: "Transfer",
	Type: graphql.NewNonNull(TXType),
	Args: graphql.FieldConfigArgument{
		"sender": {
			Type:         graphql.NewList(SenderInput),
			DefaultValue: nil,
			Description:  "可定制的付款信息,如果为空系统自动查询可用金额",
		},
		"receiver": {
			Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(ReceiverInput))),
			Description: "收款人列表",
		},
		"fee": {
			Type:         graphql.Int,
			DefaultValue: 0,
			Description:  "交易费",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		senders := []SenderInfo{}
		err := DecodeArgs(p, &senders, "sender")
		if err != nil {
			return NewError(100, err)
		}
		receiver := []ReceiverInfo{}
		err = DecodeArgs(p, &receiver, "receiver")
		if err != nil {
			return NewError(101, err)
		}
		for _, v := range receiver {
			if v.Meta == "" {
				continue
			}
			_, err = v.ParseMeta()
			if err != nil {
				return NewError(102, "parse meta error=%v meta=%s", err, v.Meta)
			}
		}
		fee := p.Args["fee"].(int)
		objs := GetObjects(p)
		bi := objs.BlockIndex()
		lis := objs.NewTrans(senders, receiver)
		tx, err := lis.NewTx(xginx.Amount(fee))
		if err != nil {
			return NewError(103, err)
		}
		err = tx.Sign(bi, lis)
		if err != nil {
			return NewError(104, err)
		}
		bp := bi.GetTxPool()
		err = bp.PushTx(bi, tx)
		if err != nil {
			return NewError(105, err)
		}
		return tx, nil
	},
	Description: "转账指定的金额到其他账号,返回交易信息",
}
