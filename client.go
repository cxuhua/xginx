package xginx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ClientIn  = 1
	ClientOut = 2
)

type ClientMsg struct {
	c *Client
	m MsgIO
}

func NewClientMsg(c *Client, m MsgIO) *ClientMsg {
	return &ClientMsg{
		c: c,
		m: m,
	}
}

type Client struct {
	*NetStream
	typ     int
	ctx     context.Context
	cancel  context.CancelFunc
	wc      chan MsgIO
	rc      chan MsgIO
	Addr    NetAddr
	id      uint64
	err     interface{}
	ss      *TcpServer
	ping    int
	ptimer  *time.Timer
	vtimer  *time.Timer
	isopen  bool    //收到msgversion算打开成功
	Ver     uint32  //版本
	Service uint32  //服务
	Height  BHeight //节点区块高度
	bloom   *BloomFilter
}

//
func (c *Client) Equal(b *Client) bool {
	return c.id == b.id
}

//服务器端处理包
func (c *Client) processMsgIn(m MsgIO) {

}

//是否是连入的
func (c *Client) IsIn() bool {
	return c.typ == ClientIn
}

//是否是连出的
func (c *Client) IsOut() bool {
	return c.typ == ClientOut
}

//客户端处理包
func (c *Client) processMsgOut(m MsgIO) {

}

//请求对方区块头
func (c *Client) ReqBlockHeaders(bi *BlockIndex, hh uint32) {
	lh := bi.LastHeight()
	if lh == InvalidHeight && hh == InvalidHeight {
		return
	}
	//本地无区块头，请求远程的
	if lh == InvalidHeight && hh != InvalidHeight {
		rsg := bi.ReqMsgHeaders()
		c.SendMsg(rsg)
		return
	}
	//远程无区块头，发送本地的
	if lh != InvalidHeight && hh == InvalidHeight {
		rsg := bi.GetMsgHeadersUseHeight(hh)
		c.SendMsg(rsg)
		return
	}
	//本地比远程多,发送本地的过去
	if lh > hh {
		rsg := bi.GetMsgHeadersUseHeight(hh)
		c.SendMsg(rsg)
		return
	}
	//远程比本地多，请求远程的
	if hh > lh {
		rsg := bi.ReqMsgHeaders()
		c.SendMsg(rsg)
		return
	}
	//两边一样多
}

func (c *Client) processMsg(m MsgIO) error {
	ps := GetPubSub()
	bi := GetBlockIndex()
	typ := m.Type()
	switch typ {
	case NT_ALERT:
		msg := m.(*MsgAlert)
		//继续广播出去
		c.ss.BroadMsg(msg, c)
		LogInfo("recv alert message:", msg.Msg.String())
	case NT_ERROR:
		msg := m.(*MsgError)
		LogError("recv error msg code =", msg.Code, "error =", msg.Error, c.id)
	case NT_HEADERS:
		msg := m.(*MsgHeaders)
		c.Height = msg.Height
		ps.Pub(msg, NetMsgHeadersTopic)
	case NT_GET_HEADERS:
		msg := m.(*MsgGetHeaders)
		rsg := bi.GetMsgHeaders(msg)
		c.SendMsg(rsg)
	case NT_GET_INV:
		msg := m.(*MsgGetInv)
		if len(msg.Invs) == 0 {
			break
		}
		bi.GetMsgGetInv(msg, c)
	case NT_BLOCK:
		msg := m.(*MsgBlock)
		ps.Pub(msg.Blk, NetMsgBlockTopic)
	case NT_TX:
		msg := m.(*MsgTx)
		ps.Pub(msg.Tx, NetMsgTxTopic)
	case NT_ADDRS:
		msg := m.(*MsgAddrs)
		ps.Pub(msg, NetMsgAddrsTopic)
	case NT_GET_ADDRS:
		msg := c.ss.NewMsgAddrs(c)
		c.SendMsg(msg)
	case NT_PONG:
		msg := m.(*MsgPong)
		c.Height = msg.Height
		c.ping = msg.Ping()
	case NT_PING:
		msg := m.(*MsgPing)
		c.Height = msg.Height
		rsg := msg.NewPong(bi.GetNodeHeight())
		c.SendMsg(rsg)
	case NT_VERSION:
		msg := m.(*MsgVersion)
		//保存到地址列表
		if msg.Addr.IsGlobalUnicast() {
			c.ss.addrs.Set(msg.Addr)
		}
		//防止两节点重复连接，并且防止自己连接自己
		if c.ss.HasClient(msg.NodeID, c) {
			c.Close()
			return errors.New("has connection,closed")
		}
		c.Addr = msg.Addr
		c.Height = msg.Height
		//保存节点信息
		c.Ver = msg.Ver
		c.Service = msg.Service
		//如果是连入的，返回节点版本信息
		if c.IsIn() {
			rsg := bi.NewMsgVersion()
			rsg.Service = c.ss.Service()
			c.SendMsg(rsg)
		}
		//连出的判断区块高度
		if c.IsOut() {
			c.ReqBlockHeaders(bi, msg.Height.HH)
		}
	default:
		if c.IsIn() {
			c.processMsgIn(m)
		} else if c.IsOut() {
			c.processMsgOut(m)
		}
	}
	//发布消息
	ps.Pub(NewClientMsg(c, m), NetMsgTopic)
	return nil
}

func (c *Client) recoverError() {
	if gin.Mode() == gin.DebugMode {
		c.cancel()
		return
	}
	if err := recover(); err != nil {
		c.err = err
		c.cancel()
	}
}

func (c *Client) stop() {
	defer c.recoverError()
	c.isopen = false
	//更新关闭时间
	if ap := c.ss.addrs.Get(c.Addr); ap != nil {
		ap.closeTime = time.Now()
	}
	if c.ss != nil {
		c.ss.DelClient(c.id, c)
	}
	close(c.wc)
	close(c.rc)
	if c.Conn != nil {
		_ = c.Conn.Close()
	}
	LogInfo("client stop", c.Addr, "error=", c.err)
}

//连接到指定地址
func (c *Client) Open(addr NetAddr) error {
	if addr.Equal(conf.GetNetAddr()) {
		return errors.New("self connect self,ignore")
	}
	if c.ss.HasAddr(addr) {
		return errors.New("addr has client connected,ignore!")
	}
	return c.connect(addr)
}

func (c *Client) connect(addr NetAddr) error {
	if !addr.IsGlobalUnicast() {
		return errors.New("addr not is global unicast,can't connect")
	}
	//更新最后链接时间
	if ap := c.ss.addrs.Get(addr); ap != nil {
		ap.lastTime = time.Now()
	}
	conn, err := net.DialTimeout(addr.Network(), addr.Addr(), time.Second*15)
	if err != nil {
		return err
	}
	//更新链接时间
	if ap := c.ss.addrs.Get(addr); ap != nil {
		ap.openTime = time.Now()
	}
	c.isopen = true
	LogInfo("connect to", addr, "success")
	c.typ = ClientOut
	c.Addr = addr
	c.NetStream = NewNetStream(conn)
	//发送第一个包
	bi := GetBlockIndex()
	c.SendMsg(bi.NewMsgVersion())
	return nil
}

func (c *Client) Loop() {
	go c.loop()
}

func (c *Client) loop() {
	defer c.stop()

	c.ss.wg.Add(1)
	defer c.ss.wg.Done()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				c.err = err
				c.cancel()
			}
		}()
		for {
			m, err := c.ReadMsg()
			if err != nil {
				panic(fmt.Errorf("read msg error %w", err))
			}
			c.rc <- m
		}
	}()
	for {
		select {
		case wp := <-c.wc:
			err := c.WriteMsg(wp)
			if err != nil {
				panic(fmt.Errorf("write msg error %v", err))
			}
		case rp := <-c.rc:
			err := c.processMsg(rp)
			if err != nil {
				LogError("process msg", rp.Type(), "error", err)
			}
		case <-c.vtimer.C:
			if !c.isopen {
				c.Close()
				LogError("msgversion timeout,closed")
			}
		case <-c.ptimer.C:
			if !c.isopen {
				break
			}
			bi := GetBlockIndex()
			msg := NewMsgPing(bi.GetNodeHeight())
			c.SendMsg(msg)
			c.ptimer.Reset(time.Second * time.Duration(Rand(40, 60)))
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) SendMsg(m MsgIO) {
	c.wc <- m
}

func (c *Client) Close() {
	c.cancel()
}
