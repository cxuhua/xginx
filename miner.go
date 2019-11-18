package xginx

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	//新交易订阅主体 *TX
	NewTxTopic = "NewTx"
	//新区块订阅 *Block
	NewBlockTopic = "NewBlock"
	//矿工操作 MinerAct
	NewMinerActTopic = "NewMinerAct"
	//链上新连接了区块
	UpdateBlockTopic = "UpdateBlock"
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
	mbc    chan uint32 //
	gening ONE         //
	mu     sync.RWMutex
}

func newMinerEngine() IMiner {
	return &minerEngine{
		mbc: make(chan uint32, 1),
	}
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
func (m *minerEngine) genBlock(ver uint32) {
	if !m.gening.Running() {
		return
	}
	defer m.gening.Reset()
	bi := GetBlockIndex()
	blk, err := bi.NewBlock(ver)
	if err != nil {
		LogError("new block error ", err)
		return
	}
	if err := blk.Finish(bi); err != nil {
		LogError("finish block error ", err)
		return
	}
	err = blk.Check(bi, false)
	if err != nil {
		LogError("check block error ", err)
		return
	}
	genok := false
	hb := blk.Header.Bytes()
	times := uint32(opt.MiB * 10)
	for i, j := UR32(), uint32(0); ; i++ {
		if err := m.ctx.Err(); err != nil {
			LogError("gen block ctx err igonre", err)
			break
		}
		id := hb.Hash()
		if !CheckProofOfWork(id, blk.Header.Bits) {
			j++
			hb.SetNonce(i)
		} else {
			blk.Header = hb.Header()
			genok = true
			break
		}
		if j%times == 0 {
			LogInfo("genblock %d times, bits=%x id=%v nonce=%x height=%d\n", times, blk.Meta.Bits, id, i, blk.Meta.Height)
			i = UR32()
			j = 0
		}
		if i > (^uint32(0))-1 {
			hb.SetTime(time.Now())
			i = UR32()
		}
	}
	if !genok {
		LogError("get block not finish")
		return
	}
	LogInfo("gen block ok id = ", blk)
	if err := bi.LinkHeader(blk.Header); err != nil {
		LogError("new block linkto chain error ", err)
		return
	}
	if err := bi.UpdateBlk(blk); err != nil {
		LogError("new block linkto chain error ", err)
		return
	}
	LogInfo("new block linkto chain success id=", blk)
}

//处理操作
func (m *minerEngine) processOpt(opt MinerAct) {
	switch opt.Opt {
	case OptGetBlock:
		ver, ok := opt.Arg.(uint32)
		if !ok {
			LogError("OptGetBlock args type error", opt.Arg)
			break
		}
		m.mu.RLock()
		if m.acc != nil {
			m.mbc <- ver
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
	wtimer := time.NewTimer(time.Second * 60)
	for {
		select {
		case op := <-ch:
			if opv, ok := op.(MinerAct); ok {
				m.processOpt(opv)
			} else {
				LogError("dispatch recv error opt", op)
			}
		case <-wtimer.C:
			//m.NewBlock(1)
			wtimer.Reset(time.Second * 60)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *minerEngine) onRecvTx(tx *TX) {
	bi := GetBlockIndex()
	err := tx.Check(bi, true)
	if err != nil {
		LogError("tx check error", err, "drop tx")
		return
	}
	tp := bi.GetTxPool()
	err = tp.PushBack(tx)
	if err != nil {
		LogError("tx push to pool error", err, "drop tx")
		return
	}
	LogInfo("current txpool len=", tp.Len())
}

func (m *minerEngine) onRecvBlock(blk *BlockInfo) {
	bi := GetBlockIndex()
	if err := bi.UpdateBlk(blk); err != nil {
		LogError("link block error", err, "drop block", blk)
		return
	}
	LogInfo("new block link to chain", blk)
}

func (m *minerEngine) loop(i int, wch chan interface{}) {
	LogInfo("miner worker", i, "start")
	m.wg.Add(1)
	defer m.wg.Done()
	for {
		select {
		case ptr := <-wch:
			if tx, ok := ptr.(*TX); ok {
				m.onRecvTx(tx)
			} else if blk, ok := ptr.(*BlockInfo); ok {
				m.onRecvBlock(blk)
			} else {
				LogError("recv unknow msg", ptr)
			}
		case ver := <-m.mbc:
			m.genBlock(ver)
		case <-m.ctx.Done():
			return
		}
	}
}

//开始工作
func (m *minerEngine) Start(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
	//订阅交易和区块
	wch := GetPubSub().Sub(NewTxTopic, NewBlockTopic)
	for i := 0; i < 4; i++ {
		go m.loop(i, wch)
	}
	//订阅矿工操作
	optch := GetPubSub().Sub(NewMinerActTopic)
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
