package xginx

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	ClientIn  = 1
	ClientOut = 2
)

type IClient interface {
	OnOpen()
	OnClose()
	OnRecvMsg(db DBImp, m MsgIO)
}

type Client struct {
	*NetStream
	typ    int
	ctx    context.Context
	cancel context.CancelFunc
	wc     chan MsgIO
	rc     chan MsgIO
	addr   NetAddr
	err    interface{}
	ss     *Server
	lis    IClient
	mVer   *MsgVersion //对方版本信息
	ping   int
}

//
func (c *Client) Equal(b *Client) bool {
	return c.NodeID().Equal(b.NodeID())
}

//服务器端处理包
func (c *Client) processMsgIn(db DBImp, m MsgIO) {

}

//是否是连入的
func (c *Client) IsIn() bool {
	return c.typ == ClientIn
}

//是否是连出的
func (c *Client) IsOut() bool {
	return c.typ == ClientOut
}

//获取对方节点id
func (c *Client) NodeID() UserID {
	if c.mVer == nil {
		return NewNodeID()
	} else {
		return c.mVer.NodeID
	}
}

//客户端处理包
func (c *Client) processMsgOut(db DBImp, m MsgIO) {

}

func (c *Client) processMsg(db DBImp, m MsgIO) error {
	if m.Type() == NT_PONG {
		msg := m.(*MsgPong)
		c.ping = msg.Ping()
	} else if m.Type() == NT_PING {
		msg := m.(*MsgPing)
		c.SendMsg(msg.NewPong())
	} else if m.Type() == NT_VERSION {
		osg := NewMsgVersion()
		msg := m.(*MsgVersion)
		//解码证书失败直接返回错误
		if err := conf.DecodeCerts(msg.Certs); err != nil {
			c.Close()
			return err
		}
		//保存连接地址和版本信息
		c.ss.addrs.Set(msg.Addr)
		c.mVer = msg
		//防止两节点重复连接，并且防止自己连接自己
		if c.ss.HasClient(msg.NodeID, c) {
			c.Close()
			return errors.New("has connection,closed")
		}
		//返回服务器版本信息
		if c.IsIn() {
			osg.Service = c.ss.Service()
			c.SendMsg(osg)
		}
	} else if c.IsIn() {
		c.processMsgIn(db, m)
	} else if c.IsOut() {
		c.processMsgOut(db, m)
	}
	if c.lis != nil {
		c.lis.OnRecvMsg(db, m)
	}
	return nil
}

func (c *Client) stop() {
	c.cancel()
	if err := recover(); err != nil {
		c.err = err
	}
	close(c.wc)
	close(c.rc)
	if c.Conn != nil {
		_ = c.Conn.Close()
	}
	if c.mVer != nil {
		c.ss.addrs.Del(c.mVer.Addr)
	}
	if c.lis != nil {
		c.lis.OnClose()
	}
	log.Println("client stop", c.addr, "error=", c.err)
}

//连接到指定地址
func (c *Client) Open(addr NetAddr) error {
	return c.connect(addr)
}

func (c *Client) connect(addr NetAddr) error {
	conn, err := net.DialTimeout("tcp", addr.Addr(), time.Second*30)
	if err != nil {
		return err
	}
	c.typ = ClientOut
	c.addr = addr
	c.NetStream = &NetStream{Conn: conn}
	if c.lis != nil {
		c.lis.OnOpen()
	}
	//发送第一个包
	c.SendMsg(NewMsgVersion())
	return nil
}

func (c *Client) Loop() {
	go c.loop()
}

func (c *Client) loop() {
	c.ss.wg.Add(1)
	defer c.ss.wg.Done()
	defer c.stop()
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
	ptimer := time.NewTimer(time.Second * time.Duration(Rand(40, 60)))
	vtimer := time.NewTimer(time.Second * 10) //10秒内不应答MsgVersion将关闭
	for {
		select {
		case wp := <-c.wc:
			err := c.WriteMsg(wp)
			if err != nil {
				panic(fmt.Errorf("write msg error %v", err))
			}
		case rp := <-c.rc:
			err := UseSession(c.ctx, func(db DBImp) error {
				return c.processMsg(db, rp)
			})
			if err != nil {
				log.Println("process msg", rp.Type(), "error", err)
			}
		case <-vtimer.C:
			if c.mVer == nil {
				c.Close()
				log.Println("msgversion timeout,closed")
			}
		case <-ptimer.C:
			if c.mVer == nil {
				break
			}
			c.SendMsg(NewMsgPing())
			ptimer.Reset(time.Second * time.Duration(Rand(40, 60)))
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
