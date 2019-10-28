package xginx

import (
	"context"
	"log"
	"net"
	"sync"
)

type Server struct {
	lis    net.Listener
	addr   *net.TCPAddr
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	err    interface{}
	wg     sync.WaitGroup
}

func (s *Server) stop() {
	s.cancel()
}

func (s *Server) run() {
	log.Println(s.addr.Network(), "server startup", s.addr)
	defer func() {
		if err := recover(); err != nil {
			s.err = err
			log.Println("server err=", s.err, "close")
		}
		log.Println("wait server close")
		s.wg.Wait()
		log.Println("server closed")
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
			s.err = err
			continue
		}
		c.typ = ClientIn
		c.ss = s
		s.wg.Add(1)
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
	return s, nil
}
