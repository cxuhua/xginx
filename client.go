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
}

func (c *Client) processMsg(m MsgIO) {
	log.Println("Get MESSAGE:", m.Type())
}

func (c *Client) stop() {
	if err := recover(); err != nil {
		c.err = err
	}
	close(c.wc)
	close(c.rc)
	if c.Conn != nil {
		_ = c.Conn.Close()
	}
	if c.ss != nil {
		c.ss.wg.Done()
	}
	log.Println("client stop", c.addr)
}

func (c *Client) connect(addr NetAddr) error {
	conn, err := net.DialTimeout("tcp", addr.Addr(), time.Second*30)
	if err != nil {
		return err
	}
	c.typ = ClientOut
	c.addr = addr
	c.NetStream = &NetStream{Conn: conn}
	return nil
}

func (c *Client) loop() {
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
				panic(fmt.Errorf("read msg error %v", err))
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
			c.processMsg(rp)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) Close() {
	c.cancel()
}

func NewClient(ctx context.Context) *Client {
	c := &Client{}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.wc = make(chan MsgIO, 10)
	c.rc = make(chan MsgIO, 10)
	return c
}
