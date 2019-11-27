package xginx

import "github.com/gin-gonic/gin"

//所有回调可能来自不同的协程
type IListener interface {
	//当区块头被成功链接时 处于blockindex锁中
	OnUpdateHeader(bi *BlockIndex, ele *TBEle)
	//更新区块数据成功时  处于blockindex锁中
	OnUpdateBlock(bi *BlockIndex, blk *BlockInfo)
	//当块创建时，可以添加，修改块内信息
	OnNewBlock(bi *BlockIndex, blk *BlockInfo) error
	//完成区块，当检测完成调用,设置merkle之前
	OnFinished(bi *BlockIndex, blk *BlockInfo) error
	//当收到网络数据时
	OnClientMsg(c *Client, msg MsgIO)
	//链关闭时
	OnClose(bi *BlockIndex)
	//获取当前设置的钱包
	GetWallet() IWallet
	//当服务启动后会调用一次
	OnStartup()
	//初始化http服务器后
	OnInitHttp(m *gin.Engine)
	//当产生了新交易
	OnNewTx(bi *BlockIndex, tx *TX) error
}
