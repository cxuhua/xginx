package xginx

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"
)

func TestPostSignData(t *testing.T) {
	surl := "http://192.168.31.177:9334/sign/OO01000000507C5C45B6D4CB17CADEC3AE1EDE36775302EF0330C5295F714ACF8089047D1432AA6180000010C930ABEAFC1D8553"
	otag := NewTagInfo(surl)
	//客户端服务器端都要解码
	if err := otag.DecodeURL(); err != nil {
		panic(err)
	}
	sigb, err := otag.ToSigBytes()
	if err != nil {
		panic(err)
	}
	//模拟客户端签名
	pk, err := LoadPrivateKey(cpkey)
	if err != nil {
		panic(err)
	}
	client := &CliPart{}
	client.CLoc.Set(122.33, 112.44)
	client.Prev = Hash256{}
	client.CTime = time.Now().UnixNano()
	cb, err := client.Sign(pk, sigb)
	if err != nil {
		panic(err)
	}
	res, err := http.Post(surl, "application/octet-stream", bytes.NewReader(cb))
	if err != nil {
		panic(err)
	}
	log.Println(res.StatusCode)
	data, err := ioutil.ReadAll(res.Body)
	log.Println(string(data), err)
}
