package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cxuhua/xginx"
)

//当购买一件物品时,将购买交易上传到物品指定的协议地址,买方可以从此交易信息
//接受买家后签名发布交易,交易区块链确认后发出物品
//买家收到物品后确认发起一个确认交易,分配购买时转入公共账号中的金额,签名再次发送到此交易地址,买方再次确认发送到区块链完成交易
//这里负责存储和验证交易信息,按出价排序存储
type ITxSwap interface {
	//拉取rsa相关的交易(购买,确认)
	GET(rsa string) []*xginx.TX
	//添加rsa交易信息
	PUT(rsa string, body []byte) error
	//删除相关的交易
	DELETE(rsa string) error
}

//接收和验证购买交易,根据rsa密钥id分类存储
type httptxswap struct {
	objs Objects
}

func (tr *httptxswap) getRSAId(req *http.Request) (string, error) {
	q := req.URL.Query()
	rsa := q.Get("rsa")
	if rsa == "" {
		return "", fmt.Errorf("rsa miss")
	}
	pre, _, err := xginx.DecodeAddressWithPrefix(rsa)
	if err != nil {
		return "", err
	}
	if pre != "rsa" {
		return "", fmt.Errorf("rsa error")
	}
	return rsa, nil
}

func (tr *httptxswap) writeError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	_, _ = fmt.Fprintf(w, "error = %v", err)
}

func (tr *httptxswap) getBody(req *http.Request) ([]byte, error) {
	defer req.Body.Close()
	return ioutil.ReadAll(req.Body)
}

//删除rsa信息 成功返回 200
func (tr *httptxswap) deleteRSA(w http.ResponseWriter, rsa string) {

}

//获取和rsa相关的交易信息 成功返回 200
func (tr *httptxswap) listRSATxs(w http.ResponseWriter, rsa string) {

}

//保存和rsa相关的交易信息 成功返回 200
func (tr *httptxswap) putRSATx(w http.ResponseWriter, rsa string, body []byte) {

}

//http://127.0.0.1:9334/swap/tx?rsa=rsaxxxx
func (tr *httptxswap) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rsa, err := tr.getRSAId(req)
	if err != nil {
		tr.writeError(w, err)
	} else if req.Method == http.MethodDelete {
		tr.deleteRSA(w, rsa)
	} else if req.Method == http.MethodPut {
		body, err := tr.getBody(req)
		if err != nil {
			tr.writeError(w, err)
			return
		}
		tr.putRSATx(w, rsa, body)
	} else if req.Method == http.MethodGet {
		tr.listRSATxs(w, rsa)
	}
}
