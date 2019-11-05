package xginx

import (
	"context"
	"errors"
	"log"
	"sync"
)

//矿工接口
type IMiner interface {
	//收到单元块
	OnUnit(b *Unit)
	//收到交易数据
	OnTx(tx *TX)
	//开始工作
	Start(ctx context.Context)
	//停止
	Stop()
	//等待停止
	Wait()
}

var (
	Miner IMiner = &minerEngine{
		uc:  make(chan *Unit, 50),
		tc:  make(chan *TX, 50),
		uxs: map[HASH256]*Unit{},
		txs: map[HASH256]*TX{},
		dok: make(chan bool),
	}
)

type minerEngine struct {
	wg     sync.WaitGroup
	mpk    *PublicKey
	ctx    context.Context
	cancel context.CancelFunc
	uc     chan *Unit
	tc     chan *TX
	uxs    map[HASH256]*Unit
	umu    sync.RWMutex
	txs    map[HASH256]*TX
	tmu    sync.RWMutex
	dok    chan bool
}

//收到有效的单元块
func (m *minerEngine) OnUnit(b *Unit) {
	m.uc <- b
}

//收到有效的交易数据
func (m *minerEngine) OnTx(tx *TX) {
	m.tc <- tx
}

//计算打包数据
func (m *minerEngine) calcdata() error {
	return nil
}

func (m *minerEngine) loop(i int) {
	log.Println("miner worker", i, "start")
	defer func() {
		if err := recover(); err != nil {
			log.Println("miner worker error id=", i, err)
			m.cancel()
		}
	}()
	m.wg.Add(1)
	defer m.wg.Done()
	for {
		select {
		case <-m.dok:
			err := m.calcdata()
			if err != nil {
				log.Println("calcdata error", err)
			}
		case ux := <-m.uc:
			m.umu.RLock()
			_, ok := m.uxs[ux.Hash()]
			m.umu.RUnlock()
			if !ok {
				continue
			}
			m.umu.Lock()
			m.uxs[ux.Hash()] = ux
			m.umu.Unlock()
			m.dok <- true
			log.Println("recv unit hash=", ux.Hash())
		case tx := <-m.tc:
			m.tmu.RLock()
			_, ok := m.txs[tx.Hash()]
			m.tmu.RUnlock()
			if ok {
				continue
			}
			m.tmu.Lock()
			m.txs[tx.Hash()] = tx
			m.tmu.Unlock()
			m.dok <- true
			log.Println("recv tx hash=", tx.Hash())
		case <-m.ctx.Done():
			return
		}
	}
}

//开始工作
func (m *minerEngine) Start(ctx context.Context) {
	m.mpk = conf.GetMinerPubKey()
	if m.mpk == nil {
		panic(errors.New("miner pubkey miss"))
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	for i := 0; i < 4; i++ {
		go m.loop(i)
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
