package xginx

import (
	"context"
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

//服务器端处理包
func (c *Client) processMsgIn(db DBImp, m MsgIO) {

}

func (c *Client) IsIn() bool {
	return c.typ == ClientIn
}

func (c *Client) IsOut() bool {
	return c.typ == ClientOut
}

//客户端处理包
func (c *Client) processMsgOut(db DBImp, m MsgIO) {

}

func (c *Client) processMsg(db DBImp, m MsgIO) error {
	if m.Type() == NT_PONG {
		msg := m.(*MsgPong)
		//网络延迟（毫秒)
		c.ping = msg.Ping()
		log.Println("PING", c.ping)
	} else if m.Type() == NT_PING {
		msg := m.(*MsgPing)
		c.SendMsg(msg.NewPong())
	} else if m.Type() == NT_VERSION {
		wm := NewMsgVersion()
		msg := m.(*MsgVersion)
		//解码证书失败直接返回错误
		if err := conf.DecodeCerts(msg.Certs); err != nil {
			c.stop()
			return err
		}
		//返回服务器版本信息
		if c.IsIn() {
			wm.Service = c.ss.Service()
			c.SendMsg(wm)
		}
		//保存连接地址
		c.ss.addrs.Set(msg.Addr)
		c.mVer = msg
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
	c.ss.DelClient(c)
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
	defer c.stop()
	c.ss.AddClient(c)
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
				panic(fmt.Errorf("read msg error %v", err))
			}
			c.rc <- m
		}
	}()
	ptimer := time.NewTimer(time.Second * time.Duration(Rand(40, 60)))
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

func NewClient(ctx context.Context) *Client {
	c := &Client{}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.wc = make(chan MsgIO, 32)
	c.rc = make(chan MsgIO, 32)
	return c
}
