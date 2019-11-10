package xginx

import (
	"context"
	"errors"
	"log"
	"sync"
)

//矿工接口
type IMiner interface {
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
	Miner IMiner = newMinerEngine()
)

type minerEngine struct {
	wg     sync.WaitGroup
	mpk    *PublicKey
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
