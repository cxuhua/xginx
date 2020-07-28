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

	"github.com/cxuhua/xginx"
)

//MetaHash hash方法
func MetaHash(s string) string {
	return xginx.Hash160From([]byte(s)).String()
}

func MetaHashBytes(bb []byte) string {
	return xginx.Hash160From(bb).String()
}

const (
	//文本元素 sum为文本内容的sha256
	MetaEleTEXT = "TEXT"
	//url链接元素 sum为链接内容的sum
	MetaEleURL = "URL"
	//HASH公钥,用于合成地址信息
	MetaEleHASH = "HASH"
	//RSA公钥,用于信息加密
	MetaEleRSA = "RSA"
	//url资源对应的最大大小
	MaxURLSize = 1024 * 1024 * 5
)

//meta元素
type MetaEle struct {
	//元素类型 TEXT URL
	Type string `json:"type"`
	//内容长度
	Size int `json:"size"`
	//校验和 sha256算法 HASH和RSA公钥不需要校验
	Sum string `json:"sum,omitempty"`
	//对应的text内容或者是url 例如:http://www.baidu.com/logo.png
	Body string `json:"body"`
}

func (ele MetaEle) checkurl(ctx context.Context, urlv *url.URL) error {
	scheme := strings.ToLower(urlv.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only support http or https")
	}
	res, err := http.Get(urlv.String())
	if err != nil {
		return err
	}
	defer res.Body.Close()
	cls := res.Header.Get("Content-Length")
	if cl, err := strconv.ParseInt(cls, 10, 32); err != nil {
		return err
	} else if cl > MaxURLSize {
		return fmt.Errorf("url resource too big < %d", MaxURLSize)
	} else if cl != int64(ele.Size) {
		return fmt.Errorf("url resource size err %d != %d", cl, ele.Size)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if len(body) > MaxURLSize {
		return fmt.Errorf("url resource too big < %d", MaxURLSize)
	}
	if len(body) != ele.Size {
		return fmt.Errorf("url resource size err %d != %d", len(body), ele.Size)
	}
	if MetaHashBytes(body) != ele.Sum {
		return fmt.Errorf("url resource sum error")
	}
	return nil
}

func (ele MetaEle) Check(ctx context.Context) error {
	if ele.Type == MetaEleTEXT {
		if len(ele.Body) != ele.Size {
			return fmt.Errorf("ele size error")
		}
		if MetaHash(ele.Body) != ele.Sum {
			return fmt.Errorf("ele hash sum error")
		}
		return nil
	}
	if ele.Type == MetaEleHASH {
		if len(ele.Body) != ele.Size {
			return fmt.Errorf("ele size error")
		}
		if ele.Size != len(xginx.ZERO256)*2 {
			return fmt.Errorf("hash length error")
		}
		_, err := hex.DecodeString(ele.Body)
		if err != nil {
			return err
		}
		return nil
	}
	if ele.Type == MetaEleRSA {
		if len(ele.Body) != ele.Size {
			return fmt.Errorf("ele size error")
		}
		_, err := xginx.LoadRSAPublicKey(ele.Body)
		if err != nil {
			return err
		}
		return nil
	}
	if ele.Type == MetaEleURL {
		urlv, err := url.Parse(ele.Body)
		if err != nil {
			return err
		}
		err = ele.checkurl(ctx, urlv)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("type %s error", ele.Type)
}

const (
	//出售
	MetaTypeSell = 1
	//购买
	MetaTypeBuy = 2
	//确认
	MetaTypeConfirm = 3
)

//txout输出meta,meta末尾为meta元素的sha256校验和(64字节,hex格式编码)
type MetaBody struct {
	Type int       `json:"type"` //1-出售 2-购买 3-确认
	Tags []string  `json:"tags"` //内容关键字,用于商品关注过滤存储
	Eles []MetaEle `json:"eles"` //元素集合
}

func (mb MetaBody) To() (ShopMeta, error) {
	jv, err := json.Marshal(mb)
	if err != nil {
		return "", err
	}
	hv := xginx.Hash160From(jv)
	str := ShopMeta(string(jv) + hv.String())
	if len(str) > xginx.MaxMetaSize {
		return "", fmt.Errorf("content length > %d", xginx.MaxMetaSize)
	}
	return str, nil
}

type ShopMeta string

func (s ShopMeta) To() (*MetaBody, error) {
	if len(s) > xginx.MaxMetaSize {
		return nil, fmt.Errorf("content length > %d", xginx.MaxMetaSize)
	}
	sl := len(xginx.ZERO160) * 2
	if len(s) < sl+16 {
		return nil, fmt.Errorf("meta length error")
	}
	bb := string(s[:len(s)-sl])
	if MetaHash(bb) != string(s[len(s)-sl:]) {
		return nil, fmt.Errorf("hash sum error")
	}
	mb := &MetaBody{}
	err := json.Unmarshal([]byte(bb), mb)
	if err != nil {
		return nil, err
	}
	return mb, nil
}

func NewShopMeta(ctx context.Context, mb MetaBody) (ShopMeta, error) {
	if mb.Type != MetaTypeSell && mb.Type != MetaTypeBuy && mb.Type != MetaTypeConfirm {
		return "", fmt.Errorf("type %d error", mb.Type)
	}
	for _, ele := range mb.Eles {
		if err := ele.Check(ctx); err != nil {
			return "", err
		}
	}
	return mb.To()
}
