package xginx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

//连接类型
const (
	ClientIn  = 1
	ClientOut = 2
)

//ClientMsg 是网络消息通道数据类型
type ClientMsg struct {
	c *Client
	m MsgIO
}

//NewClientMsg 会创建一个新的网络通道数据
func NewClientMsg(c *Client, m MsgIO) *ClientMsg {
	return &ClientMsg{
		c: c,
		m: m,
	}
}

//属性名称
const (
	MapKeyFilter = "BloomFilter"
)

//Client 连接客户端类型定义
type Client struct {
	*NetStream
	typ     int
	cctx    context.Context
	cfun    context.CancelFunc
	wc      chan MsgIO
	rc      chan MsgIO
	Addr    NetAddr
	id      uint64
	err     interface{}
	ss      *TCPServer
	ping    int
	pt      *time.Timer
	vt      *time.Timer
	isopen  bool      //收到msgversion算打开成功
	Ver     uint32    //节点版本
	Service uint32    //节点提供的服务
	Height  uint32    //节点区块高度
	vmap    *sync.Map //属性存储器
}

//FilterAdd 添加过滤数据
func (c *Client) FilterAdd(key []byte) error {
	blm, has := c.GetFilter()
	if !has {
		return errors.New("VMAP_KEY_FILTER type miss")
	}
	blm.Add(key)
	return nil
}

//LoadFilter 设置过滤器
func (c *Client) LoadFilter(funcs uint32, tweak uint32, filter []byte) error {
	blm, err := NewBloomFilter(funcs, tweak, filter)
	if err != nil {
		return err
	}
	ptr, _ := c.vmap.LoadOrStore(MapKeyFilter, blm)
	if ptr == nil {
		return errors.New("VMAP_KEY_FILTER store or load error")
	}
	blm, ok := ptr.(*BloomFilter)
	if !ok {
		return errors.New("VMAP_KEY_FILTER type error")
	}
	return nil
}

//FilterClear 清除过滤器
func (c *Client) FilterClear() {
	c.vmap.Delete(MapKeyFilter)
}

//GetFilter 获取连接上的过滤器
//不存在返回nil,false
func (c *Client) GetFilter() (*BloomFilter, bool) {
	ptr, has := c.vmap.Load(MapKeyFilter)
	if !has {
		return nil, false
	}
	blm, ok := ptr.(*BloomFilter)
	if !ok {
		panic(errors.New("VMAP_KEY_FILTER type error"))
	}
	return blm, true
}

//FilterHas 检测过滤器是否存在
func (c *Client) FilterHas(key []byte) bool {
	if blm, isset := c.GetFilter(); isset {
		return blm.Has(key)
	}
	return true
}

//Equal 是否是相同的客户端
//检测节点id是否一致
func (c *Client) Equal(b *Client) bool {
	return c.id == b.id
}

//IsIn 是否是连入的
func (c *Client) IsIn() bool {
	return c.typ == ClientIn
}

//IsOut 是否是连出的
func (c *Client) IsOut() bool {
	return c.typ == ClientOut
}

//当收到新的区块请求时
func (c *Client) reqMsgBlock(msg *MsgGetBlock) {
	bi := GetBlockIndex()
	iter := bi.NewIter()
	//如果请求的下个区块不存在
	if !iter.SeekHeight(msg.Next) {
		rsg := NewMsgError(ErrCodeBlockMiss, fmt.Errorf("seek to next %d failed", msg.Next))
		c.SendMsg(rsg)
		return
	}
	nextid := iter.Curr().MustID()
	//如果是第一个区块直接发送
	if conf.IsGenesisID(nextid) {
		rsg := bi.NewMsgGetBlock(nextid)
		c.SendMsg(rsg)
		return
	}
	//否则它的上一个区块应该就是msg.Last
	if !iter.Prev() || !iter.Prev() {
		rsg := NewMsgError(ErrCodeBlockMiss, fmt.Errorf("next height %d prev miss", msg.Next))
		c.SendMsg(rsg)
		return
	}
	//如果id不匹配，返回区块头作为更长区块链的证据
	previd := iter.Curr().MustID()
	if !previd.Equal(msg.Last) {
		rsg := bi.NewMsgHeaders(msg)
		c.SendMsg(rsg)
		return
	}
	//获取下个区块数据返回
	rsg := bi.NewMsgGetBlock(nextid)
	c.SendMsg(rsg)
}

func (c *Client) processMsg(m MsgIO) error {
	ps := GetPubSub()
	bi := GetBlockIndex()
	typ := m.Type()
	switch typ {
	case NtBroadPkg:
		msg := m.(*MsgBroadPkg)
		//如果已经有包就忽略
		if c.ss.HasPkg(msg.MsgID.RecvKey()) {
			break
		}
		//只向最先到达的头发送数据应答
		rsg := &MsgBroadAck{MsgID: msg.MsgID}
		c.SendMsg(rsg)
	case NtBroadAck:
		msg := m.(*MsgBroadAck)
		//收到应答，有数据就发送回去
		if rsg, ok := c.ss.GetPkg(msg.MsgID.SendKey()); ok {
			c.SendMsg(rsg)
		}
	case NtGetBlock:
		msg := m.(*MsgGetBlock)
		c.reqMsgBlock(msg)
	case NtGetTxPool:
		msg := m.(*MsgGetTxPool)
		tp := bi.GetTxPool()
		c.SendMsg(tp.NewMsgTxPool(msg))
	case NtTxPool:
		msg := m.(*MsgTxPool)
		tp := bi.GetTxPool()
		tp.PushTxs(bi, msg)
	case NtTxMerkle:
		msg := m.(*MsgTxMerkle)
		err := msg.Verify(bi)
		if err != nil {
			LogError("verify txid merkle error", err)
		}
	case NtGetMerkle:
		msg := m.(*MsgGetMerkle)
		rsg, err := bi.NewMsgTxMerkle(msg.TxID)
		if err != nil {
			esg := NewMsgError(ErrCodeTxMerkle, err)
			esg.Ext = msg.TxID[:]
			c.SendMsg(esg)
		} else {
			c.SendMsg(rsg)
		}
	case NtFilterLoad:
		msg := m.(*MsgFilterLoad)
		err := c.LoadFilter(msg.Funcs, msg.Tweak, msg.Filter)
		if err != nil {
			c.SendMsg(NewMsgError(ErrCodeFilterLoad, err))
		}
	case NtFilterAdd:
		msg := m.(*MsgFilterAdd)
		err := c.FilterAdd(msg.Key)
		if err != nil {
			c.SendMsg(NewMsgError(ErrCodeFilterMiss, err))
		}
	case NtFilterClear:
		c.FilterClear()
	case NtAlert:
		msg := m.(*MsgAlert)
		c.ss.BroadMsg(msg, c)
		LogInfo("recv alert message:", msg.Msg.String())
	case NtError:
		msg := m.(*MsgError)
		LogError("recv error msg code =", msg.Code, "error =", msg.Error, c.id)
	case NtGetInv:
		msg := m.(*MsgGetInv)
		if len(msg.Invs) == 0 {
			break
		}
		bi.GetMsgGetInv(msg, c)
	case NtAddrs:
		msg := m.(*MsgAddrs)
		LogInfo("get addrs count =", len(msg.Addrs), "from", c.Addr)
	case NtGetAddrs:
		msg := c.ss.NewMsgAddrs(c)
		c.SendMsg(msg)
	case NtPong:
		//获取对方的区块高度
		msg := m.(*MsgPong)
		c.Height = msg.Height
		c.ping = msg.Ping()
	case NtPing:
		//ping 并且播报自己的区块高度
		msg := m.(*MsgPing)
		c.Height = msg.Height
		rsg := msg.NewPong(bi.BestHeight())
		c.SendMsg(rsg)
	case NtVersion:
		msg := m.(*MsgVersion)
		//保存到地址列表
		if msg.Addr.IsGlobalUnicast() {
			c.ss.addrs.Set(msg.Addr)
		}
		//防止两节点重复连接，并且防止自己连接自己
		//如果不存在将会加入连接列表
		if c.ss.HasClient(msg.NodeID, c) {
			c.Close()
			return errors.New("has connection,closed")
		}
		//保存节点信息
		c.Addr = msg.Addr
		c.Height = msg.Height
		c.Ver = msg.Ver
		c.Service = msg.Service
		//如果是连入的，返回节点版本信息
		if c.IsIn() {
			rsg := bi.NewMsgVersion()
			c.SendMsg(rsg)
		}
	}
	//发布消息
	ps.Pub(NewClientMsg(c, m), NetMsgTopic)
	return nil
}

func (c *Client) recoverError() {
	if *IsDebug {
		c.cfun()
		return
	}
	if err := recover(); err != nil {
		c.err = err
		c.cfun()
	}
}

func (c *Client) stop() {
	defer c.recoverError()
	c.isopen = false
	//更新关闭时间
	if ap := c.ss.addrs.Get(c.Addr); ap != nil {
		ap.closeTime = time.Now()
	}
	if c.ss != nil {
		c.ss.DelClient(c.id, c)
	}
	close(c.wc)
	close(c.rc)
	if c.Conn != nil {
		_ = c.Conn.Close()
	}
	LogInfo("client stop", c.Addr, "error=", c.err)
}

//Open 连接到指定地址
func (c *Client) Open(addr NetAddr) error {
	if addr.Equal(conf.GetNetAddr()) {
		return errors.New("self connect self,ignore")
	}
	if c.ss.IsAddrOpen(addr) {
		return errors.New("addr has client connected,ignore")
	}
	return c.connect(addr)
}

func (c *Client) connect(addr NetAddr) error {
	if !addr.IsGlobalUnicast() {
		return errors.New("addr not is global unicast,can't connect")
	}
	//更新最后链接时间
	if ap := c.ss.addrs.Get(addr); ap != nil {
		ap.lastTime = time.Now()
	}
	conn, err := net.DialTimeout(addr.Network(), addr.Addr(), time.Second*10)
	if err != nil {
		return err
	}
	//更新链接时间
	if ap := c.ss.addrs.Get(addr); ap != nil {
		ap.openTime = time.Now()
	}
	c.isopen = true
	LogInfo("connect to", addr, "success")
	c.typ = ClientOut
	c.Addr = addr
	c.NetStream = NewNetStream(conn)
	//主动发送第一个包
	bi := GetBlockIndex()
	c.SendMsg(bi.NewMsgVersion())
	return nil
}

//Loop 开始后台启动服务
func (c *Client) Loop() {
	go c.loop()
}

func (c *Client) loop() {
	defer c.stop()
	c.ss.wg.Add(1)
	defer c.ss.wg.Done()
	//读取数据包
	go func() {
		defer c.recoverError()
		for {
			m, err := c.ReadMsg()
			if err != nil {
				c.err = err
				c.cfun()
				break
			}
			c.rc <- m
		}
	}()
	for {
		select {
		case wp := <-c.wc:
			if err := c.WriteMsg(wp); err != nil {
				c.err = fmt.Errorf("write msg error %v", err)
				c.cfun()
				return
			}
		case rp := <-c.rc:
			if err := c.processMsg(rp); err != nil {
				LogError("process msg", rp.Type(), "error", err)
			}
		case <-c.vt.C:
			if !c.isopen {
				c.Close()
				LogError("MsgVersion recv timeout,closed")
				break
			}
			bi := GetBlockIndex()
			tp := bi.GetTxPool()
			//获取对方地址列表
			if c.Service&FullNodeFlag != 0 {
				msg := c.ss.NewMsgAddrs(c)
				c.SendMsg(msg)
			}
			//同步双发交易池数据
			if c.Service&FullNodeFlag != 0 {
				msg := tp.NewMsgGetTxPool()
				c.SendMsg(msg)
			}
		case <-c.pt.C:
			if !c.isopen {
				break
			}
			//定时ping消息
			bi := GetBlockIndex()
			msg := NewMsgPing(bi.BestHeight())
			c.SendMsg(msg)
			c.pt.Reset(time.Second * time.Duration(Rand(40, 60)))
		case <-c.cctx.Done():
			return
		}
	}
}

//BroadMsg 广播消息
func (c *Client) BroadMsg(m MsgIO) {
	id, err := m.ID()
	if err != nil {
		panic(err)
	}
	//数据先保存在缓存
	c.ss.SetPkg(id.SendKey(), m)
	//发送广播包头
	msg := &MsgBroadPkg{MsgID: id}
	c.wc <- msg
}

//SendMsg 发送消息
func (c *Client) SendMsg(m MsgIO) {
	c.wc <- m
}

//Close 关闭客户端
func (c *Client) Close() {
	c.cfun()
}
