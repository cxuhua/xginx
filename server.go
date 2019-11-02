package xginx

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"time"
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

type TcpServer struct {
	service uint32
	lis     net.Listener
	addr    *net.TCPAddr
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
	err     interface{}
	wg      sync.WaitGroup
	clients map[Hash160]*Client //连接的所有client
	addrs   *NetAddrMap         //连接我的ip地址和我连接出去的
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

func (s *TcpServer) HasClient(id Hash160, c *Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.clients[id]
	if !ok {
		s.clients[id] = c
	}
	return ok
}

func (s *TcpServer) DelClient(id Hash160, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, id)
}

func (s *TcpServer) AddClient(id Hash160, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[id] = c
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
	c.wc = make(chan MsgIO, 32)
	c.rc = make(chan MsgIO, 32)
	return c
}

func (s *TcpServer) Service() uint32 {
	return s.service
}

func (s *TcpServer) run() {
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
		c := s.NewClient()
		c.NetStream = &NetStream{Conn: conn}
		err = c.addr.From(conn.RemoteAddr().String())
		if err != nil {
			_ = conn.Close()
			continue
		}
		c.typ = ClientIn
		log.Println("new connection", conn.RemoteAddr())
		c.Loop()
	}
}

//生成一个临时id全网唯一
func NewNodeID() Hash160 {
	id := Hash160{}
	_ = binary.Read(rand.Reader, Endian, id[:])
	buf := &bytes.Buffer{}
	buf.Write(id[:])
	buf.Write([]byte(conf.TcprIp))
	_ = binary.Write(buf, Endian, time.Now().UnixNano())
	copy(id[:], HASH160(buf.Bytes()))
	return id
}

func NewTcpServer(ctx context.Context, conf *Config) (*TcpServer, error) {
	s := &TcpServer{}
	s.addr = conf.GetListenAddr().ToTcpAddr()
	lis, err := net.ListenTCP(s.addr.Network(), s.addr)
	if err != nil {
		return nil, err
	}
	s.lis = lis
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.clients = map[Hash160]*Client{}
	s.addrs = NewNetAddrMap()
	s.service = SERVICE_SIG_DATA | SERVICE_SIG_TAG
	return s, nil
}
