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
		uxs: map[HashID]*Unit{},
		txs: map[HashID]*TX{},
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
	uxs    map[HashID]*Unit
	umu    sync.RWMutex
	txs    map[HashID]*TX
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
func (m *minerEngine) calcdata(db DBImp) error {
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
			err := store.UseSession(m.ctx, func(db DBImp) error {
				return m.calcdata(db)
			})
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
