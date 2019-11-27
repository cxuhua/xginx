package xginx

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	//矿工操作 MinerAct
	NewMinerActTopic = "NewMinerAct"
	//链上新连接了区块 BlockHeader
	NewLinkHeaderTopic = "NewLinkHeader"
	//更新了一个区块数据 BlockInfo
	NewUpdateBlockTopic = "NewUpdateBlock"
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
}

var (
	Miner = newMinerEngine()
)

type minerEngine struct {
	wg     sync.WaitGroup     //
	ctx    context.Context    //
	cancel context.CancelFunc //
	acc    *Account
	mu     sync.RWMutex
	ogb    ONCE
	sch    chan bool        //停止当前正在创建的区块
	mbv    HeaderBytes      //正在处理的区块头数据
	mch    chan HeaderBytes //接收一个区块头数据进行验证
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
	ps := GetPubSub()
	bi := GetBlockIndex()
	blk, err := bi.NewBlock(ver)
	if err != nil {
		return err
	}
	txp := bi.GetTxPool()
	//添加交易
	txs, err := txp.GetTxs(bi, blk)
	if err != nil {
		return err
	}
	if len(txs) > 0 {
		err = blk.AddTxs(bi, txs)
	}
	if err != nil {
		return err
	}
	LogInfof("gen new block add %d Tx, prev=%v ", len(txs), blk.Meta.Prev)
	//
	if err := blk.Finish(bi); err != nil {
		return err
	}
	err = blk.Check(bi, false)
	if err != nil {
		return err
	}
	m.mbv = blk.Header.Bytes()
	//次数
	times := uint32(opt.MiB * 10)
	//当一个新块头，并且比当前区块高时取消当前进度
	hbc := ps.Sub(NewLinkHeaderTopic)
	defer ps.Unsub(hbc)
	for i, j, l := UR32(), uint32(0), 0; ; i++ {
		select {
		case mbv := <-m.mch:
			m.mbv = mbv
			LogInfo("recv block header bytes", hex.EncodeToString(mbv))
		case <-m.sch:
			return errors.New("force stop current gen block")
		case <-m.ctx.Done():
			return m.ctx.Err()
		case bhp := <-hbc:
			ele, ok := bhp.(*TBEle)
			if !ok {
				LogError("NewBlockHeaderTopic recv error data", bhp)
				break
			}
			if ele.Height < blk.Meta.Height {
				break
			}
			return errors.New("recv new block header ,ignore gen block")
		default:
			break
		}
		if id := m.mbv.Hash(); !CheckProofOfWork(id, blk.Header.Bits) {
			j++
			m.mbv.SetNonce(i)
		} else {
			blk.Header = m.mbv.Header()
			break
		}
		if j%times == 0 {
			l++
			j = 0
			LogInfof("%d*%d times , bits=%08x time=%08x nonce=%08x height=%d txs=%d", l, times, blk.Header.Bits, blk.Header.Time, i, blk.Meta.Height, len(txs))
		}
		//重新设置时间和随机数
		if i >= ^uint32(0) {
			blk.Header.Time = uint32(time.Now().Unix())
			m.mbv.SetTime(time.Now())
			i = UR32()
		}
	}
	LogInfo("gen new block success, id = ", blk)
	if _, err := bi.LinkHeader(blk.Header); err != nil {
		LogError("link new block header error", err)
		return err
	}
	if err = bi.UpdateBlk(blk); err != nil {
		LogError("update new block error unlink", err)
		return bi.UnlinkLast()
	}
	//广播更新了区块数据
	ps.Pub(blk, NewUpdateBlockTopic)
	//广播区块头
	msg := bi.NewMsgHeaders(blk.Header)
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
		LogInfo("recv stop current gen block")
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
		return
	}
	if err := recover(); err != nil {
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
			dt.Reset(time.Second * 10)
		case <-m.ctx.Done():
			return
		}
	}
}

//开始工作
func (m *minerEngine) Start(ctx context.Context, lis IListener) {
	ps := GetPubSub()
	m.ctx, m.cancel = context.WithCancel(ctx)
	//订阅矿工操作
	ch := ps.Sub(NewMinerActTopic)
	//每隔10秒开始自动创建区块
	dt := time.NewTimer(time.Second * 10)
	for i := 0; i < 4; i++ {
		go m.loop(i, ch, dt)
	}
}

//停止
func (m *minerEngine) Stop() {
	m.cancel()
}

//等待停止
func (m *minerEngine) Wait() {
	m.wg.Wait()
}
