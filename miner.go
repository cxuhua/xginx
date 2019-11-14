package xginx

import (
	"context"
	"log"
	"sync"
	"time"
)

//矿工接口
type IMiner interface {
	//开始工作
	Start(ctx context.Context)
	//停止
	Stop()
	//等待停止
	Wait()
	//创建新区块
	NewBlock(ver uint32)
}

var (
	Miner = newMinerEngine()
)

type minerEngine struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	mbc    chan uint32
	gening ONE
}

func newMinerEngine() IMiner {
	return &minerEngine{
		mbc: make(chan uint32, 1),
	}
}

//创建新块任务
func (m *minerEngine) NewBlock(ver uint32) {
	m.mbc <- ver
}

//创建一个区块
func (m *minerEngine) genBlock(ver uint32) {
	if !m.gening.Running() {
		return
	}
	defer m.gening.Reset()
	bi := GetChain()
	blk, err := bi.NewBlock(ver)
	if err != nil {
		log.Println("new block error ", err)
		return
	}
	if err := blk.Finish(bi); err != nil {
		log.Println("finish block error ", err)
		return
	}
	err = blk.Check(bi, false)
	if err != nil {
		log.Println("check block error ", err)
		return
	}
	genok := false
	hb := blk.Header.Bytes()
	r := uint32(0)
	SetRandInt(&r)
	for i := uint32(0); ; i++ {
		if err := m.ctx.Err(); err != nil {
			log.Println("gen block ctx err igonre", err)
			break
		}
		id := hb.Hash()
		if !CheckProofOfWork(id, blk.Header.Bits) {
			hb.SetNonce(i)
		} else {
			blk.Header = hb.Header()
			genok = true
			break
		}
		if i%10000000 == 0 {
			log.Printf("genblock 10000000 bits=%x ID=%v Nonce=%x Height=%d\n", blk.Meta.Bits, id, i, blk.Meta.Height)
		}
		if i > (^uint32(0))-1 {
			hb.SetTime(time.Now())
			i = 0
		}
	}
	if !genok {
		log.Println("get block not finish")
		return
	}
	log.Println("gen block ok id = ", blk.ID())
	if err := bi.LinkTo(blk); err != nil {
		log.Println("new block linkto chain error ", err)
		return
	}
	log.Println("new block linkto chain success id=", blk.ID())
}

//定时分配工作
func (m *minerEngine) dispatch() {
	log.Println("miner dispatch worker start")
	m.wg.Add(1)
	defer m.wg.Done()
	wtimer := time.NewTimer(time.Second * 60)
	for {
		select {
		case <-wtimer.C:
			m.NewBlock(1)
			wtimer.Reset(time.Second * 60)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *minerEngine) loop(i int) {
	log.Println("miner worker", i, "start")
	m.wg.Add(1)
	defer m.wg.Done()
	for {
		select {
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
	for i := 0; i < 4; i++ {
		go m.loop(i)
	}
	go m.dispatch()
}

//停止
func (m *minerEngine) Stop() {
	m.cancel()
}

//等待停止
func (m *minerEngine) Wait() {
	m.wg.Wait()
}
