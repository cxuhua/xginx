package xginx

import (
	"container/list"
	"errors"
	"sync"

	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/syndtr/goleveldb/leveldb/comparer"

	"github.com/syndtr/goleveldb/leveldb/memdb"
)

const (
	MAX_TX_POOL_SIZE = 4096 * 4
)

//交易池，存放签名成功，未确认的交易
//当区块连接后需要把区块中的交易从这个池子删除
type TxPool struct {
	mu   sync.RWMutex
	tlis *list.List
	tmap map[HASH256]*list.Element
	mdb  *memdb.DB
}

func NewTxPool() *TxPool {
	return &TxPool{
		tlis: list.New(),
		tmap: map[HASH256]*list.Element{},
		mdb:  memdb.New(comparer.DefaultComparer, 1024*4),
	}
}

func (p *TxPool) Close() {
	p.mdb.Reset()
}

//返回非空是移除的交易
func (p *TxPool) Del(id HASH256) *TX {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ele, has := p.tmap[id]; has {
		tx := ele.Value.(*TX)
		_ = p.setMemIdx(tx, false)
		p.tlis.Remove(ele)
		delete(p.tmap, id)
		return tx
	}
	return nil
}

//当交易加入交易池
func (p *TxPool) setMemIdx(tx *TX, add bool) error {
	tid, err := tx.ID()
	if err != nil {
		return err
	}
	tx.pool = add
	//存储已经消费的输出
	buf := NewWriter()
	for _, in := range tx.Ins {
		buf.Reset()
		err := in.OutHash.Encode(buf)
		if err != nil {
			return err
		}
		err = in.OutIndex.Encode(buf)
		if err != nil {
			return err
		}
		if add {
			err = p.mdb.Put(buf.Bytes(), tid[:])
		} else {
			err = p.mdb.Delete(buf.Bytes())
		}
		if err != nil {
			return err
		}
	}
	//存储可用的金额
	for idx, out := range tx.Outs {
		tk := &CoinKeyValue{}
		pkh, err := out.Script.GetPkh()
		if err != nil {
			return err
		}
		tk.Value = out.Value
		tk.CPkh = pkh
		tk.Index = VarUInt(idx)
		tk.TxId = tid
		if add {
			err = p.mdb.Put(tk.GetKey(), tk.GetValue())
		} else {
			err = p.mdb.Delete(tk.GetKey())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

//移除多个交易
func (p *TxPool) DelTxs(txs []*TX) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, tx := range txs {
		id, err := tx.ID()
		if err != nil {
			continue
		}
		ele, has := p.tmap[id]
		if !has {
			continue
		}
		_ = p.setMemIdx(tx, false)
		p.tlis.Remove(ele)
		delete(p.tmap, id)
	}
	return nil
}

//获取所有的tx
func (p *TxPool) AllTxs() []*TX {
	p.mu.RLock()
	defer p.mu.RUnlock()
	txs := []*TX{}
	//获取用来打包区块的交易
	for cur := p.tlis.Front(); cur != nil; cur = cur.Next() {
		tx := cur.Value.(*TX)
		txs = append(txs, tx)
	}
	return txs
}

//取出交易，大小不能超过限制
func (p *TxPool) GetTxs(bi *BlockIndex, blk *BlockInfo) ([]*TX, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	txs := []*TX{}
	size := 0
	buf := NewWriter()
	//获取用来打包区块的交易
	for cur := p.tlis.Front(); cur != nil; cur = cur.Next() {
		buf.Reset()
		tx := cur.Value.(*TX)
		//未到达时间不获取
		err := tx.CheckLockTime(blk)
		if err != nil {
			continue
		}
		err = tx.Check(bi, true)
		if err != nil {
			continue
		}
		err = tx.Encode(buf)
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

//是否存在可消费的coin
func (p *TxPool) HasCoin(coin *CoinKeyValue) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mdb.Contains(coin.GetKey())
}

//获取pkh在交易池中可用的金额
func (p *TxPool) ListCoins(spkh HASH160) (Coins, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	coins := Coins{}
	key := append([]byte{}, COIN_PREFIX...)
	key = append(key, spkh[:]...)
	iter := p.mdb.NewIterator(util.BytesPrefix(key))
	defer iter.Release()
	for iter.Next() {
		tk := &CoinKeyValue{}
		err := tk.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		tk.pool = true
		coins = append(coins, tk)
	}
	return coins, nil
}

//一笔钱是否已经在内存交易池中某个交易消费
func (p *TxPool) IsSpentCoin(coin *CoinKeyValue) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	buf := NewWriter()
	err := coin.TxId.Encode(buf)
	if err != nil {
		return false
	}
	err = coin.Index.Encode(buf)
	if err != nil {
		return false
	}
	return p.mdb.Contains(buf.Bytes())
}

//交易池是否存在某个交易
func (p *TxPool) Has(id HASH256) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, has := p.tmap[id]
	return has
}

func (p *TxPool) Get(id HASH256) (*TX, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if ele, has := p.tmap[id]; has {
		return ele.Value.(*TX), nil
	}
	return nil, errors.New("txpool not found tx")
}

func (p *TxPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tlis.Len()
}

//添加进去一笔交易放入最后
//交易必须是校验过的
func (p *TxPool) PushBack(tx *TX) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tlis.Len() >= MAX_TX_POOL_SIZE {
		return errors.New("tx pool full,ignore push back")
	}
	id, err := tx.ID()
	if err != nil {
		return err
	}
	if _, has := p.tmap[id]; has {
		return errors.New("tx exists")
	}
	if err := p.setMemIdx(tx, true); err != nil {
		return err
	}
	ele := p.tlis.PushBack(tx)
	p.tmap[id] = ele
	return nil
}
