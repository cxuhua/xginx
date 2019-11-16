package xginx

import (
	"context"
	"errors"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	//新交易订阅主体
	NewTxTopic = "NewTx"
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
	//获取订阅发布接口
	GetPubSub() *PubSub
	//设置矿工账号
	SetMiner(acc *Account) error
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
	pubsub *PubSub
	mu     sync.RWMutex
}

func newMinerEngine() IMiner {
	return &minerEngine{
		pubsub: NewPubSub(5),
		mbc:    make(chan uint32, 1),
	}
}

func (m *minerEngine) GetPubSub() *PubSub {
	return m.pubsub
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

//创建新块任务
func (m *minerEngine) NewBlock(ver uint32) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.acc == nil {
		log.Println("miner account not set,new block error")
		return
	}
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
	times := uint32(opt.MiB * 10)
	for i, j := UR32(), uint32(0); ; i++ {
		if err := m.ctx.Err(); err != nil {
			log.Println("gen block ctx err igonre", err)
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
			log.Printf("genblock %d times, bits=%x id=%v nonce=%x height=%d\n", times, blk.Meta.Bits, id, i, blk.Meta.Height)
			i = UR32()
			j = 0
		}
		if i > (^uint32(0))-1 {
			hb.SetTime(time.Now())
			i = UR32()
		}
	}
	if !genok {
		log.Println("get block not finish")
		return
	}
	log.Println("gen block ok id = ", blk)
	if err := bi.LinkTo(blk); err != nil {
		log.Println("new block linkto chain error ", err)
		return
	}
	log.Println("new block linkto chain success id=", blk)
}

//定时分配工作
func (m *minerEngine) dispatch() {
	log.Println("miner dispatch worker start")
	m.wg.Add(1)
	defer m.wg.Done()
	defer func() {
		m.pubsub.Shutdown()
	}()
	wtimer := time.NewTimer(time.Second * 60)
	for {
		select {
		case <-wtimer.C:
			//m.NewBlock(1)
			wtimer.Reset(time.Second * 60)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *minerEngine) onRecvTx(tx *TX) {
	chain := GetChain()
	err := tx.Check(chain, true)
	if err != nil {
		log.Println("tx check error", err, "drop tx")
		return
	}
	tp := chain.GetTxPool()
	err = tp.PushBack(tx)
	if err != nil {
		log.Println("tx push to pool error", err, "drop tx")
		return
	}
	log.Println("current txpool len=", tp.Len())
}

func (m *minerEngine) loop(i int, txch chan interface{}) {
	log.Println("miner worker", i, "start")
	m.wg.Add(1)
	defer m.wg.Done()
	for {
		select {
		case cv := <-txch:
			tx, ok := cv.(*TX)
			if ok {
				m.onRecvTx(tx)
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
	cpu := runtime.NumCPU()
	m.ctx, m.cancel = context.WithCancel(ctx)
	//订阅交易
	txch := m.pubsub.Sub(NewTxTopic) //*TX
	for i := 0; i < cpu; i++ {
		go m.loop(i, txch)
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
