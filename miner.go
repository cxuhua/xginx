package xginx

import (
	"context"
	"log"
	"sync"
)

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
	Miner IMiner = newMinerEngine()
)

type minerEngine struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	tc     chan *TX
	umu    sync.RWMutex
	txs    map[HASH256]*TX
	tmu    sync.RWMutex
	dok    chan bool
}

func newMinerEngine() IMiner {
	return &minerEngine{
		tc:  make(chan *TX, 50),
		txs: map[HASH256]*TX{},
		dok: make(chan bool),
	}
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
		case tv := <-m.tc:
			m.tmu.RLock()
			_, ok := m.txs[tv.Hash()]
			m.tmu.RUnlock()
			if ok {
				continue
			}
			m.tmu.Lock()
			m.txs[tv.Hash()] = tv
			m.tmu.Unlock()
			m.dok <- true
			log.Println("recv tx hash=", tv.Hash())
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
}

//停止
func (m *minerEngine) Stop() {
	m.cancel()
}

//等待停止
func (m *minerEngine) Wait() {
	m.wg.Wait()
}
