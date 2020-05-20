package xginx

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// 消息订阅
const (
	//收到所有消息
	NetMsgTopic = "NetMsg"
	//创建了新的交易进入了交易池
	NewTxTopic = "NewTx"
)

//AddrNode 地址节点
type AddrNode struct {
	addr      NetAddr
	addTime   time.Time //加入时间
	openTime  time.Time //打开时间
	closeTime time.Time //关闭时间
	lastTime  time.Time //最后链接时间
	connErr   int       //连接错误次数
}

//IsNeedConn 是否需要连接
func (node AddrNode) IsNeedConn() bool {
	if !node.addr.IsGlobalUnicast() {
		return false
	}
	span := time.Minute * 5 * time.Duration(node.connErr+1)
	if time.Now().Sub(node.lastTime) > span {
		return true
	}
	return false
}

//AddrMap 地址表
type AddrMap struct {
	mu    sync.RWMutex
	addrs map[string]*AddrNode
}

//IncErr 添加错误次数
func (m *AddrMap) IncErr(a NetAddr) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v := m.addrs[a.String()]
	if v != nil {
		v.connErr++
	}
}

//Has 是否存在
func (m *AddrMap) Has(a NetAddr) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, has := m.addrs[a.String()]
	return has
}

//Get 获取地址
func (m *AddrMap) Get(a NetAddr) *AddrNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.addrs[a.String()]
}

//Set 设置地址
func (m *AddrMap) Set(a NetAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()
	node := &AddrNode{
		addr:    a,
		addTime: time.Now(),
	}
	m.addrs[a.String()] = node
}

//NewAddrMap 创建地址集合
func NewAddrMap() *AddrMap {
	return &AddrMap{
		addrs: map[string]*AddrNode{},
	}
}

//IServer 服务器接口
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

//默认服务
var (
	Server = NewTCPServer()
)

//TCPServer 基于tcp的服务
type TCPServer struct {
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

//DoOpt 操作通道
func (s *TCPServer) DoOpt(opt int) {
	s.dopt <- opt
}

//Addrs 获取节点保留的地址
func (s *TCPServer) Addrs() []*AddrNode {
	s.addrs.mu.RLock()
	defer s.addrs.mu.RUnlock()
	ds := []*AddrNode{}
	for _, v := range s.addrs.addrs {
		ds = append(ds, v)
	}
	return ds
}

//Clients 获取连接的客户端
func (s *TCPServer) Clients() []*Client {
	cs := []*Client{}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.cls {
		cs = append(cs, v)
	}
	return cs
}

//IsOpen 地址是否打开
func (s *TCPServer) IsOpen(id uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, has := s.cls[id]
	return has
}

//NewMsgAddrs 创建地址列表消息
func (s *TCPServer) NewMsgAddrs(c *Client) *MsgAddrs {
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

//BroadMsg 如果c不空不会广播给c
func (s *TCPServer) BroadMsg(m MsgIO, skips ...*Client) {
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

//IsAddrOpen 地址是否已经链接
func (s *TCPServer) IsAddrOpen(addr NetAddr) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.cls {
		if v.Addr.Equal(addr) {
			return true
		}
	}
	return false
}

//HasClient 是否存在客户端
func (s *TCPServer) HasClient(id uint64, c *Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.cls[id]
	if !ok {
		c.id = id
		s.cls[id] = c
	}
	return ok
}

//DelClient 移除客户端
func (s *TCPServer) DelClient(id uint64, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cls, id)
}

//AddClient 添加客户端
func (s *TCPServer) AddClient(id uint64, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cls[id] = c
}

//Stop 地址服务
func (s *TCPServer) Stop() {
	s.cfun()
}

//Run 启动服务
func (s *TCPServer) Run() {
	go s.run()
}

//Wait 等待服务停止
func (s *TCPServer) Wait() {
	s.wg.Wait()
}

//NewClientWithConn 从网络连接创建客户端
func (s *TCPServer) NewClientWithConn(conn net.Conn) *Client {
	c := s.NewClient()
	c.NetStream = NewNetStream(conn)
	return c
}

//NewClient 创建客户端用来连接其他服务器
func (s *TCPServer) NewClient() *Client {
	c := &Client{ss: s}
	c.cctx, c.cfun = context.WithCancel(s.cctx)
	c.wc = make(chan MsgIO, 4)
	c.rc = make(chan MsgIO, 4)
	c.pt = time.NewTimer(time.Second * time.Duration(Rand(40, 60)))
	c.vt = time.NewTimer(time.Second * 10) //10秒内不应答MsgVersion将关闭
	c.vmap = &sync.Map{}
	return c
}

//ConnNum 活跃连接地址数量
func (s *TCPServer) ConnNum() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cls)
}

//开始连接一个地址
func (s *TCPServer) openAddr(addr NetAddr) error {
	c := s.NewClient()
	err := c.Open(addr)
	if err == nil {
		c.Loop()
	}
	return err
}

//收到地址列表，如果没有达到最大链接开始链接
func (s *TCPServer) recvMsgAddrs(c *Client, msg *MsgAddrs) error {
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

func (s *TCPServer) recoverError() {
	if *IsDebug {
		s.cfun()
	} else if err := recover(); err != nil {
		s.err = err
		s.cfun()
	} else {
		s.cfun()
	}
}

func (s *TCPServer) recvMsgTx(c *Client, msg *MsgTx) error {
	bi := GetBlockIndex()
	txp := bi.GetTxPool()
	rsg := &MsgTx{}
	for _, tx := range msg.Txs {
		//放入交易池
		err := txp.PushTx(bi, tx)
		if err != nil {
			continue
		}
		rsg.Add(tx)
		LogInfo("recv new tx =", tx, " txpool size =", txp.Len())
	}
	//广播到周围节点,不包括c
	if len(rsg.Txs) > 0 {
		s.BroadMsg(rsg, c)
	}
	return nil
}

//获取一个可以获取此区块数据的连接
func (s *TCPServer) findBlockClient(h uint32) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.cls {
		if c.Service&FullNodeFlag == 0 {
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
func (s *TCPServer) recvMsgHeaders(c *Client, msg *MsgHeaders) error {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	return bi.Unlink(msg.Headers)
}

//收到块数据
func (s *TCPServer) recvMsgBlock(c *Client, msg *MsgBlock) error {
	s.single.Lock()
	defer s.single.Unlock()
	ps := GetPubSub()
	bi := GetBlockIndex()
	//尝试更新区块数据
	if err := bi.LinkBlk(msg.Blk); err != nil {
		LogError("link block error", err)
		return err
	}
	LogInfo("update block ", msg.Blk, "height =", msg.Blk.Meta.Height, "cache =", bi.CacheSize())
	s.dt.Reset(time.Microsecond * 300)
	//如果是新区块继续广播
	if msg.IsNewBlock() {
		s.BroadMsg(msg, c)
	}
	ps.Pub(msg.Blk, NewRecvBlockTopic)
	return nil
}

//定时向拥有更高区块的节点请求区块数据
func (s *TCPServer) reqMsgGetBlock() {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	bv := bi.GetBestValue()
	//查询拥有这个高度的客户端
	c := s.findBlockClient(bv.Next())
	if c != nil {
		msg := &MsgGetBlock{
			Next: bv.Next(),
			Last: bv.LastID(),
		}
		c.SendMsg(msg)
	}
}

//尝试重新连接其他地址
func (s *TCPServer) tryConnect() {
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
			s.addrs.IncErr(v)
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

func (s *TCPServer) dispatch(idx int, ch chan interface{}) {
	LogInfo("server dispatch startup", idx)
	defer s.recoverError()
	s.wg.Add(1)
	defer s.wg.Done()
	for {
		select {
		case opt := <-s.dopt:
			switch opt {
			case 1:
				s.loadSeedIP()
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
func (s *TCPServer) loadSeedIP() {
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

func (s *TCPServer) run() {
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

//GetPkg 获取广播数据包
func (s *TCPServer) GetPkg(id string) (MsgIO, bool) {
	msg, has := s.pkgs.Get(id)
	if !has {
		return nil, false
	}
	return msg.(MsgIO), true
}

//SetPkg 保存广播数据包
func (s *TCPServer) SetPkg(id string, m MsgIO) {
	s.pkgs.Set(id, m, time.Minute*5)
}

//HasPkg 是否有广播数据包
func (s *TCPServer) HasPkg(id string) bool {
	_, has := s.pkgs.Get(id)
	if !has {
		s.pkgs.Set(id, time.Now(), time.Minute*5)
	}
	return has
}

//Start 启动服务
func (s *TCPServer) Start(ctx context.Context, lptr IListener) {
	s.lptr = lptr
	s.cctx, s.cfun = context.WithCancel(ctx)
	s.addr = conf.GetTCPListenAddr().ToTCPAddr()
	tcplis, err := net.ListenTCP(s.addr.Network(), s.addr)
	if err != nil {
		panic(err)
	}
	s.tcplis = tcplis
	s.Run()
}

//NewTCPServer 创建TCp服务
func NewTCPServer() IServer {
	s := &TCPServer{}
	s.cls = map[uint64]*Client{}
	s.addrs = NewAddrMap()
	s.dopt = make(chan int, 5)
	s.pt = time.NewTimer(time.Second)
	s.dt = time.NewTimer(time.Second)
	s.pkgs = cache.New(time.Minute*5, time.Minute*15)
	return s
}
