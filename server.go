package xginx

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	//收到所有消息
	NetMsgTopic = "NetMsg"
	//收到地址消息订阅
	NetMsgAddrsTopic = "NetMsgAddrs"
	//收到区块头
	NetMsgHeadersTopic = "NetMsgHeaders"
	//收到区块
	NetMsgTxTopic = "NetMsgTx"
	//收到交易
	NetMsgBlockTopic = "NetMsgBlock"
)

type AddrNode struct {
	addr      NetAddr
	addTime   time.Time //加入时间
	openTime  time.Time //打开时间
	closeTime time.Time //关闭时间
	lastTime  time.Time //最后链接时间
}

type AddrMap struct {
	mu    sync.RWMutex
	addrs map[string]*AddrNode
}

func (m *AddrMap) Get(a NetAddr) *AddrNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.addrs[a.String()]
}

func (m *AddrMap) Set(a NetAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()
	node := &AddrNode{
		addr:    a,
		addTime: time.Now(),
	}
	m.addrs[a.String()] = node
}

func NewAddrMap() *AddrMap {
	return &AddrMap{
		addrs: map[string]*AddrNode{},
	}
}

type IServer interface {
	Start(ctx context.Context)
	Stop()
	Wait()
	NewClient() *Client
	BroadMsg(c *Client, m MsgIO)
}

var (
	Server IServer = &server{}
)

type server struct {
	ser *TcpServer
}

func (s *server) BroadMsg(c *Client, m MsgIO) {
	s.ser.BroadMsg(c, m)
}

func (s *server) Wait() {
	s.ser.Wait()
}

func (s *server) NewClient() *Client {
	return s.ser.NewClient()
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
	addrs   *AddrMap
	single  sync.Mutex
}

//地址是否打开
func (s *TcpServer) IsOpen(a NetAddr) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, has := s.clients[a.String()]
	return has
}

func (s *TcpServer) NewMsgAddrs(c *Client) *MsgAddrs {
	s.addrs.mu.RLock()
	s.addrs.mu.RUnlock()
	msg := &MsgAddrs{}
	for _, v := range s.addrs.addrs {
		//不包括它自己
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
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *TcpServer) Service() uint32 {
	return s.service
}

//开始连接一个地址
func (s *TcpServer) openAddr(addr NetAddr) error {
	c := s.NewClient()
	err := c.Open(addr)
	if err == nil {
		c.Loop()
	}
	return err
}

//收到地址列表，如果没有达到最大链接开始链接
func (s *TcpServer) recvMsgAddrs(c *Client, msg *MsgAddrs) error {
	if cl := s.ConnNum(); cl >= conf.MaxConn {
		return fmt.Errorf("max conn=%d ,pause connect client", cl)
	}
	for _, addr := range msg.Addrs {
		if !addr.IsGlobalUnicast() {
			continue
		}
		if s.HasAddr(addr) {
			continue
		}
		err := s.openAddr(addr)
		if err != nil {
			return fmt.Errorf("connect %v error %w", addr, err)
		}
	}
	return nil
}

func (s *TcpServer) recoverError() {
	//if err := recover(); err != nil {
	//	s.err = err
	//	s.cancel()
	//}
}

func (s *TcpServer) recvMsgTx(c *Client, tx *TX) error {
	bi := GetBlockIndex()
	if err := tx.Check(bi, true); err != nil {
		return err
	}
	//放入交易池
	return bi.tp.PushBack(tx)
}

//获取一个连接的主节点区块高度>=h
func (s *TcpServer) FindClient(h uint32) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		if c.Service&SERVICE_NODE == 0 {
			continue
		}
		if c.Height < h {
			continue
		}
		return c
	}
	return nil
}

//收到块数据
func (s *TcpServer) recvMsgBlock(c *Client, blk *BlockInfo, dt *time.Timer) error {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	ps := GetPubSub()
	bblk := false
	//尝试连接区块头
	if err := bi.LinkHeader(blk.Header); err == nil {
		LogInfo("link block header id =", blk.Header)
		bblk = true
	}
	//尝试更新区块数据
	if err := bi.UpdateBlk(blk); err == nil {
		LogInfo("update block to chain success, blk =", blk, "height =", blk.Meta.Height, "cache =", bi.CacheSize())
		dt.Reset(time.Microsecond * 10)
	} else {
		LogError("update blk error", err)
		bblk = false
	}
	//如果是新块,广播区块
	if bblk {
		ps.Pub(blk, NewBlockLinkTopic)
		s.BroadMsg(c, NewMsgBlock(blk))
	}
	return nil
}

//下载块数据
func (s *TcpServer) reqMsgGetBlock() {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	bv := bi.GetBestValue()
	iter := bi.NewIter()
	if bv.IsValid() {
		if !iter.SeekID(bv.Id, 1) {
			return
		}
	} else {
		if !iter.First() {
			return
		}
	}
	if !iter.Next() {
		return
	}
	c := s.FindClient(iter.Height())
	if c == nil {
		return
	}
	msg := &MsgGetInv{}
	msg.AddInv(InvTypeBlock, iter.ID())
	c.SendMsg(msg)
}

//下载区块头
func (s *TcpServer) recvMsgHeaders(c *Client, msg *MsgHeaders) error {
	s.single.Lock()
	defer s.single.Unlock()
	//链接头
	bi := GetBlockIndex()
	for _, hv := range msg.Headers {
		err := bi.LinkHeader(hv)
		if err != nil {
			return fmt.Errorf("recv msgheader link header error %w", err)
		}
		LogInfo("link block header id =", hv, "height =", bi.LastHeight())
	}
	//请求下一批,不包含最后一个
	rmsg := bi.ReqMsgHeaders()
	c.SendMsg(rmsg)
	return nil
}

func (s *TcpServer) dispatch(idx int, ch chan interface{}, pt *time.Timer, dt *time.Timer) {
	LogInfo("server dispatch startup", idx)
	defer s.recoverError()
	for {
		select {
		case <-s.ctx.Done():
			err := s.lis.Close()
			if err != nil {
				LogError("server listen close", err)
			}
			return
		case cv := <-ch:
			m, ok := cv.(*ClientMsg)
			if !ok {
				break
			}
			if msg, ok := m.m.(*MsgAddrs); ok && len(msg.Addrs) > 0 {
				err := s.recvMsgAddrs(m.c, msg)
				if err != nil {
					LogError(err)
				}
				break
			}
			if msg, ok := m.m.(*MsgHeaders); ok && len(msg.Headers) > 0 {
				err := s.recvMsgHeaders(m.c, msg)
				if err != nil {
					LogError(err)
				}
				break
			}
			if msg, ok := m.m.(*MsgBlock); ok {
				err := s.recvMsgBlock(m.c, msg.Blk, dt)
				if err != nil {
					LogError(err)
					//m.c.SendMsg(NewMsgError(ErrCodeRecvBlock, err))
				}
				break
			}
			if msg, ok := m.m.(*MsgTx); ok {
				err := s.recvMsgTx(m.c, msg.Tx)
				if err != nil {
					m.c.SendMsg(NewMsgError(ErrCodeRecvTx, err))
				}
				break
			}
		case <-dt.C:
			s.reqMsgGetBlock()
			dt.Reset(time.Second)
		case <-pt.C:
			//测试连接
			//if s.ConnNum() == 1 {
			//	pt.Reset(time.Second * 3)
			//	break
			//}
			//err := s.openAddr(NetAddrForm("192.168.31.178:9333"))
			//if err != nil {
			//	LogError(err)
			//}
			pt.Reset(time.Second * 3)
		}
	}
}

func (s *TcpServer) run() {
	LogInfo(s.addr.Network(), "server startup", s.addr)
	defer s.recoverError()
	ch := GetPubSub().Sub(NetMsgTopic)
	pt := time.NewTimer(time.Second)
	dt := time.NewTimer(time.Second)
	for i := 0; i < 4; i++ {
		go s.dispatch(i, ch, pt, dt)
	}
	for {
		conn, err := s.lis.Accept()
		if err != nil {
			LogError(err)
			s.cancel()
			return
		}
		c := s.NewClient()
		c.NetStream = NewNetStream(conn)
		err = c.addr.From(conn.RemoteAddr().String())
		if err != nil {
			_ = conn.Close()
			continue
		}
		c.typ = ClientIn
		c.isopen = true
		LogInfo("new connection", conn.RemoteAddr())
		c.Loop()
	}
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
	s.addrs = NewAddrMap()
	return s, nil
}
