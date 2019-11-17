package xginx

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net"
	"sync"
	"time"
)

const (
	//收到地址消息订阅
	NetMsgAddrsTopic = "NetMsgAddrs"
)

type IServer interface {
	Start(ctx context.Context)
	Stop()
	Wait()
}

var (
	Server IServer = &server{}
)

type server struct {
	ser *TcpServer
}

func (s *server) Wait() {
	s.ser.Wait()
}

func (s *server) Stop() {
	s.ser.Stop()
}

func (s *server) Start(ctx context.Context) {
	ser, err := NewTcpServer(ctx, conf)
	if err != nil {
		panic(err)
	}
	s.ser = ser
	s.ser.Run()
}

type TcpServer struct {
	service uint32
	lis     net.Listener
	addr    *net.TCPAddr
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
	err     interface{}
	wg      sync.WaitGroup
	clients map[string]*Client //连接的所有client
}

func (s *TcpServer) NewMsgAddrs(c *Client) *MsgAddrs {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := &MsgAddrs{}
	for _, v := range s.clients {
		//不包括自己
		if v.addr.Equal(c.addr) {
			continue
		}
		if msg.Add(v.addr) {
			break
		}
	}
	return msg
}

//如果c不空不会广播给c
func (s *TcpServer) BroadMsg(c *Client, m MsgIO) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.clients {
		if c != nil && v.Equal(c) {
			continue
		}
		v.SendMsg(m)
	}
}

func (s *TcpServer) HasAddr(addr NetAddr) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.clients {
		if v.addr.Equal(addr) {
			return true
		}
	}
	return false
}

func (s *TcpServer) HasClient(addr NetAddr, c *Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := addr.String()
	_, ok := s.clients[key]
	if !ok {
		c.addr = addr
		s.clients[key] = c
	}
	return ok
}

func (s *TcpServer) DelClient(addr NetAddr, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, addr.String())
}

func (s *TcpServer) AddClient(addr NetAddr, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[addr.String()] = c
}

func (s *TcpServer) Stop() {
	s.cancel()
}

func (s *TcpServer) Run() {
	go s.run()
}

func (s *TcpServer) Wait() {
	s.wg.Wait()
}

func (s *TcpServer) NewClient() *Client {
	c := &Client{ss: s}
	c.ctx, c.cancel = context.WithCancel(s.ctx)
	c.wc = make(chan MsgIO, 4)
	c.rc = make(chan MsgIO, 4)
	c.ptimer = time.NewTimer(time.Second * time.Duration(Rand(40, 60)))
	c.vtimer = time.NewTimer(time.Second * 10) //10秒内不应答MsgVersion将关闭
	return c
}

func (s *TcpServer) ConnNum() int {
	s.mu.RLock()
	defer s.mu.Unlock()
	return len(s.clients)
}

func (s *TcpServer) Service() uint32 {
	return s.service
}

//收到地址列表，如果没有达到最大链接开始链接
func (s *TcpServer) recvMsgAddrs(msg *MsgAddrs) {
	if cl := s.ConnNum(); cl >= conf.MaxConn {
		LogInfof("max conn=%d ,pause connect client", cl)
		return
	}
	for _, addr := range msg.Addrs {
		if !addr.IsGlobalUnicast() {
			continue
		}
		if s.HasAddr(addr) {
			continue
		}
		c := s.NewClient()
		err := c.Open(addr)
		if err != nil {
			LogError("connect", addr, "error", err)
			continue
		}
		c.Loop()
	}
}

func (s *TcpServer) run() {
	LogInfo(s.addr.Network(), "server startup", s.addr)
	defer func() {
		if err := recover(); err != nil {
			s.err = err
			s.cancel()
		}
	}()
	go func() {
		//订阅收到地址列表
		ch := GetPubSub().Sub(NetMsgAddrsTopic)
		for {
			select {
			case <-s.ctx.Done():
				_ = s.lis.Close()
				return
			case cv := <-ch:
				if msg, ok := cv.(*MsgAddrs); ok {
					s.recvMsgAddrs(msg)
				}

			}
		}
	}()
	for {
		conn, err := s.lis.Accept()
		if err != nil {
			panic(err)
		}
		c := s.NewClient()
		c.NetStream = NewNetStream(conn)
		err = c.addr.From(conn.RemoteAddr().String())
		if err != nil {
			_ = conn.Close()
			continue
		}
		c.typ = ClientIn
		LogInfo("new connection", conn.RemoteAddr())
		c.Loop()
	}
}

//生成一个临时id全网唯一
func NewNodeID(c *Config) HASH160 {
	id := HASH160{}
	_ = binary.Read(rand.Reader, Endian, id[:])
	w := NewWriter()
	_, _ = w.Write(id[:])
	_, _ = w.Write([]byte(c.TcprIp))
	_ = w.TWrite(time.Now().UnixNano())
	copy(id[:], Hash160(w.Bytes()))
	return id
}

func NewTcpServer(ctx context.Context, c *Config) (*TcpServer, error) {
	s := &TcpServer{}
	s.addr = c.GetTcpListenAddr().ToTcpAddr()
	lis, err := net.ListenTCP(s.addr.Network(), s.addr)
	if err != nil {
		return nil, err
	}
	s.lis = lis
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.clients = map[string]*Client{}
	s.service = SERVICE_NODE
	return s, nil
}
