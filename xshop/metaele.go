package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cxuhua/xginx"
)

const (
	//购买交易分类,当购买一个产品生成的交易临时存储在这里
	TypeDBTypeTTPTX = "TPTX"
)

//MetaHash hash方法
func MetaHash(s string) string {
	return MetaHashBytes([]byte(s))
}

func MetaHashBytes(dat []byte) string {
	bb := xginx.Ripemd160(dat)
	return hex.EncodeToString(bb)
}

const (
	//唯一的UUID <= 64 byte
	MetaEleUUID = "UUID"
	//文本元素 sum为文本内容的sha256
	MetaEleTEXT = "TEXT"
	//url链接元素 sum为链接内容的sum
	MetaEleURL = "URL"
	//HASH公钥,用于合成地址信息
	MetaEleHASH = "HASH"
	//RSA公钥,用于信息加密
	MetaEleRSA = "RSA"
	//KID 私钥ID = 公钥hash swit编码 可解码出公钥hash256用于生成地址
	MetaEleKID = "KID"
	//url资源对应的最大大小
	MaxURLSize = 1024 * 1024 * 5
)

type MetaType int8

const (
	MetaTypeSell     MetaType = 1 //出售
	MetaTypePurchase MetaType = 2 //购买
	MetaTypeConfirm  MetaType = 3 //确认
)

//数据扩展信息 保存执行区块交易的信息
type MetaExt struct {
	TxID  xginx.HASH256 //交易ID
	Index xginx.VarUInt //输出索引
}

//获取对应的输出
func (ext *MetaExt) GetTxOut(bi *xginx.BlockIndex) (*xginx.TxOut, error) {
	tx, err := bi.LoadTX(ext.TxID)
	if err != nil {
		return nil, err
	}
	if idx := ext.Index.ToInt(); idx >= len(tx.Outs) {
		return nil, fmt.Errorf("index out bound")
	} else {
		return tx.Outs[idx], nil
	}
}

func (ext *MetaExt) Decode(bb []byte) error {
	r := xginx.NewReader(bb)
	err := ext.TxID.Decode(r)
	if err != nil {
		return err
	}
	err = ext.Index.Decode(r)
	if err != nil {
		return err
	}
	return nil
}

func (ext MetaExt) Encode() ([]byte, error) {
	w := xginx.NewWriter()
	err := ext.TxID.Encode(w)
	if err != nil {
		return nil, err
	}
	err = ext.Index.Encode(w)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

//GetDocumentExt 获取文档扩展信息
func GetDocumentExt(docdb xginx.IDocSystem, id xginx.DocumentID) (*MetaExt, error) {
	bb, err := docdb.GetExt(id)
	if err != nil {
		return nil, err
	}
	ext := &MetaExt{}
	err = ext.Decode(bb)
	if err != nil {
		return nil, err
	}
	return ext, nil
}

//从文档系统获取扩展信息
func (mb *MetaBody) GetSellExt(docdb xginx.IDocSystem) (*MetaExt, error) {
	id := mb.MustID()
	return GetDocumentExt(docdb, id)
}

//txout输出meta,meta末尾为meta元素的sha256校验和(64字节,hex格式编码)
type MetaBody struct {
	Type MetaType  `json:"type"`           //1-出售 2-购买 3-确认
	Tags []string  `json:"tags,omitempty"` //内容关键字,购买meta不存在
	Eles []MetaEle `json:"eles"`           //元素集合
	Ext  *MetaExt  `json:"-"`              //扩展信息
}

func (mb *MetaBody) ToDocument() (*xginx.Document, error) {
	doc := xginx.NewDocument()
	doc.ID = mb.MustID()
	str, err := mb.To()
	if err != nil {
		return nil, err
	}
	doc.Body = []byte(str)
	doc.Tags = mb.Tags
	return doc, nil
}

func ParseMetaBody(b []byte) (*MetaBody, error) {
	return ShopMeta(b).To()
}

//meta元素
type MetaEle struct {
	//元素类型 TEXT URL
	Type string `json:"type"`
	//对应的text内容或者是url 例如:http://www.baidu.com/logo.png
	Body string `json:"body"`
}

//NewMetaEle 创建一个元素类型
func NewMetaEle(typ string, body string) MetaEle {
	me := MetaEle{
		Type: typ,
		Body: body,
	}
	return me
}

//NewMetaUrl 下载url资源生成描述
func NewMetaUrl(ctx context.Context, surl string) (MetaEle, error) {
	me := MetaEle{
		Type: MetaEleURL,
	}
	urlv, err := url.Parse(surl)
	if err != nil {
		return me, err
	}
	//获取文件头
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, surl, nil)
	if err != nil {
		return me, err
	}
	res, err := httpclient.Do(req)
	if err != nil {
		return me, err
	}
	//资源的content-length必须存在
	cls := res.Header.Get("Content-Length")
	if cls == "" {
		return me, fmt.Errorf("miss http Content-Length header")
	} else if cl, err := strconv.ParseInt(cls, 10, 32); err != nil {
		return me, err
	} else if cl > MaxURLSize {
		return me, fmt.Errorf("url resource %d too big > %d", cl, MaxURLSize)
	}
	//下载数据
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, surl, nil)
	if err != nil {
		return me, err
	}
	res, err = httpclient.Do(req)
	if err != nil {
		return me, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return me, err
	}
	q := urlv.Query()
	q.Set("sum", MetaHashBytes(body))
	urlv.RawQuery = q.Encode()
	//追加sum用来检测内容是否一致
	me.Body = urlv.String()
	return me, nil
}

var (
	httpclient = http.Client{
		Timeout: time.Second * 30,
	}
)

//下载数据检测
func (ele MetaEle) checkurl(ctx context.Context) error {
	urlv, err := url.Parse(ele.Body)
	if err != nil {
		return err
	}
	q := urlv.Query()
	if q.Get("sum") == "" {
		return fmt.Errorf("url %s miss sum query args", ele.Body)
	}
	scheme := strings.ToLower(urlv.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only support http or https")
	}
	elev, err := NewMetaUrl(ctx, urlv.String())
	if err != nil {
		return err
	}
	if elev.Body != ele.Body {
		return fmt.Errorf("body not match")
	}
	return nil
}

func (ele MetaEle) Check(ctx context.Context) error {
	if ele.Type == MetaEleKID {
		_, err := xginx.DecodePublicHash(ele.Body)
		return err
	}
	if ele.Type == MetaEleUUID {
		if len(ele.Body) == 0 {
			return fmt.Errorf("ele size error")
		}
		return nil
	}
	if ele.Type == MetaEleTEXT {
		if len(ele.Body) == 0 {
			return fmt.Errorf("ele size error")
		}
		return nil
	}
	if ele.Type == MetaEleHASH {
		if len(ele.Body) != len(xginx.ZERO256)*2 {
			return fmt.Errorf("hash length error")
		}
		_, err := hex.DecodeString(ele.Body)
		if err != nil {
			return err
		}
		return nil
	}
	if ele.Type == MetaEleRSA {
		_, err := xginx.LoadRSAPublicKey(ele.Body)
		if err != nil {
			return err
		}
		return nil
	}
	if ele.Type == MetaEleURL {
		return ele.checkurl(ctx)
	}
	return fmt.Errorf("type %s error", ele.Type)
}

func (mb *MetaBody) Check(ctx context.Context) error {
	if len(mb.Eles) == 0 {
		return fmt.Errorf("eles empty")
	}
	for _, ele := range mb.Eles {
		err := ele.Check(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (mb *MetaBody) To() (ShopMeta, error) {
	//sum不需要签名
	jv, err := json.Marshal(mb)
	if err != nil {
		return "", err
	}
	hv := MetaHashBytes(jv)
	str := ShopMeta(string(jv) + hv)
	if len(str) > xginx.MaxMetaSize {
		return "", fmt.Errorf("content length > %d", xginx.MaxMetaSize)
	}
	return str, nil
}

type ShopMeta string

func (s ShopMeta) To() (*MetaBody, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("meta empty")
	}
	if len(s) > xginx.MaxMetaSize {
		return nil, fmt.Errorf("content length > %d", xginx.MaxMetaSize)
	}
	sl := len(xginx.ZERO160) * 2
	if len(s) < sl {
		return nil, fmt.Errorf("meta length error")
	}
	mb := &MetaBody{}
	bb := string(s[:len(s)-sl])
	sb := string(s[len(s)-sl:])
	if MetaHash(bb) != sb {
		return nil, fmt.Errorf("check hash error")
	}
	err := json.Unmarshal([]byte(bb), mb)
	if err != nil {
		return nil, err
	}
	return mb, nil
}

func NewShopMeta(ctx context.Context, mb *MetaBody) (ShopMeta, error) {
	if mb.Type != MetaTypeSell && mb.Type != MetaTypePurchase && mb.Type != MetaTypeConfirm {
		return "", fmt.Errorf("type %d error", mb.Type)
	}
	for _, ele := range mb.Eles {
		if err := ele.Check(ctx); err != nil {
			return "", err
		}
	}
	return mb.To()
}
