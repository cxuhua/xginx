package xginx

//所有回调可能来自不同的协程
type IListener interface {
	//是否在测试环境
	IsTest() bool
	//时间戳发生器
	TimeNow() uint32
	//获取当前配置
	GetConfig() *Config
	//设置当前使用的链
	SetBlockIndex(bi *BlockIndex)
	//当一个区块断开后
	OnUnlinkBlock(blk *BlockInfo)
	//更新区块数据成功时
	OnLinkBlock(blk *BlockInfo)
	//当块创建时，可以添加，修改块内信息
	OnNewBlock(blk *BlockInfo) error
	//完成区块，当检测完成调用,设置merkle之前
	OnFinished(blk *BlockInfo) error
	//当收到网络数据时,数据包根据类型转换成需要的包
	OnClientMsg(c *Client, msg MsgIO)
	//链关闭时
	OnClose()
	//当服务启动后会调用一次
	OnStartup()
	//当交易进入交易池之前，返回错误不会进入交易池
	OnTxPool(tx *TX) error
	//当账户没有私钥签名时调用此方法
	OnSignTx(singer ISigner) error
	//当交易池的交易因为seq设置被替换时
	OnTxRep(old *TX, new *TX)
}
