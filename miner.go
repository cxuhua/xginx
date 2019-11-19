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
	//开始创建区块
	NewGenBlockTopic = "NewGenBlock"
)

const (
	//开始挖矿操作 args(uint32) = block ver
	OptGetBlock = iota
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
}

func newMinerEngine() IMiner {
	return &minerEngine{}
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
func (m *minerEngine) genBlock(ver uint32) error {
	ps := GetPubSub()
	bi := GetBlockIndex()
	blk, err := bi.NewBlock(ver)
	if err != nil {
		return err
	}
	//添加交易
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
			LogInfof("genblock %d times, bits=%x id=%v nonce=%x height=%d", times, blk.Meta.Bits, id, i, blk.Meta.Height)
			i = UR32()
			j = 0
		}
		if i > (^uint32(0))-1 {
			hb.SetTime(time.Now())
			i = UR32()
		}
	}
	LogInfo("gen block ok id = ", blk)
	err = bi.LinkHeader(blk.Header)
	if err != nil {
		return err
	}
	err = bi.UpdateBlk(blk)
	if err != nil {
		err = bi.UnlinkLast()
	}
	if err != nil {
		return err
	}
	Server.BroadMsg(nil, NewMsgBlock(blk))
	return nil
}

//处理操作
func (m *minerEngine) processOpt(opt MinerAct) {
	ps := GetPubSub()
	switch opt.Opt {
	case OptGetBlock:
		ver, ok := opt.Arg.(uint32)
		if !ok {
			LogError("OptGetBlock args type error", opt.Arg)
			break
		}
		m.mu.RLock()
		if m.acc != nil {
			ps.Pub(ver, NewGenBlockTopic)
		} else {
			LogError("miner account not set,new block error")
		}
		m.mu.RUnlock()
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

//定时分配工作
func (m *minerEngine) dispatch(ch chan interface{}) {
	LogInfo("miner dispatch worker start")
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

func (m *minerEngine) loop(i int, wch chan interface{}) {
	LogInfo("miner worker", i, "start")
	m.wg.Add(1)
	defer m.wg.Done()
	for {
		select {
		case ptr := <-wch:
			switch ptr.(type) {
			case uint32:
				ver := ptr.(uint32)
				LogInfo("recv NewGenBlock Topic,start gen block,ver =", ver)
				err := m.genBlock(ver)
				if err != nil {
					LogError("gen block error", err)
				}
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
	//订阅交易和区块
	wch := ps.Sub(NewGenBlockTopic)
	for i := 0; i < 4; i++ {
		go m.loop(i, wch)
	}
	//订阅矿工操作
	optch := ps.Sub(NewMinerActTopic)
	go m.dispatch(optch)
}

//停止
func (m *minerEngine) Stop() {
	m.cancel()
}

//等待停止
func (m *minerEngine) Wait() {
	m.wg.Wait()
}
