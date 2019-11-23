package xginx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	//收到所有消息
	NetMsgTopic = "NetMsg"
	//收到地址消息订阅
	NetMsgAddrsTopic = "NetMsgAddrs"
	//收到区块头
	NetMsgHeadersTopic = "NetMsgHeaders"
	//收到交易
	NetMsgTxTopic = "NetMsgTx"
	//收到区块
	NetMsgBlockTopic = "NetMsgBlock"
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
	Start(ctx context.Context, lis IListener)
	Stop()
	Wait()
	NewClient() *Client
	BroadMsg(m MsgIO, skips ...*Client)
	DoOpt(opt int)
	Clients() []*Client
}

var (
	Server IServer = &server{}
)

type server struct {
	ser *TcpServer
}

func (s *server) DoOpt(opt int) {
	s.ser.dopt <- opt
}

func (s *server) Clients() []*Client {
	cs := []*Client{}
	s.ser.mu.RLock()
	defer s.ser.mu.RUnlock()
	for _, v := range s.ser.clients {
		cs = append(cs, v)
	}
	return cs
}

func (s *server) BroadMsg(m MsgIO, skips ...*Client) {
	s.ser.BroadMsg(m, skips...)
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

func (s *server) Start(ctx context.Context, lis IListener) {
	ser, err := NewTcpServer(ctx, conf)
	if err != nil {
		panic(err)
	}
	s.ser = ser
	s.ser.Run()
}

type TcpServer struct {
	lis     net.Listener
	addr    *net.TCPAddr
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
	err     interface{}
	wg      sync.WaitGroup
	clients map[uint64]*Client //连接的所有client
	addrs   *AddrMap
	single  sync.Mutex
	dopt    chan int //获取线程做一些操作
}

//地址是否打开
func (s *TcpServer) IsOpen(id uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, has := s.clients[id]
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
	for _, c := range s.clients {
		if skipf(c) {
			continue
		}
		c.SendMsg(m)
	}
}

func (s *TcpServer) HasAddr(addr NetAddr) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.clients {
		if v.Addr.Equal(addr) {
			return true
		}
	}
	return false
}

func (s *TcpServer) HasClient(id uint64, c *Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.clients[id]
	if !ok {
		c.id = id
		s.clients[id] = c
	}
	return ok
}

func (s *TcpServer) DelClient(id uint64, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, id)
}

func (s *TcpServer) AddClient(id uint64, c *Client) {
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

func (s *TcpServer) NewClientWithConn(conn net.Conn) *Client {
	c := s.NewClient()
	c.NetStream = NewNetStream(conn)
	return c
}

func (s *TcpServer) NewClient() *Client {
	c := &Client{ss: s}
	c.ctx, c.cancel = context.WithCancel(s.ctx)
	c.wc = make(chan MsgIO, 4)
	c.rc = make(chan MsgIO, 4)
	c.ptimer = time.NewTimer(time.Second * time.Duration(Rand(40, 60)))
	c.vtimer = time.NewTimer(time.Second * 10) //10秒内不应答MsgVersion将关闭
	c.vmap = &sync.Map{}
	return c
}

func (s *TcpServer) ConnNum() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
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
	if gin.Mode() == gin.DebugMode {
		s.cancel()
		return
	}
	if err := recover(); err != nil {
		s.err = err
		s.cancel()
	}
}

func (s *TcpServer) recvMsgTx(c *Client, tx *TX) error {
	bi := GetBlockIndex()
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
	LogInfo("recv new tx =", tx, " txpool size =", txp.Len())
	//广播到周围节点,不包括c
	s.BroadMsg(NewMsgTx(tx), c)
	//放入交易池
	return txp.PushBack(tx)
}

//获取一个可以获取此区块头数据的连接
func (s *TcpServer) findHeaderClient(h uint32) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		if c.Height.HH < h {
			continue
		}
		return c
	}
	return nil
}

//获取一个可以获取此区块数据的连接
func (s *TcpServer) findBlockClient(h uint32) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		if c.Service&SERVICE_NODE == 0 {
			continue
		}
		if c.Height.BH < h {
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
	ps := GetPubSub()
	bi := GetBlockIndex()
	//尝试更新区块数据
	if err := bi.UpdateBlk(blk); err != nil {
		LogError("update block error", err)
		return err
	}
	ps.Pub(blk, NewUpdateBlockTopic)
	LogInfo("update block to chain success, blk =", blk, "height =", blk.Meta.Height, "cache =", bi.CacheSize())
	dt.Reset(time.Microsecond * 10)
	return nil
}

//下载块数据
func (s *TcpServer) reqMsgGetBlock() error {
	s.single.Lock()
	defer s.single.Unlock()
	bi := GetBlockIndex()
	if bi.Len() == 0 {
		return nil
	} else if ele, err := bi.GetNextSync(); err != nil {
		return err
	} else if c := s.findBlockClient(ele.Height); c == nil {
		return errors.New("find client error")
	} else if id, err := ele.ID(); err != nil {
		return err
	} else {
		msg := &MsgGetInv{}
		msg.AddInv(InvTypeBlock, id)
		c.SendMsg(msg)
	}
	return nil
}

//需要更多的证明
func (s *TcpServer) reqMoreHeaders(c *Client, id HASH256, cnt uint32) error {
	rsg := &MsgGetHeaders{}
	rsg.Limit = VarInt(-(10 + cnt)) //向前多获取10个
	rsg.Start = id
	c.SendMsg(rsg)
	return nil
}

//下载区块头
//无法连接并存在最后一个,回退到最后存在的那个继续连接
//需要提供证明比我的区块长的列表,后续连接的区块数量比我回退的多
func (s *TcpServer) recvMsgHeaders(c *Client, msg *MsgHeaders) error {
	s.single.Lock()
	defer s.single.Unlock()
	//更新节点区块高度
	bi := GetBlockIndex()
	ps := GetPubSub()
	//检查连续性
	if err := msg.Check(); err != nil {
		return err
	}
	for i, lid, hl := 0, ZERO, len(msg.Headers); i < hl; {
		hv := msg.Headers[i]
		id, err := hv.ID()
		if err != nil {
			return err
		}
		if err := hv.Check(); err != nil {
			return err
		}
		if bi.HasBlock(id) {
			i++
			lid = id
		} else if ele, err := bi.LinkHeader(hv); err == nil {
			ps.Pub(ele, NewLinkHeaderTopic)
			LogInfo("link block header id =", hv, "height =", bi.LastHeight())
			i++
		} else if unnum, err := bi.UnlinkCount(lid); err != nil {
			return err
		} else if hl-i-1 <= int(unnum) {
			return s.reqMoreHeaders(c, msg.LastID(), unnum)
		} else if err = bi.UnlinkTo(lid); err != nil {
			return err
		}
	}
	//请求下一批区块头
	if cc := s.findHeaderClient(msg.Height.HH); cc != nil {
		cc.ReqBlockHeaders(bi, msg.Height.HH)
	}
	return nil
}

//尝试重新连接其他地址
func (s *TcpServer) tryConnect() {
	s.addrs.mu.RLock()
	defer s.addrs.mu.RUnlock()
	for _, v := range s.addrs.addrs {
		if s.HasAddr(v.addr) {
			continue
		}
		if !v.IsNeedConn() {
			continue
		}
		c := s.NewClient()
		err := c.Open(v.addr)
		if err != nil {
			LogError("try connect error", err)
			continue
		}
		c.Loop()
		if s.ConnNum() >= conf.MaxConn {
			break
		}
	}
}

func (s *TcpServer) dispatch(idx int, ch chan interface{}, pt *time.Timer, dt *time.Timer) {
	LogInfo("server dispatch startup", idx)
	defer s.recoverError()
	for {
		select {
		case opt := <-s.dopt:
			switch opt {
			case 1:
				s.loadSeedIp()
			case 2:
				LogInfo(opt)
			}
		case <-s.ctx.Done():
			err := s.lis.Close()
			if err != nil {
				LogError("server listen close", err)
			}
			return
		case cv := <-ch:
			if tx, ok := cv.(*TX); ok {
				//广播交易
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
					m.c.SendMsg(NewMsgError(ErrCodeRecvBlock, err))
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
			_ = s.reqMsgGetBlock()
			dt.Reset(time.Second * 5)
		case <-pt.C:
			if s.ConnNum() < conf.MaxConn {
				s.tryConnect()
			}
			pt.Reset(time.Second * 10)
		}
	}
}

//加载seed域名ip地址
func (s *TcpServer) loadSeedIp() {
	lipc := 0
	for _, v := range conf.Seeds {
		ips, err := net.LookupIP(v)
		if err != nil {
			continue
		}
		for _, ip := range ips {
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
	LogInfo("load seed ip", lipc)
}

func (s *TcpServer) run() {
	LogInfo(s.addr.Network(), "server startup", s.addr)
	defer s.recoverError()
	ch := GetPubSub().Sub(NetMsgTopic, NewTxTopic)
	pt := time.NewTimer(time.Second)
	dt := time.NewTimer(time.Second)
	for i := 0; i < 4; i++ {
		go s.dispatch(i, ch, pt, dt)
	}
	s.dopt <- 1 //load seed ip
	for {
		conn, err := s.lis.Accept()
		if err != nil {
			LogError(err)
			s.cancel()
			return
		}
		c := s.NewClientWithConn(conn)
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
	s.clients = map[uint64]*Client{}
	s.addrs = NewAddrMap()
	s.dopt = make(chan int, 10)
	return s, nil
}
