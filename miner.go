package xginx

import (
	"context"
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
	if !m.ogb.Running() {
		return errors.New("gening block")
	}
	defer m.ogb.Reset()
	ps := GetPubSub()
	bi := GetBlockIndex()
	blk, err := bi.NewBlock(ver)
	if err != nil {
		return err
	}
	//添加交易
	txs, err := bi.GetTxPool().GetTxs()
	if err != nil {
		return err
	}
	if len(txs) > 0 {
		err = blk.AddTxs(bi, txs)
	}
	if err != nil {
		return err
	}
	LogInfof("gen new block add %d Tx", len(txs))
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
	//当一个新块头，并且比当前区块高时取消当前进度
	hbc := ps.Sub(NewLinkHeaderTopic)
	defer ps.Unsub(hbc)
	for i, j := UR32(), uint32(0); ; i++ {
		select {
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
	if _, err := bi.LinkHeader(blk.Header); err != nil {
		return err
	}
	if err = bi.UpdateBlk(blk); err != nil {
		LogError("update blk error unlink", err)
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

func (m *minerEngine) loop(i int, ch chan interface{}) {
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
	for i := 0; i < 4; i++ {
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
