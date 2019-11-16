package xginx

import (
	"bytes"
	"container/list"
	"errors"
	"sync"
)

//交易池，存放签名成功，未确认的交易
//当区块连接后需要把区块中的交易从这个池子删除
type TxPool struct {
	mu   sync.RWMutex
	tlis *list.List
	tmap map[HASH256]*list.Element
}

func NewTxPool() *TxPool {
	return &TxPool{
		tlis: list.New(),
		tmap: map[HASH256]*list.Element{},
	}
}

//返回非空是移除的交易
func (p *TxPool) Del(id HASH256) *TX {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ele, has := p.tmap[id]; has {
		tx := ele.Value.(*TX)
		p.tlis.Remove(ele)
		delete(p.tmap, id)
		return tx
	}
	return nil
}

//移除多个交易
func (p *TxPool) DelTxs(txs []*TX) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	//检测错误
	for _, tx := range txs {
		_, err := tx.ID()
		if err != nil {
			return err
		}
	}
	//开始移除
	for _, tx := range txs {
		id, err := tx.ID()
		if err != nil {
			continue
		}
		ele, has := p.tmap[id]
		if !has {
			continue
		}
		p.tlis.Remove(ele)
		delete(p.tmap, id)
	}
	return nil
}

//取出交易，大小不能超过限制
func (p *TxPool) GetTxs() ([]*TX, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	txs := []*TX{}
	size := 0
	buf := &bytes.Buffer{}
	//获取用来打包区块的交易
	for cur := p.tlis.Front(); cur != nil; cur = cur.Next() {
		buf.Reset()
		tx := cur.Value.(*TX)
		err := tx.Encode(buf)
		if err != nil {
			return nil, err
		}
		size += buf.Len()
		if size > MAX_BLOCK_SIZE {
			break
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

//一笔钱是否已经在池中某个交易消费
func (p *TxPool) FindCoin(coin *CoinKeyValue) (*TX, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, ele := range p.tmap {
		tx := ele.Value.(*TX)
		for _, in := range tx.Ins {
			if !in.OutHash.Equal(coin.TxId) {
				continue
			}
			if in.OutIndex != coin.Index {
				continue
			}
			return tx, nil
		}
	}
	return nil, errors.New("txpool not found coin")
}

func (p *TxPool) Get(id HASH256) (*TX, error) {
	p.mu.RUnlock()
	defer p.mu.RUnlock()
	if ele, has := p.tmap[id]; has {
		return ele.Value.(*TX), nil
	}
	return nil, errors.New("txpool not found tx")
}

func (p *TxPool) Len() int {
	p.mu.RUnlock()
	defer p.mu.RUnlock()
	return p.tlis.Len()
}

//添加进去一笔交易放入最后
//交易必须是实现校验过的
func (p *TxPool) PushBack(tx *TX) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	id, err := tx.ID()
	if err != nil {
		return err
	}
	if _, has := p.tmap[id]; has {
		return errors.New("tx exists")
	}
	ele := p.tlis.PushBack(tx)
	p.tmap[id] = ele
	return nil
}