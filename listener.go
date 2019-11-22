package xginx

import "github.com/gin-gonic/gin"

type IListener interface {
	//当块创建时，可以添加，修改块内信息
	OnNewBlock(bi *BlockIndex, blk *BlockInfo) error
	//完成区块，当检测完成调用,设置merkle之前
	OnFinished(bi *BlockIndex, blk *BlockInfo) error
	//获取签名账户
	GetAccount(bi *BlockIndex, pkh HASH160) (*Account, error)
	//链关闭时
	OnClose(bi *BlockIndex)
	//获取当前设置的钱包
	GetWallet() IWallet
	//当服务启动
	OnStartup()
	//初始化http服务器
	OnInitHttp(m *gin.Engine)
}
