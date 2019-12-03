package xginx

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/gin-gonic/gin"
)

const (
	//收到所有消息
	NetMsgTopic = "NetMsg"
	//创建了新的交易进入了交易池
	NewTxTopic = "NewTx"
)

type AddrNode struct {
	addr      NetAddr
	addTime   time.Time //加入时间
	openTime  time.Time //打开时间
	closeTime time.Time //关闭时间
	lastTime  time.Time //最后链接时间
}

//是否需要连接
func (node AddrNode) IsNeedConn() bool {
	if !node.addr.IsGlobalUnicast() {
		return false
	}
	if time.Now().Sub(node.lastTime) > time.Minute*10 {
		return true
	}
	return false
}

//地址表
type AddrMap struct {
	mu    sync.RWMutex
	addrs map[string]*AddrNode
}

func (m *AddrMap) Has(a NetAddr) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, has := m.addrs[a.String()]
	return has
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
	Start(ctx context.Context, lis IListener)
	Stop()
	Wait()
	NewClient() *Client
	BroadMsg(m MsgIO, skips ...*Client)
	DoOpt(opt int)
	Clients() []*Client
	Addrs() []*AddrNode
}

var (
	Server = NewTcpServer()
)

type TcpServer struct {
	lptr   IListener
	tcplis net.Listener
	addr   *net.TCPAddr
	cctx   context.Context
	cfun   context.CancelFunc
	mu     sync.RWMutex
	err    interface{}
	wg     sync.WaitGroup
	cls    map[uint64]*Client //连接的所有client
	addrs  *AddrMap
	single sync.Mutex
	dopt   chan int //获取线程做一些操作
	dt     *time.Timer
	pt     *time.Timer
	pkgs   *cache.Cache //包数据缓存
}

//操作通道
func (s *TcpServer) DoOpt(opt int) {
	s.dopt <- opt
}

//获取节点保留的地址
func (s *TcpServer) Addrs() []*AddrNode {
	s.addrs.mu.RLock()
	defer s.addrs.mu.RUnlock()
	ds := []*AddrNode{}
	for _, v := range s.addrs.addrs {
		ds = append(ds, v)
	}
	return ds
}

//获取连接的客户端
func (s *TcpServer) Clients() []*Client {
	cs := []*Client{}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.cls {
		cs = append(cs, v)
	}
	return cs
}

//地址是否打开
func (s *TcpServer) IsOpen(id uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, has := s.cls[id]
	return has
}

func (s *TcpServer) NewMsgAddrs(c *Client) *MsgAddrs {
	s.addrs.mu.RLock()
	defer s.addrs.mu.RUnlock()
	msg := &MsgAddrs{}
	for _, v := range s.addrs.addrs {
		//不包括它自己
		if v.addr.Equal(c.Addr) {
			continue
		}
		if msg.Add(v.addr) {
			break
		}
	}
	return msg
}

//如果c不空不会广播给c
func (s *TcpServer) BroadMsg(m MsgIO, skips ...*Client) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	//检测是否在忽略列表中
	skipf := func(v *Client) bool {
		for _, cc := range skips {
			if cc.Equal(v) {
				return true
			}
		}
		return false
	}
	//一般不会发送给接收到数据的节点
	for _, c := range s.cls {
		if skipf(c) {
			continue
		}
		c.BroadMsg(m)
	}
}

//地址是否已经链接
func (s *TcpServer) IsAddrOpen(addr NetAddr) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.cls {
		if v.Addr.Equal(addr) {
			return true
		}
	}
	return false
}

func (s *TcpServer) HasClient(id uint64, c *Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.cls[id]
	if !ok {
		c.id = id
		s.cls[id] = c
	}
	return ok
}

func (s *TcpServer) DelClient(id uint64, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cls, id)
}

func (s *TcpServer) AddClient(id uint64, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cls[id] = c
}

func (s *TcpServer) Stop() {
	s.cfun()
}

func (s *TcpServer) Run() {
	go s.run()
}

func (s *TcpServer) Wait() {
	s.wg.Wait()
}

func (s *TcpServer) NewClientWithConn(conn net.Conn) *Client {
	c := s.NewClient()
	c.NetStream = NewNetStream(conn)
	return c
}

func (s *TcpServer) NewClient() *Client {
	c := &Client{ss: s}
	c.cctx, c.cfun = context.WithCancel(s.cctx)
	c.wc = make(chan MsgIO, 4)
	c.rc = make(chan MsgIO, 4)
	c.pt = time.NewTimer(time.Second * time.Duration(Rand(40, 60)))
	c.vt = time.NewTimer(time.Second * 10) //10秒内不应答MsgVersion将关闭
	c.vmap = &sync.Map{}
	return c
}

func (s *TcpServer) ConnNum() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cls)
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
		if s.addrs.Has(addr) {
			continue
		}
		if s.IsAddrOpen(addr) {
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
	if gin.Mode() == gin.DebugMode {
		s.cfun()
	} else if err := recover(); err != nil {
		s.err = err
		s.cfun()
	} else {
		s.cfun()
	}
}

func (s *TcpServer) recvMsgTx(c *Client, msg *MsgTx) error {
	bi := GetBlockIndex()
	rsg := &MsgTx{}
	for _, tx := range msg.Txs {
		//获取交易id
		id, err := tx.ID()
		if err != nil {
			return err
		}
		//检测交易是否可用
		if err := tx.Check(bi, true); err != nil {
			return err
		}
		//如果交易已经在区块中忽略
		if _, err := bi.LoadTX(id); err == nil {
			return nil
		}
		txp := bi.GetTxPool()
		//已经存在交易池中忽略
		if txp.Has(id) {
			return nil
		}
		//放入交易池
		err = txp.PushTx(bi, tx)
		if err != nil {
			LogError("push tx error", err, "skip push tx")
			continue
		}
		rsg.Add(tx)
		LogInfo("recv new tx =", tx, " txpool size =", txp.Len())
	}
	//广播到周围节点,不包括c
	s.BroadMsg(rsg, c)
	return nil
}

//获取一个可以获取此区块数据的连接
func (s *TcpServer) findBlockClient(h uint32) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.cls {
		if c.Service&SERVICE_NODE == 0 {
			continue
		}
		if c.Height == InvalidHeight {
			continue
		}
		if c.Height < h {
			continue
		}
		return c
	}
	return nil
}

//收到区块头列表
func (s *TcpServer) recvMsgHeaders(c *Client, msg *MsgHeaders) error {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	return bi.Unlink(msg.Headers)
}

//收到块数据
func (s *TcpServer) recvMsgBlock(c *Client, msg *MsgBlock) error {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	//尝试更新区块数据
	if err := bi.LinkBlk(msg.Blk); err != nil {
		LogError("link block error", err)
		return err
	}
	ps := GetPubSub()
	ps.Pub(msg.Blk, NewRecvBlockTopic)
	LogInfo("update block to chain success, blk =", msg.Blk, "height =", msg.Blk.Meta.Height, "cache =", bi.CacheSize())
	s.dt.Reset(time.Microsecond * 300)
	if msg.IsBroad() {
		s.BroadMsg(msg, c)
	}
	return nil
}

//下载块数据
func (s *TcpServer) reqMsgGetBlock() {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	//获取下个高度的区块
	bv := bi.GetBestValue()
	next := uint32(0)
	last := conf.genesis
	if bv.IsValid() {
		next = bv.Height + 1
		last = bv.Id
	}
	//查询拥有这个高度的客户端
	c := s.findBlockClient(next)
	if c != nil {
		msg := &MsgGetBlock{
			Next: next,
			Last: last,
		}
		c.SendMsg(msg)
	}
}

//尝试重新连接其他地址
func (s *TcpServer) tryConnect() {
	//获取需要连接的地址
	cs := []NetAddr{}
	s.addrs.mu.RLock()
	for _, v := range s.addrs.addrs {
		if s.IsAddrOpen(v.addr) {
			continue
		}
		if !v.IsNeedConn() {
			continue
		}
		v.lastTime = time.Now()
		cs = append(cs, v.addr)
	}
	s.addrs.mu.RUnlock()
	//开始连接
	for _, v := range cs {
		c := s.NewClient()
		err := c.Open(v)
		if err != nil {
			LogError("try connect error", err)
			continue
		}
		//连接成功开始工作
		c.Loop()
		if s.ConnNum() >= conf.MaxConn {
			break
		}
	}
}

func (s *TcpServer) dispatch(idx int, ch chan interface{}) {
	LogInfo("server dispatch startup", idx)
	defer s.recoverError()
	s.wg.Add(1)
	defer s.wg.Done()
	for {
		select {
		case opt := <-s.dopt:
			switch opt {
			case 1:
				s.loadSeedIp()
			case 2:
				LogInfo(opt)
			}
		case <-s.cctx.Done():
			_ = s.tcplis.Close()
			return
		case cv := <-ch:
			if tx, ok := cv.(*TX); ok {
				s.BroadMsg(NewMsgTx(tx))
				break
			}
			m, ok := cv.(*ClientMsg)
			if !ok {
				break
			}
			if msg, ok := m.m.(*MsgAddrs); ok && len(msg.Addrs) > 0 {
				err := s.recvMsgAddrs(m.c, msg)
				if err != nil {
					LogError(err)
				}
			} else if msg, ok := m.m.(*MsgBlock); ok {
				err := s.recvMsgBlock(m.c, msg)
				if err != nil {
					m.c.SendMsg(NewMsgError(ErrCodeRecvBlock, err))
				}
			} else if msg, ok := m.m.(*MsgTx); ok {
				err := s.recvMsgTx(m.c, msg)
				if err != nil {
					m.c.SendMsg(NewMsgError(ErrCodeRecvTx, err))
				}
			} else if msg, ok := m.m.(*MsgHeaders); ok {
				err := s.recvMsgHeaders(m.c, msg)
				if err != nil {
					m.c.SendMsg(NewMsgError(ErrCodeHeaders, err))
				}
			}
			if msg, ok := m.m.(MsgIO); ok {
				s.lptr.OnClientMsg(m.c, msg)
			}
		case <-s.dt.C:
			s.reqMsgGetBlock()
			s.dt.Reset(time.Second * 5)
		case <-s.pt.C:
			if s.ConnNum() < conf.MaxConn {
				s.tryConnect()
			}
			s.pt.Reset(time.Second * 10)
		}
	}
}

//加载seed域名ip地址
func (s *TcpServer) loadSeedIp() {
	lipc := 0
	sipc := 0
	for _, v := range conf.Seeds {
		ips, err := net.LookupIP(v)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			sipc++
			addr := NetAddr{
				ip:   ip,
				port: uint16(9333), //使用默认端口
			}
			if !addr.IsGlobalUnicast() {
				continue
			}
			if addr.Equal(conf.GetNetAddr()) {
				continue
			}
			s.addrs.Set(addr)
			lipc++
		}
	}
	LogInfof("load seed ip %d/%d", lipc, sipc)
}

func (s *TcpServer) run() {
	LogInfo(s.addr.Network(), "server startup", s.addr)
	defer s.recoverError()
	s.wg.Add(1)
	defer s.wg.Done()
	var delay time.Duration
	ch := GetPubSub().Sub(NetMsgTopic, NewTxTopic)
	for i := 0; i < 4; i++ {
		go s.dispatch(i, ch)
	}
	s.dopt <- 1 //load seed ip
	for {
		conn, err := s.tcplis.Accept()
		//是否达到最大连接
		if s.ConnNum() >= conf.MaxConn {
			LogError("conn arrive max,ignore", conn)
			continue
		}
		if err == nil {
			delay = 0
			c := s.NewClientWithConn(conn)
			c.typ = ClientIn
			c.isopen = true
			LogInfo("new connection", conn.RemoteAddr())
			c.Loop()
			continue
		}
		if ne, ok := err.(net.Error); ok && ne.Temporary() {
			if delay == 0 {
				delay = 5 * time.Millisecond
			} else {
				delay *= 2
			}
			if max := 1 * time.Second; delay > max {
				delay = max
			}
			LogError("Accept error: %v; retrying in %v", err, delay)
			time.Sleep(delay)
			continue
		} else {
			s.err = err
			s.cfun()
			break
		}
	}
}

//获取广播数据包
func (s *TcpServer) GetPkg(id string) (MsgIO, bool) {
	msg, has := s.pkgs.Get(id)
	if !has {
		return nil, false
	}
	return msg.(MsgIO), true
}

//保存广播数据包
func (s *TcpServer) SetPkg(id string, m MsgIO) {
	s.pkgs.Set(id, m, time.Minute*5)
}

//是否有广播数据包
func (s *TcpServer) HasPkg(id string) bool {
	_, has := s.pkgs.Get(id)
	if !has {
		s.pkgs.Set(id, time.Now(), time.Minute*5)
	}
	return has
}

func (s *TcpServer) Start(ctx context.Context, lptr IListener) {
	s.lptr = lptr
	s.cctx, s.cfun = context.WithCancel(ctx)
	s.addr = conf.GetTcpListenAddr().ToTcpAddr()
	tcplis, err := net.ListenTCP(s.addr.Network(), s.addr)
	if err != nil {
		panic(err)
	}
	s.tcplis = tcplis
	s.Run()
}

func NewTcpServer() IServer {
	s := &TcpServer{}
	s.cls = map[uint64]*Client{}
	s.addrs = NewAddrMap()
	s.dopt = make(chan int, 5)
	s.pt = time.NewTimer(time.Second)
	s.dt = time.NewTimer(time.Second)
	s.pkgs = cache.New(time.Minute*5, time.Minute*15)
	return s
}
