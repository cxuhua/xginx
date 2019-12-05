package xginx

import (
	"context"
	"errors"
	"io/ioutil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	//矿工操作 MinerAct
	NewMinerActTopic = "NewMinerAct"
	//更新了一个区块数据 BlockInfo
	NewLinkBlockTopic = "NewLinkBlock"
	//接收的广播区块
	NewRecvBlockTopic = "NewRecvBlock"
	//当交易池中的交易被移除时 txid
	TxPoolDelTxTopic = "TxPoolDelTx"
)

const (
	//每隔多少秒打印挖掘状态
	MINER_LOG_SECONDS = 5
)

const (
	//开始挖矿操作 args(uint32) = block ver
	OptGenBlock = iota
	//设置矿工奖励账号 arg=*Account
	OptSetMiner
	//停止当前区块创建
	OptStopGenBlock
	//发送一个区块头数据进行验证 args = HeaderBytes
	OptSendHeadBytes
)

//计算群组
type MinerGroup struct {
	hb    HeaderBytes
	stop  bool
	wg    sync.WaitGroup
	num   int
	ok    bool
	bh    BlockHeader
	bits  uint32
	times uint64 //总计算次数
	exit  chan bool
}

func (g *MinerGroup) Times() uint64 {
	return atomic.LoadUint64(&g.times)
}

func (g *MinerGroup) docalc(cb HeaderBytes) {
	defer g.wg.Done()
	for i := UR32(); ; i++ {
		if g.stop {
			break
		}
		if id := cb.Hash(); !CheckProofOfWork(id, g.bits) {
			cb.SetNonce(i)
			atomic.AddUint64(&g.times, 1)
		} else {
			g.ok = true
			g.bh = cb.Header()
			break
		}
		if i >= ^uint32(0) {
			cb.SetTime(Miner.TimeNow())
			i = UR32()
		}
	}
	g.Stop()
}

func (g *MinerGroup) Stop() {
	g.stop = true
}

func (g *MinerGroup) StopAndWait() {
	g.stop = true
	<-g.exit
}

func (g *MinerGroup) Run() {
	for i := 0; i < g.num; i++ {
		g.wg.Add(1)
		cb := g.hb.Clone()
		go g.docalc(cb)
	}
	go func() {
		g.wg.Wait()
		g.exit <- true
	}()
}

func NewMinerGroup(hb HeaderBytes, bits uint32, num int) *MinerGroup {
	m := &MinerGroup{
		hb:   hb,
		stop: false,
		num:  num,
		bits: bits,
		exit: make(chan bool),
	}
	return m
}

//矿产操作
type MinerAct struct {
	Opt int
	Arg interface{}
}

//矿工接口
type IMiner interface {
	//开始工作
	Start(ctx context.Context, lis IListener)
	//停止
	Stop()
	//等待停止
	Wait()
	//设置矿工账号
	SetMiner(acc *Account) error
	//获取矿工账号
	GetMiner() *Account
	//获取区块头
	GetHeader() ([]byte, error)
	//设置区块头
	SetHeader(b []byte) error
	//重新开始计算区块
	ResetMiner() error
	//当前时间戳
	TimeNow() uint32
}

var (
	Miner = newMinerEngine()
)

type minerEngine struct {
	wg   sync.WaitGroup     //
	cctx context.Context    //
	cfun context.CancelFunc //
	acc  *Account
	mu   sync.RWMutex
	ogb  ONCE
	sch  chan bool        //停止当前正在创建的区块
	mbv  HeaderBytes      //正在处理的区块头数据
	mch  chan HeaderBytes //接收一个区块头数据进行验证
	lptr IListener
}

func newMinerEngine() IMiner {
	return &minerEngine{
		sch: make(chan bool, 1),
		mch: make(chan HeaderBytes, 1),
	}
}

func (m *minerEngine) GetMiner() *Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.acc
}

func (m *minerEngine) TimeNow() uint32 {
	return m.lptr.TimeNow()
}

//获取区块头
func (m *minerEngine) GetHeader() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ogb.IsRunning() {
		return nil, errors.New("miner not running")
	}
	return m.mbv, nil
}

func (m *minerEngine) ResetMiner() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.ogb.IsRunning() {
		return errors.New("miner not running")
	}
	ps := GetPubSub()
	ps.Pub(MinerAct{
		Opt: OptStopGenBlock,
		Arg: true,
	}, NewMinerActTopic)
	return nil
}

//设置区块头
func (m *minerEngine) SetHeader(b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.ogb.IsRunning() {
		return errors.New("miner not running")
	}
	if len(b) != blockheadersize {
		return errors.New("bytes len error")
	}
	ps := GetPubSub()
	ps.Pub(MinerAct{
		Opt: OptSendHeadBytes,
		Arg: HeaderBytes(b),
	}, NewMinerActTopic)
	return nil
}

func (m *minerEngine) SetMiner(acc *Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acc = acc
	return nil
}

//创建一个区块
func (m *minerEngine) genNewBlock(ver uint32) error {
	if !m.ogb.Running() {
		return nil
	}
	defer m.ogb.Reset()
	if conf.MinerNum == 0 {
		return errors.New("miner_num = 0,disable miner calc")
	}
	bi := GetBlockIndex()
	txp := bi.GetTxPool()
	blk, err := bi.NewBlock(ver)
	if err != nil {
		return err
	}
	err = blk.LoadTxs(bi)
	if err != nil {
		return err
	}
	//
	if err := blk.Finish(bi); err != nil {
		return err
	}
	LogInfof("gen new block add %d Tx, prev=%v cpu=%d", len(blk.Txs), blk.Meta.Prev, conf.MinerNum)
	m.mbv = blk.Header.Bytes()
	mg := NewMinerGroup(m.mbv, blk.Header.Bits, conf.MinerNum)
	defer mg.Stop()
	mg.Run()
	//打印定时器
	dt := time.NewTimer(time.Second * MINER_LOG_SECONDS)
	genok := false
	ptime := uint64(0)
	ps := GetPubSub()
	bch := ps.Sub(NewRecvBlockTopic, TxPoolDelTxTopic)
	defer ps.Unsub(bch)
finished:
	for !genok {
		select {
		case <-dt.C:
			ppv := uint64(0)
			smt := mg.Times()
			if ptime == 0 {
				ptime = smt
				ppv = ptime / MINER_LOG_SECONDS
			} else {
				ppv = (smt - ptime) / MINER_LOG_SECONDS
				ptime = smt
			}
			ts := time.Unix(int64(blk.Header.Time), 0).Format("2006-01-02 15:04:05")
			LogInfof("%d times/s, total=%d, bits=%08x time=%s height=%d txs=%d txp=%d cpu=%d cache=%d",
				ppv,
				ptime,
				blk.Header.Bits,
				ts, blk.Meta.Height,
				len(blk.Txs),
				txp.Len(),
				conf.MinerNum,
				bi.CacheSize(),
			)
			dt.Reset(time.Second * MINER_LOG_SECONDS)
		case <-mg.exit:
			if mg.ok {
				blk.Header = mg.bh
				genok = true
				break finished
			}
		case mbv := <-m.mch:
			if id := mbv.Hash(); CheckProofOfWork(id, blk.Header.Bits) {
				mg.StopAndWait()
				blk.Header = mbv.Header()
				genok = true
				break finished
			}
		case chv := <-bch:
			//如果交易池中的交易被删除，或者收到新的区块检测是否停止区块生成
			if rlk, ok := chv.(*BlockInfo); ok && rlk.Meta.Height >= blk.Meta.Height {
				mg.StopAndWait()
				return errors.New("new block recv,stop gen block")
			} else if tid, ok := chv.(HASH256); ok && blk.HasTx(tid) {
				mg.StopAndWait()
				return errors.New("tx pool removed,stop gen block")
			}
		case <-m.sch:
			mg.StopAndWait()
			return errors.New("force stop current gen block")
		case <-m.cctx.Done():
			mg.StopAndWait()
			return m.cctx.Err()
		}
	}
	if !genok {
		return errors.New("miner gen block failed")
	}
	if bi.Len() == 0 {
		buf := NewWriter()
		_ = blk.Encode(buf)
		ioutil.WriteFile("genesis.blk", buf.Bytes(), 0644)
		LogInfo("save first block")
	}
	LogInfo("gen new block success, id = ", blk)
	if err = bi.LinkBlk(blk); err != nil {
		LogError("link new block error", err)
		return err
	}
	//广播更新了区块数据
	ps.Pub(blk, NewLinkBlockTopic)
	//广播区块
	msg := NewMsgBlock(blk)
	msg.AddFlags(MsgBlockNewFlags)
	Server.BroadMsg(msg)
	return nil
}

//处理操作
func (m *minerEngine) processOpt(opt MinerAct) {
	switch opt.Opt {
	case OptSendHeadBytes:
		if bh, ok := opt.Arg.(HeaderBytes); ok {
			m.mch <- bh
		}
	case OptStopGenBlock:
		m.sch <- true
	case OptGenBlock:
		ver, ok := opt.Arg.(uint32)
		if !ok {
			LogError("OptGetBlock args type error", opt.Arg)
			break
		}
		if acc := m.GetMiner(); acc != nil {
			err := m.genNewBlock(ver)
			if err != nil {
				LogError("gen new block error", err)
			}
		} else {
			LogError("miner account not set,gen new block error")
		}
	case OptSetMiner:
		acc, ok := opt.Arg.(*Account)
		if !ok {
			LogError("OptGetBlock args type error", opt.Arg)
			break
		}
		if err := m.SetMiner(acc); err != nil {
			LogError("set miner error", err)
		}
	default:
		LogError("unknow opt type,no process", opt)
	}
}

func (m *minerEngine) recoverError() {
	if gin.Mode() == gin.DebugMode {
		m.cfun()
		return
	}
	if err := recover(); err != nil {
		m.cfun()
		LogError(err)
	}
}

func (m *minerEngine) loop(i int, ch chan interface{}, dt *time.Timer) {
	LogInfo("miner worker", i, "start")
	defer m.recoverError()
	m.wg.Add(1)
	defer m.wg.Done()
	for {
		select {
		case op := <-ch:
			if opv, ok := op.(MinerAct); ok {
				m.processOpt(opv)
			} else {
				LogError("dispatch recv error opt", op)
			}
		case <-dt.C:
			if acc := m.GetMiner(); acc == nil {
				LogError("miner acc not set,can't gen new block")
			} else if err := m.genNewBlock(1); err != nil {
				LogError("gen new block error", err)
			}
			if conf.MinerNum > 0 {
				dt.Reset(time.Second * 30)
			}
		case <-m.cctx.Done():
			return
		}
	}
}

//开始工作
func (m *minerEngine) Start(ctx context.Context, lis IListener) {
	m.lptr = lis
	ps := GetPubSub()
	m.cctx, m.cfun = context.WithCancel(ctx)
	//订阅矿工操作
	ch := ps.Sub(NewMinerActTopic)
	//每隔30秒开始自动创建区块
	dt := time.NewTimer(time.Second * 30)
	for i := 0; i < 4; i++ {
		go m.loop(i, ch, dt)
	}
}

//停止
func (m *minerEngine) Stop() {
	m.cfun()
}

//等待停止
func (m *minerEngine) Wait() {
	m.wg.Wait()
}
