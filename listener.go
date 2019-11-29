package xginx

import "github.com/gin-gonic/gin"

//所有回调可能来自不同的协程
type IListener interface {
	//获取当前配置
	GetConfig() *Config
	//设置当前使用的链
	SetBlockIndex(bi *BlockIndex)
	//当一个区块断开后
	OnUnlinkBlock(blk *BlockInfo)
	//当区块头被成功链接时 处于blockindex锁中
	OnUpdateHeader(ele *TBEle)
	//更新区块数据成功时  处于blockindex锁中
	OnUpdateBlock(blk *BlockInfo)
	//当块创建时，可以添加，修改块内信息
	OnNewBlock(blk *BlockInfo) error
	//完成区块，当检测完成调用,设置merkle之前
	OnFinished(blk *BlockInfo) error
	//当收到网络数据时
	OnClientMsg(c *Client, msg MsgIO)
	//链关闭时
	OnClose()
	//获取当前设置的钱包
	GetWallet() IWallet
	//当服务启动后会调用一次
	OnStartup()
	//初始化http服务器后
	OnInitHttp(m *gin.Engine)
	//当交易进入交易池之前
	OnTxPool(tx *TX) error
}
