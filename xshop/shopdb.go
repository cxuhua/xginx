package main

import "github.com/cxuhua/xginx"

//金额数据库处理

const (
	MinerAddressKey = "__miner_address_key__"
)

type ShopDB struct {
	//文档处理db
	DocDB xginx.IDocSystem
	//密钥db
	KeyDB xginx.IKeysDB
}
