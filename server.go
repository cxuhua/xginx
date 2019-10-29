package xginx

import (
	"context"
	"log"
	"net"
	"sync"
)

type Server struct {
	lis     net.Listener
	addr    *net.TCPAddr
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
	err     interface{}
	wg      sync.WaitGroup
	clients map[string]*Client //连接的所有client
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

func (s *Server) stop() {
	s.cancel()
}

func (s *Server) Run() {
	go s.run()
}

func (s *Server) Wait() {
	s.wg.Wait()
}

func (s *Server) run() {
	log.Println(s.addr.Network(), "server startup", s.addr)
	defer func() {
		if err := recover(); err != nil {
			s.err = err
			log.Println("server err=", s.err, "close")
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				s.err = err
			}
			s.cancel()
		}()
		for {
			select {
			case <-s.ctx.Done():
				_ = s.lis.Close()
				break
			}
		}
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
		go c.loop()
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
	return s, nil
}
