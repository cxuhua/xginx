package xginx

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

//客户端实现
type IRpcClient interface {
	IRpcAccount
	IRpcMiner
}

type rpcclientimp struct {
	rpcaccountimp
	rpcminerimp
}

func newRpcClient(c *rpc.Client) (IRpcClient, error) {
	rv := &rpcclientimp{}
	rv.rpcaccountimp.rc = c
	rv.rpcminerimp.rc = c
	return rv, nil
}

type RpcNIL struct {
}

var (
	RpcNil = &RpcNIL{}
)

//rpc 服务
type IRpcServer interface {
	Start(ctx context.Context)
	Stop()
	Wait()
}

var (
	Rpc IRpcServer = &rpcimp{}
)

type rpcimp struct {
	ser *RpcServer
}

func (s *rpcimp) Wait() {
	s.ser.Wait()
}

func (s *rpcimp) Stop() {
	s.ser.Stop()
}

func (s *rpcimp) Start(ctx context.Context) {
	ser, err := NewRpcServer(ctx, conf)
	if err != nil {
		panic(err)
	}
	s.ser = ser
	s.ser.Run()
}

type RpcServer struct {
	lis    net.Listener
	addr   *net.TCPAddr
	rser   *rpc.Server
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	err    interface{}
}

func (s *RpcServer) Stop() {
	s.cancel()
}

func (s *RpcServer) Run() {
	go s.run()
}

func (s *RpcServer) Wait() {

}

func (s *RpcServer) init() error {
	err := s.rser.Register(&RpcAccount{})
	if err != nil {
		return err
	}
	err = s.rser.Register(&RpcMiner{})
	if err != nil {
		return err
	}
	return nil
}

func (s *RpcServer) run() {
	log.Println(s.addr.Network(), "rpc startup", s.addr)
	defer func() {
		if err := recover(); err != nil {
			s.err = err
			s.cancel()
		}
	}()
	//注册服务
	if err := s.init(); err != nil {
		panic(err)
	}
	go func() {
		<-s.ctx.Done()
		_ = s.lis.Close()
	}()
	go s.rser.Accept(s.lis)
}

//创建rpc客户端
func NewRpcClient(ip string, port int) (IRpcClient, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), time.Second*30)
	if err != nil {
		return nil, err
	}
	rc := rpc.NewClient(conn)
	return newRpcClient(rc)
}

func NewRpcServer(ctx context.Context, c *Config) (*RpcServer, error) {
	s := &RpcServer{}
	s.addr = c.GetRpcListenAddr().ToTcpAddr()
	lis, err := net.ListenTCP(s.addr.Network(), s.addr)
	if err != nil {
		return nil, err
	}
	s.rser = rpc.NewServer()
	s.lis = lis
	s.ctx, s.cancel = context.WithCancel(ctx)
	return s, nil
}
