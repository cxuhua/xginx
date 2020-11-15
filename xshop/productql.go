package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/cxuhua/xginx"

	"github.com/graphql-go/graphql"
)

//从交易中分离出meta数据
func SeparateMetaBodyFromTx(tx *xginx.TX, typ byte) []*MetaBody {
	mbs := []*MetaBody{}
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
		mb.Index = xginx.VarUInt(idx)
		mb.TxID = tx.MustID()
		if mb.Type == typ {
			mbs = append(mbs, mb)
		}
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

func (mb *MetaBody) checkpurchase() error {
	if len(mb.Eles) < PurchaseProductInfoStartIndex {
		return fmt.Errorf("purchas eles num error")
	}
	_, err := mb.ID()
	if err != nil {
		return err
	}
	_, err = mb.GetKID()
	if err != nil {
		return err
	}
	_, err = mb.GetPurchaseRSAID()
	if err != nil {
		return err
	}
	return nil
}

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

func (mb *MetaBody) checksell() error {
	if len(mb.Eles) < SellProductInfoStartIndex {
		return fmt.Errorf("sell eles num error")
	}
	_, err := mb.ID()
	if err != nil {
		return err
	}
	_, err = mb.GetKID()
	if err != nil {
		return err
	}
	_, err = mb.GetSellRSA()
	if err != nil {
		return err
	}
	return nil
}

func (mb *MetaBody) CheckType() error {
	if mb.Type == MetaTypeSell {
		return mb.checksell()
	}
	if mb.Type == MetaTypePurchase {
		return mb.checkpurchase()
	}
	return nil
}

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
	//检测类型是否匹配
	id := xginx.DocumentIDFromHex(ele.Body)
	if id.Type() != mb.Type {
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
func (mb MetaBody) GetPurchaseRSAID() (string, error) {
	if mb.Type != MetaTypePurchase {
		return "", fmt.Errorf("mb type error")
	}
	ele, err := mb.GetEle(PurchaseProductRSAIDIndex)
	if err != nil {
		return "", err
	}
	if ele.Type != MetaEleTEXT {
		return "", fmt.Errorf("kid kind error")
	}
	pre, hash, err := xginx.DecodeAddressWithPrefix(ele.Body)
	if err != nil {
		return "", err
	}
	if pre != "rsa" {
		return "", fmt.Errorf("rsa pre error")
	}
	if hash.Equal(xginx.ZERO160) {
		return "", fmt.Errorf("rsa hash error")
	}
	return ele.Body, nil
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
	id := xginx.DocumentIDFromHex(ele.Body)
	if id.Type() != mb.Type {
		return xginx.NilDocumentID, fmt.Errorf("mb type !=id type")
	}
	return id, nil
}

//获取metabody的id
func (mb MetaBody) MustID() xginx.DocumentID {
	id, err := mb.ID()
	if err != nil {
		panic(err)
	}
	return id
}

//标签用,号隔开,使用这个特殊索引
const (
	ProductTagsEleIndex = 100
)

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
		return GetMetaBody(docdb, id)
	},
	Description: "从文档系统通过id获取数据",
}

func GetMetaBody(db xginx.IDocSystem, id xginx.DocumentID) (*MetaBody, error) {
	doc, err := db.Get(id)
	if err != nil {
		return nil, err
	}
	mb, err := ShopMeta(doc.Body).To()
	if err != nil {
		return nil, err
	}
	mb.TxID = doc.TxID
	mb.Index = doc.Index
	mb.Next = doc.Next
	mb.Prev = doc.Prev
	return mb, nil
}

//NewDocID 创建一个递增唯一的ID
func NewDocID() xginx.HASH160 {
	id := xginx.HASH160{}
	now := time.Now().UnixNano()
	binary.BigEndian.PutUint64(id[:], uint64(now))
	return id
}

var sellProduct = &graphql.Field{
	Name: "SellProduct",
	Type: graphql.NewNonNull(TXType),
	Args: graphql.FieldConfigArgument{
		"sender": {
			Type:        graphql.NewList(graphql.NewNonNull(SenderInput)),
			Description: "使用哪些金额作为产品保证金",
		},
		"receiver": {
			Type:        graphql.NewList(graphql.NewNonNull(ReceiverInput)),
			Description: "带metabody的产品信息的输出",
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
		//可一次发送多个
		for _, v := range receiver {
			if v.Meta == "" {
				return NewError(102, "meta args miss", err, v.Meta)
			}
			//上传产品时meta代表产品id,优先从docdb获取
			docid := xginx.DocumentIDFromHex(v.Meta)
			if docid.IsNil() {
				return NewError(103, "docid error")
			}
		}
		//交易费
		fee := p.Args["fee"].(int)
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
	Description: "出售产品到区块链,发布一个出售交易,接收为自己的可控制的地址",
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
