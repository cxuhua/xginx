package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cxuhua/xginx"

	"github.com/graphql-go/graphql"
)

//消息广播类型定义 定制 xginx.MsgBroadInfo 消息
const (
	//出售广播类型
	MsgActionSellMeta = uint8(0xf0) // ShopMeta
	//购买交易
	MsgActionPurchaseTx = uint8(0xf1) // xginx.TX 收到交易信息,如果正确签名发布交易
)

//创建一个消息广播来购买指定的产品
func NewMsgBroadInfoWithPurchaseTx(tx *xginx.TX) (*xginx.MsgBroadInfo, error) {
	msg := &xginx.MsgBroadInfo{}
	msg.Action = MsgActionPurchaseTx
	w := xginx.NewWriter()
	err := tx.Encode(w)
	if err != nil {
		return nil, err
	}
	msg.Msg = w.Bytes()
	return msg, nil
}

//从交易中分离出meta数据
func SeparateMetaBodyFromTx(tx *xginx.TX) []*MetaBody {
	mbs := []*MetaBody{}
	for _, out := range tx.Outs {
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
		mbs = append(mbs, mb)
	}
	return mbs
}

//购买产品元素
const (
	//ID MetaEleTEXT 执行metaid
	PurchaseProductUUIDEleIndex = iota
	//购买者提供的私钥id部分
	PurchaseProductPartKIDIndex
	//rsa id
	PurchaseProductRSAIDIndex
	//信息开始,所有信息通过rsa公钥加密,并且用b58编码
	PurchaseProductInfoStartIndex
)

//出售产品元素索引定义,必须存在的索引
const (
	//ID MetaEleTEXT
	SellProductUUIDEleIndex = iota
	//类型 MetaEleTEXT 不能为空
	//购买时用于转账的私钥ID,和购买者提供的私钥ID合成一个2-2账号
	//此账户由买卖双方控制,交易完成后从此账户将金额转入卖家的地址(发布出售产品的地址)
	//类型 MetaEleKID
	SellProductPartKIDIndex
	//用于加密买家信息的RSA公钥
	//类型 MetaEleRSA
	SellProductEncryptionIndex
	//产品信息开始索引
	SellProductInfoStartIndex
)

//检测是否包含合法的文档ID
func (mb MetaBody) HasID() bool {
	var ele MetaEle
	var err error
	if mb.Type == MetaTypeSell {
		ele, err = mb.GetEle(SellProductUUIDEleIndex)
	} else if mb.Type == MetaTypePurchase {
		ele, err = mb.GetEle(PurchaseProductUUIDEleIndex)
	} else {
		return false
	}
	if err != nil {
		return false
	}
	if len(ele.Body) != xginx.DocumentIDLen*2 {
		return false
	}
	return true
}

//获取KID
func (mb MetaBody) GetEle(idx int) (MetaEle, error) {
	var ele MetaEle
	if len(mb.Eles) <= idx {
		return ele, fmt.Errorf("miss kid")
	}
	ele = mb.Eles[idx]
	if ele.Body == "" {
		return ele, fmt.Errorf("ele body emtpy")
	}
	return ele, nil
}

//获取购买meta中的加密信息并且解密body,如果能解密并且获取到这个信息,才是发给我的,因为只有我才有私钥
func (mb MetaBody) GetPurchaseInfo(keydb xginx.IKeysDB) ([]MetaEle, error) {
	if mb.Type != MetaTypePurchase {
		return nil, fmt.Errorf("mb type error")
	}
	rsapri, err := mb.GetPurchaseRSA(keydb)
	if err != nil {
		return nil, err
	}
	eles := []MetaEle{}
	for _, ele := range mb.Eles[PurchaseProductInfoStartIndex:] {
		bb, err := base64.StdEncoding.DecodeString(ele.Body)
		if err != nil {
			return nil, err
		}
		str, err := rsapri.Decrypt(bb)
		if err != nil {
			return nil, err
		}
		eles = append(eles, MetaEle{
			Type: ele.Type,
			Body: string(str),
		})
	}
	return eles, nil
}

//获取购买meta中的rsa id并加载私钥 如果密钥是我生成的才能获取到并且解密
func (mb MetaBody) GetPurchaseRSA(keydb xginx.IKeysDB) (*xginx.RSAPrivateKey, error) {
	if mb.Type != MetaTypePurchase {
		return nil, fmt.Errorf("mb type error")
	}
	ele, err := mb.GetEle(PurchaseProductRSAIDIndex)
	if err != nil {
		return nil, err
	}
	if ele.Type != MetaEleTEXT {
		return nil, fmt.Errorf("kid kind error")
	}
	return keydb.GetRSA(ele.Body)
}

//获取加密公钥
func (mb MetaBody) GetSellRSA() (*xginx.RSAPublicKey, error) {
	if mb.Type != MetaTypeSell {
		return nil, fmt.Errorf("mb type error")
	}
	ele, err := mb.GetEle(SellProductEncryptionIndex)
	if err != nil {
		return nil, err
	}
	if ele.Type != MetaEleRSA {
		return nil, fmt.Errorf("kid kind error")
	}
	return xginx.LoadRSAPublicKey(ele.Body)
}

//获取KID
func (mb MetaBody) GetKID() (string, error) {
	var ele MetaEle
	var err error
	if mb.Type == MetaTypeSell {
		ele, err = mb.GetEle(SellProductPartKIDIndex)
	} else if mb.Type == MetaTypePurchase {
		ele, err = mb.GetEle(PurchaseProductPartKIDIndex)
	} else {
		return "", fmt.Errorf("mb type error")
	}
	if err != nil {
		return "", err
	}
	if ele.Type != MetaEleKID {
		return "", fmt.Errorf("kid kind error")
	}
	if ele.Body == "" {
		return "", fmt.Errorf("kid emtpy")
	}
	_, err = xginx.DecodePublicHash(ele.Body)
	if err != nil {
		return "", err
	}
	return ele.Body, nil
}

func (mb MetaBody) ID() (xginx.DocumentID, error) {
	var ele MetaEle
	var err error
	if mb.Type == MetaTypeSell {
		ele, err = mb.GetEle(SellProductUUIDEleIndex)
	} else if mb.Type == MetaTypePurchase {
		ele, err = mb.GetEle(PurchaseProductUUIDEleIndex)
	} else {
		return xginx.NilDocumentID, fmt.Errorf("mb type error")
	}
	if err != nil {
		return xginx.NilDocumentID, err
	}
	return xginx.DocumentIDFromHex(ele.Body), nil
}

//获取metabody的id
func (mb MetaBody) MustID() xginx.DocumentID {
	var ele MetaEle
	var err error
	if mb.Type == MetaTypeSell {
		ele, err = mb.GetEle(SellProductUUIDEleIndex)
	} else if mb.Type == MetaTypePurchase {
		ele, err = mb.GetEle(PurchaseProductUUIDEleIndex)
	}
	if err != nil {
		panic(err)
	}
	return xginx.DocumentIDFromHex(ele.Body)
}

//标签用,号隔开,使用这个特殊索引
const (
	ProductTagsEleIndex = 100
)

//账户类型
var SellEleIndexType = graphql.NewEnum(graphql.EnumConfig{
	Name: "SellEleIndexType",
	Values: graphql.EnumValueConfigMap{
		"UUID": {
			Value:       SellProductUUIDEleIndex,
			Description: "产品ID",
		},
		"TAGS": {
			Value:       ProductTagsEleIndex,
			Description: "产品标签",
		},
		"KID": {
			Value:       SellProductPartKIDIndex,
			Description: "私钥ID",
		},
		"RSA": {
			Value:       SellProductEncryptionIndex,
			Description: "加密rsa公钥",
		},
		"INFO": {
			Value:       SellProductInfoStartIndex,
			Description: "最大固定索引,这个开始就是产品信息",
		},
	},
})

var (
	//临时产品缓存
	tempproducts = &sync.Map{}
)

var ProductType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ProductType",
	Fields: graphql.Fields{
		"check": {
			Name: "Check",
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb, ok := p.Source.(*MetaBody)
				if !ok {
					return NewError(200, "source value type error")
				}
				return mb.Check(p.Context) == nil, nil
			},
			Description: "验证数据合法性",
		},
		"save": {
			Name: "Save",
			Type: HashType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				mb, ok := p.Source.(*MetaBody)
				if !ok {
					return NewError(200, "source value type error")
				}
				if err := mb.Check(p.Context); err != nil {
					return NewError(201, err)
				}
				id := mb.MustID()
				objs := GetObjects(p)
				docdb := objs.DocDB()
				doc, err := mb.ToDocument()
				if err != nil {
					return NewError(202, err)
				}
				if has, err := docdb.Has(doc.ID); err != nil {
					return NewError(203, err)
				} else if has {
					return NewError(204, "doc %s exists", id)
				}
				err = docdb.Insert(doc)
				if err != nil {
					return NewError(205, err)
				}
				//删除临时的
				tempproducts.Delete(id)
				return doc.ID, nil
			},
			Description: "保存到文档,成功文档ID",
		},
		"setEle": {
			Name: "SetEle",
			Args: graphql.FieldConfigArgument{
				"index": {
					Type:        graphql.NewNonNull(SellEleIndexType),
					Description: "内容索引",
				},
				"type": {
					Type:        graphql.NewNonNull(EleType),
					Description: "内容类型",
				},
				"body": {
					Type:        graphql.NewNonNull(graphql.String),
					Description: "内容数据,如果是rsa,传入rsa id",
				},
			},
			Type: graphql.NewNonNull(MetaBodyType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				objs := GetObjects(p)
				kdb := objs.KeyDB()
				args := struct {
					Index int
					Type  string
					Body  string
				}{}
				err := DecodeArgs(p, &args)
				if err != nil {
					return NewError(201, err)
				}
				mb, ok := p.Source.(*MetaBody)
				if !ok {
					return NewError(202, "source value type error")
				}
				//如果是rsa私钥从kdb数据库获取,args.body代表rsa id
				if args.Index == SellProductEncryptionIndex {
					rsa, err := kdb.GetRSA(args.Body)
					if err != nil {
						return NewError(203, err)
					}
					pub := rsa.PublicKey()
					args.Body, err = pub.Dump()
					if err != nil {
						return NewError(204, err)
					}
				}
				err = mb.SetEle(p.Context, args.Index, args.Type, args.Body)
				if err != nil {
					return NewError(205, err)
				}
				return mb, nil
			},
			Description: "设置产品元素数据",
		},
	},
	IsTypeOf: func(p graphql.IsTypeOfParams) bool {
		return true
	},
	Description: "产品相关操作",
})

var product = &graphql.Field{
	Name: "Product",
	Type: ProductType,
	Args: graphql.FieldConfigArgument{
		"id": {
			Type:        graphql.NewNonNull(HashType),
			Description: "创建产品返回的id",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id := p.Args["id"].(xginx.DocumentID)
		pptr, ok := tempproducts.Load(id)
		if !ok {
			return NewError(100, "id %s miss", id.String())
		}
		return pptr, nil
	},
	Description: "产品相关操作",
}

var listTempProduct = &graphql.Field{
	Name: "ListTempProduct",
	Type: graphql.NewList(graphql.NewNonNull(MetaBodyType)),
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		mbs := []*MetaBody{}
		tempproducts.Range(func(key, value interface{}) bool {
			mbs = append(mbs, value.(*MetaBody))
			return true
		})
		return mbs, nil
	},
	Description: "获取临时保存的产品",
}

var findProduct = &graphql.Field{
	Name: "FindProduct",
	Type: graphql.NewList(graphql.NewNonNull(MetaBodyType)),
	Args: graphql.FieldConfigArgument{
		"key": {
			Type:         graphql.String,
			DefaultValue: "",
			Description:  "查询关键字",
		},
		"prefix": {
			Type:         graphql.Boolean,
			DefaultValue: false,
			Description:  "是否按前缀查询",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		mbs := []*MetaBody{}
		key := p.Args["key"].(string)
		prefix := p.Args["prefix"].(bool)
		objs := GetObjects(p)
		docdb := objs.DocDB()
		var iter xginx.IDocIter
		if prefix {
			iter = docdb.Prefix(key)
		} else {
			iter = docdb.Find(key)
		}
		_ = iter.Each(func(doc *xginx.Document) error {
			mb, err := ShopMeta(doc.Body).To()
			if err != nil {
				panic(err)
			}
			mbs = append(mbs, mb)
			return err
		})
		return mbs, nil
	},
	Description: "根据关键字查询数据",
}

var loadProduct = &graphql.Field{
	Name: "LoadProduct",
	Type: graphql.NewNonNull(MetaBodyType),
	Args: graphql.FieldConfigArgument{
		"id": {
			Type:        graphql.NewNonNull(HashType),
			Description: "documentid",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id := p.Args["id"].(xginx.DocumentID)
		objs := GetObjects(p)
		docdb := objs.DocDB()
		doc, err := docdb.Get(id, true)
		if err != nil {
			return NewError(200, err)
		}
		mb, err := ShopMeta(doc.Body).To()
		if err != nil {
			return NewError(201, err)
		}
		return mb, nil
	},
	Description: "从文档系统通过id获取数据",
}

func GetMetaBody(db xginx.IDocSystem, id xginx.DocumentID) (*MetaBody, error) {
	doc, err := db.Get(id, true)
	if err != nil {
		return nil, err
	}
	mb, err := ShopMeta(doc.Body).To()
	if err != nil {
		return nil, err
	}
	mb.TxID = doc.TxID
	mb.Index = doc.Index
	return mb, nil
}

//NewDocID 创建一个递增唯一的ID
func NewDocID() xginx.HASH160 {
	id := xginx.HASH160{}
	now := time.Now().UnixNano()
	binary.BigEndian.PutUint64(id[:], uint64(now))
	return id
}

var newTempProduct = &graphql.Field{
	Name: "NewTempProduct",
	Type: graphql.NewNonNull(MetaBodyType),
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id := xginx.NewDocumentID()
		product := NewProductMeta()
		err := product.SetEle(p.Context, SellProductUUIDEleIndex, MetaEleUUID, id.String())
		if err != nil {
			return NewError(102, err)
		}
		tempproducts.Store(id, product)
		return product, nil
	},
	Description: "创建一个产品放在临时缓存",
}

var purchaseProduct = &graphql.Field{
	Name: "PurchaseProduct",
	Type: graphql.NewNonNull(TXType),
	Args: graphql.FieldConfigArgument{
		"pid": {
			Type:        graphql.NewNonNull(HashType),
			Description: "将要购买的产品id",
		},
		"kid": {
			Type:        graphql.NewNonNull(graphql.String),
			Description: "使用一个自己的密钥id",
		},
		"sender": {
			Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(SenderInput))),
			Description: "使用哪些金额来购买",
		},
		"info": {
			Type:        graphql.NewNonNull(graphql.String),
			Description: "购买信息,将由公钥加密",
		},
		"offer": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "出价,买家设置的价格,可以和产品价格不一致,是否接受由卖家决定",
		},
		"fee": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "交易费",
		},
	},
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		args := struct {
			PID    xginx.DocumentID
			KID    string
			Sender []SenderInfo
			Info   string
			Offer  xginx.Amount
			Fee    xginx.Amount
		}{}
		err := DecodeArgs(p, &args)
		if err != nil {
			return NewError(100, err)
		}
		//创建购买meta信息
		bmeta := &MetaBody{
			Type: MetaTypePurchase,
			Eles: make([]MetaEle, PurchaseProductInfoStartIndex),
		}
		objs := GetObjects(p)
		docdb := objs.DocDB()
		keydb := objs.KeyDB()
		bi := objs.BlockIndex()
		//获取私钥信息
		_, err = keydb.LoadPrivateKey(args.KID)
		if err != nil {
			return NewError(101, err)
		}
		//获取信息并且带扩展信息
		meta, err := GetMetaBody(docdb, args.PID)
		if err != nil {
			return NewError(102, err)
		}
		if meta.Type != MetaTypeSell {
			return NewError(103, "meta type error")
		}
		//获取卖家提供的私钥地址
		sellkid, err := meta.GetKID()
		if err != nil {
			return NewError(104, err)
		}
		//获取信息加密用公钥
		rsapub, err := meta.GetSellRSA()
		if err != nil {
			return NewError(105, err)
		}
		//购买信息不能为空
		if args.Info == "" {
			return NewError(106, "info miss")
		}
		//构造2-2账户,由买家和卖家控制
		acc, err := xginx.NewTempAccountInfo(2, 2, false, []string{sellkid, args.KID})
		if err != nil {
			return NewError(107, err)
		}
		//设置产品ID
		bmeta.Eles[PurchaseProductUUIDEleIndex] = MetaEle{
			Type: MetaEleUUID,
			Body: args.PID.String(),
		}
		//设置2-2部分密钥
		bmeta.Eles[PurchaseProductPartKIDIndex] = MetaEle{
			Type: MetaEleKID,
			Body: args.KID,
		}
		//设置加密公钥
		bmeta.Eles[PurchaseProductRSAIDIndex] = MetaEle{
			Type: MetaEleTEXT,
			Body: rsapub.MustID(),
		}
		//加密信息并设置到meta
		infob, err := rsapub.Encrypt([]byte(args.Info))
		if err != nil {
			return NewError(108, err)
		}
		//信息使用base64标准编码
		bmeta.Eles = append(bmeta.Eles, MetaEle{
			Type: MetaEleTEXT,
			Body: base64.StdEncoding.EncodeToString(infob),
		})
		//获取产品所在的输出
		txout, err := meta.GetTxOut(bi)
		if err != nil {
			return NewError(109, err)
		}
		smeta, err := bmeta.To()
		if err != nil {
			return NewError(110, err)
		}
		receiver := []ReceiverInfo{
			{
				Addr:   acc.MustAddress(),
				Amount: txout.Value + args.Offer, //包括商品本身的价值和买家的出价,在成交后会一起转给卖家
				Meta:   string(smeta),
				Script: string(xginx.DefaultLockedScript),
			},
		}
		//获取锁定脚本
		lcks, err := txout.Script.ToLocked()
		if err != nil {
			return NewError(112, err)
		}
		//添加默认的购买产品输出
		senders := []SenderInfo{{
			Addr:   lcks.Address(),
			TxID:   meta.TxID,
			Index:  meta.Index,
			Script: string(xginx.DefaultInputScript),
			Keep:   false,
		}}
		//其他输入,必须先消耗产品输出
		stmps := []SenderInfo{}
		err = DecodeArgs(p, &stmps, "sender")
		if err != nil {
			return NewError(113, err)
		}
		senders = append(senders, stmps...)
		//上传产品创建
		lis := objs.NewTrans(senders, receiver, MetaTypePurchase)
		tx, err := lis.NewTx(args.Fee)
		if err != nil {
			return NewError(114, err)
		}
		err = tx.Sign(bi, lis)
		if err != nil && !errors.Is(err, xginx.ErrIgnoreSignError) {
			return nil, err
		}
		//保存到临时交易区,使用产品id作为key,一个产品只需生成一个购买交易
		err = objs.TypeDB().Put(TypeDBTypeTTPTX, args.PID[:], tx)
		if err != nil {
			return NewError(115, err)
		}
		return tx, nil
	},
	Description: "生成购买交易",
}

var uploadProduct = &graphql.Field{
	Name: "UploadProduct",
	Type: graphql.NewNonNull(TXType),
	Args: graphql.FieldConfigArgument{
		"sender": {
			Type:        graphql.NewList(SenderInput),
			Description: "使用哪些金额作为产品保证金",
		},
		"receiver": {
			Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(ReceiverInput))),
			Description: "带metabody的产品信息的输出",
		},
		"fee": {
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "交易费",
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
				return NewError(102, "meta args miss", err, v.Meta)
			}
			//上传产品时meta代表产品id,优先从docdb获取
			docid := xginx.DocumentIDFromHex(v.Meta)
			if docid.Equal(xginx.NilDocumentID) {
				return NewError(103, "docid error")
			}
		}
		fee := p.Args["fee"].(int)
		if fee == 0 {
			return NewError(104, "fee error,must > 0")
		}
		objs := GetObjects(p)
		bi := objs.BlockIndex()
		//上传产品创建
		lis := objs.NewTrans(senders, receiver, MetaTypeSell)
		tx, err := lis.NewTx(xginx.Amount(fee))
		if err != nil {
			return NewError(105, err)
		}
		err = tx.Sign(bi, lis)
		if err != nil {
			return NewError(106, err)
		}
		bp := bi.GetTxPool()
		err = bp.PushTx(bi, tx)
		if err != nil {
			return NewError(107, err)
		}
		return tx, nil
	},
	Description: "上传产品到区块链",
}

//创建一个出售产品meta数据
func NewProductMeta() *MetaBody {
	return &MetaBody{
		Type: MetaTypeSell,
		Tags: []string{},
		Eles: make([]MetaEle, SellProductInfoStartIndex),
	}
}

//根据sum查询是否存在相同的文本或者url数据
func (mb *MetaBody) existstext(body string) (int, bool) {
	for idx, ele := range mb.Eles {
		if ele.Body == body {
			return idx, true
		}
	}
	return -1, false
}

//设置元素内容
func (mb *MetaBody) SetEle(ctx context.Context, idx int, typ string, body string) error {
	if body == "" {
		return fmt.Errorf("body empty")
	}
	var ele MetaEle
	var err error
	//如果设置的是标签
	if idx == ProductTagsEleIndex {
		mb.Tags = strings.Split(body, ",")
		return nil
	}
	if typ == MetaEleURL {
		ele, err = NewMetaUrl(ctx, body)
	} else {
		ele = NewMetaEle(typ, body)
	}
	if err != nil {
		return err
	}
	if ridx, has := mb.existstext(ele.Body); has {
		return fmt.Errorf("repeat data index= %d", ridx)
	}
	if idx >= SellProductInfoStartIndex {
		mb.Eles = append(mb.Eles, ele)
	} else {
		mb.Eles[idx] = ele
	}
	return nil
}
