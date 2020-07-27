package main

import "github.com/cxuhua/xginx"

//金额数据库处理

type ShopDB struct {
	//主链对象
	bi *xginx.BlockIndex
	//文档处理db
	DocDB xginx.IDocSystem
	//密钥db
	KeyDB xginx.IKeysDB
}

func (sdb *ShopDB) GetMinerAddr() xginx.Address {
	return ""
}
