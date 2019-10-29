package xginx

import (
	"context"
	"log"
	"net"
	"sync"
)

type NetAddrMap struct {
	addrs map[string]NetAddr
	mu    sync.Mutex
}

func (m *NetAddrMap) Set(addr NetAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addrs[addr.String()] = addr
}

func (m *NetAddrMap) NewMsgAddrs() *MsgAddrs {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := &MsgAddrs{}
	for _, v := range m.addrs {
		msg.Add(v)
	}
	return msg
}

func (m *NetAddrMap) Del(addr NetAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.addrs, addr.String())
}

func NewNetAddrMap() *NetAddrMap {
	return &NetAddrMap{
		addrs: map[string]NetAddr{},
		mu:    sync.Mutex{},
	}
}

type Server struct {
	service uint32
	lis     net.Listener
	addr    *net.TCPAddr
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
	err     interface{}
	wg      sync.WaitGroup
	clients map[string]*Client //连接的所有client
	addrs   *NetAddrMap        //连接我的ip地址和我连接出去的
}

//广播消息
func (s *Server) BroadMsg(m MsgIO) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.clients {
		v.SendMsg(m)
	}
}

func (s *Server) DelClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wg.Done()
	delete(s.clients, c.addr.String())
}

func (s *Server) AddClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wg.Add(1)
	s.clients[c.addr.String()] = c
}

func (s *Server) Stop() {
	s.cancel()
}

func (s *Server) Run() {
	go s.run()
}

func (s *Server) Wait() {
	s.wg.Wait()
}

func (s *Server) NewClient() *Client {
	c := NewClient(s.ctx)
	c.ss = s
	return c
}

func (s *Server) Service() uint32 {
	return s.service
}

func (s *Server) run() {
	log.Println(s.addr.Network(), "server startup", s.addr)
	defer func() {
		if err := recover(); err != nil {
			s.err = err
			s.cancel()
		}
	}()
	go func() {
		<-s.ctx.Done()
		_ = s.lis.Close()
	}()
	for {
		conn, err := s.lis.Accept()
		if err != nil {
			panic(err)
		}
		c := NewClient(s.ctx)
		c.NetStream = &NetStream{Conn: conn}
		if err := c.addr.From(conn.RemoteAddr().String()); err != nil {
			log.Println("set client addr error", err)
			_ = conn.Close()
			continue
		}
		c.typ = ClientIn
		c.ss = s
		log.Println("new connection", conn.RemoteAddr())
		c.Loop()
	}
}

func NewServer(ctx context.Context, conf *Config) (*Server, error) {
	s := &Server{}
	s.addr = conf.GetListenAddr().ToTcpAddr()
	lis, err := net.ListenTCP(s.addr.Network(), s.addr)
	if err != nil {
		return nil, err
	}
	s.lis = lis
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.clients = map[string]*Client{}
	s.addrs = NewNetAddrMap()
	s.service = SERVICE_SIG_DATA | SERVICE_SIG_TAG
	return s, nil
}
