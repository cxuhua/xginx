package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cxuhua/xginx"

	"github.com/graphql-go/graphql"
)

//产品元素索引定义,必须存在的索引
const (
	//ID MetaEleTEXT
	ProductUUIDEleIndex = iota
	//类型 MetaEleTEXT 不能为空
	//购买时用于转账的私钥ID,和购买者提供的私钥ID合成一个2-2账号
	//此账户由买卖双方控制,交易完成后从此账户将金额转入卖家的地址(发布出售产品的地址)
	//类型 MetaEleKID
	ProductPartKIDIndex
	//用于加密买家信息的RSA公钥
	//类型 MetaEleRSA
	ProductEncryptionIndex
	//产品信息开始索引
	ProductInfoMaxIndex
)

//检测是否包含合法的文档ID
func (mb MetaBody) HasID() bool {
	if len(mb.Eles) <= ProductUUIDEleIndex {
		return false
	}
	ele := mb.Eles[ProductUUIDEleIndex]
	if len(ele.Body) != xginx.DocumentIDLen*2 {
		return false
	}
	return true
}

//获取metabody的id
func (mb *MetaBody) MustID() xginx.DocumentID {
	ele := mb.Eles[ProductUUIDEleIndex]
	return xginx.DocumentIDFromHex(ele.Body)
}

//标签用,号隔开,使用这个特殊索引
const (
	ProductTagsEleIndex = 100
)

//账户类型
var EleIndexType = graphql.NewEnum(graphql.EnumConfig{
	Name: "EleIndexType",
	Values: graphql.EnumValueConfigMap{
		"UUID": {
			Value:       ProductUUIDEleIndex,
			Description: "产品ID",
		},
		"TAGS": {
			Value:       ProductTagsEleIndex,
			Description: "产品标签",
		},
		"KID": {
			Value:       ProductPartKIDIndex,
			Description: "私钥ID",
		},
		"RSA": {
			Value:       ProductEncryptionIndex,
			Description: "加密rsa公钥",
		},
		"INFO": {
			Value:       ProductInfoMaxIndex,
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
					Type:        graphql.NewNonNull(EleIndexType),
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
				if args.Index == ProductEncryptionIndex {
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
			return NewError(100, "id %s miss", id.Hex())
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
	if err == nil {
		return mb, nil
	}
	mbt, has := tempproducts.Load(id)
	if has {
		return mbt.(*MetaBody), nil
	}
	return nil, fmt.Errorf("not found netaboddy %s", id.Hex())
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
		err := product.SetEle(p.Context, ProductUUIDEleIndex, MetaEleUUID, id.Hex())
		if err != nil {
			return NewError(102, err)
		}
		tempproducts.Store(id, product)
		return product, nil
	},
	Description: "创建一个产品放在临时缓存",
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
			Description: "产品价格",
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
			mbid := xginx.NewHASH160(v.Meta)
			if mbid.Equal(xginx.ZERO160) {
				return NewError(103, "metaid error")
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
		Eles: make([]MetaEle, ProductInfoMaxIndex),
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
	if idx >= ProductInfoMaxIndex {
		mb.Eles = append(mb.Eles, ele)
	} else {
		mb.Eles[idx] = ele
	}
	return nil
}
