package xginx

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	//矿工操作 MinerAct
	NewMinerActTopic = "NewMinerAct"
	//链上新连接了区块
	NewBlockLinkTopic = "NewBlockLink"
)

const (
	//开始挖矿操作 args(uint32) = block ver
	OptGenBlock = iota
	//设置矿工奖励账号 arg=*Account
	OptSetMiner
)

//矿产操作
type MinerAct struct {
	Opt int
	Arg interface{}
}

//矿工接口
type IMiner interface {
	//开始工作
	Start(ctx context.Context)
	//停止
	Stop()
	//等待停止
	Wait()
	//设置矿工账号
	SetMiner(acc *Account) error
	//获取矿工账号
	GetMiner() *Account
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
	single sync.Mutex
}

func newMinerEngine() IMiner {
	return &minerEngine{}
}

func (m *minerEngine) GetMiner() *Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.acc
}

func (m *minerEngine) SetMiner(acc *Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !acc.HasPrivate() {
		return errors.New("miner account must has privatekey")
	}
	m.acc = acc
	return nil
}

//创建一个区块
func (m *minerEngine) genNewBlock(ver uint32) error {
	m.single.Lock()
	defer m.single.Unlock()
	ps := GetPubSub()
	bi := GetBlockIndex()
	blk, err := bi.NewBlock(ver)
	if err != nil {
		return err
	}
	//添加交易
	txs, err := bi.tp.GetTxs()
	if err != nil {
		return err
	}
	if len(txs) > 0 {
		err = blk.AddTxs(bi, txs)
	}
	if err != nil {
		return err
	}
	//
	if err := blk.Finish(bi); err != nil {
		return err
	}
	err = blk.Check(bi, false)
	if err != nil {
		return err
	}
	hb := blk.Header.Bytes()
	times := uint32(opt.MiB * 10)
	//当一个新块更新到库中时，终止当前的的区块进度
	linkblk := ps.SubOnce(NewBlockLinkTopic)
	defer ps.Unsub(linkblk)
	for i, j := UR32(), uint32(0); ; i++ {
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()
		case <-linkblk:
			return errors.New("recv new block ,ignore curr block")
		default:
			break
		}
		id := hb.Hash()
		if !CheckProofOfWork(id, blk.Header.Bits) {
			j++
			hb.SetNonce(i)
		} else {
			blk.Header = hb.Header()
			break
		}
		if j%times == 0 {
			LogInfof("gen new block %d times, bits=%x id=%v nonce=%x height=%d", times, blk.Meta.Bits, id, i, blk.Meta.Height)
			i = UR32()
			j = 0
		}
		if i > (^uint32(0))-1 {
			hb.SetTime(time.Now())
			i = UR32()
		}
	}
	LogInfo("gen new block ok id = ", blk)
	if err = bi.LinkHeader(blk.Header); err != nil {
		return err
	}
	if err = bi.UpdateBlk(blk); err != nil {
		err = bi.UnlinkLast()
	}
	if err != nil {
		return err
	}
	ps.Pub(blk, NewBlockLinkTopic)
	Server.BroadMsg(nil, NewMsgBlock(blk))
	return nil
}

//处理操作
func (m *minerEngine) processOpt(opt MinerAct) {
	switch opt.Opt {
	case OptGenBlock:
		ver, ok := opt.Arg.(uint32)
		if !ok {
			LogError("OptGetBlock args type error", opt.Arg)
			break
		}
		m.mu.RLock()
		accok := m.acc != nil
		m.mu.RUnlock()
		if accok {
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
		m.mu.Lock()
		m.acc = acc
		m.mu.Unlock()
	default:
		LogError("unknow opt type,no process", opt)
	}
}

func (m *minerEngine) loop(i int, ch chan interface{}) {
	LogInfo("miner worker", i, "start")
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
		case <-m.ctx.Done():
			return
		}
	}
}

//开始工作
func (m *minerEngine) Start(ctx context.Context) {
	ps := GetPubSub()
	m.ctx, m.cancel = context.WithCancel(ctx)
	//订阅矿工操作
	ch := ps.Sub(NewMinerActTopic)
	for i := 0; i < 2; i++ {
		go m.loop(i, ch)
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
