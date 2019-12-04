package xginx

import (
	"container/list"
	"errors"
	"fmt"
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
func (p *TxPool) Del(id HASH256) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ele, has := p.tmap[id]; has {
		p.removeEle(ele)
		LogInfo("del txpool tx=", id, " pool size =", p.tlis.Len())
	}
}

//加入其他节点过来的多个交易数据
func (p *TxPool) PushTxs(bi *BlockIndex, msg *MsgTxPool) {
	bl := p.Len()
	for _, tx := range msg.Txs {
		id, err := tx.ID()
		if err != nil {
			continue
		}
		//已经被打包
		if _, err := bi.LoadTxValue(id); err == nil {
			continue
		}
		if err := tx.Check(bi, true); err != nil {
			LogError("check tx error,skip push to txpoool,", err)
			continue
		}
		err = p.PushTx(bi, tx)
		if err != nil {
			LogError("push tx to pool error", err)
		}
	}
	if p.Len() > bl {
		LogInfof("tx pool new add %d tx", p.Len()-bl)
	}
}

//发送获取交易池数据包,并告知本节点拥有的
func (p *TxPool) NewMsgGetTxPool() *MsgGetTxPool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	msg := &MsgGetTxPool{}
	for _, ele := range p.tmap {
		tx := ele.Value.(*TX)
		id, err := tx.ID()
		if err != nil {
			panic(err)
		}
		msg.Add(id)
	}
	return msg
}

//获取交易池子数据
func (p *TxPool) NewMsgTxPool(m *MsgGetTxPool) *MsgTxPool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	msg := &MsgTxPool{}
	for cur := p.tlis.Front(); cur != nil; cur = cur.Next() {
		tx := cur.Value.(*TX)
		id, err := tx.ID()
		if err != nil {
			panic(err)
		}
		//忽略对方有的
		if m.Has(id) {
			continue
		}
		msg.Add(tx)
	}
	return msg
}

//设置内存消费金额索引
func (p *TxPool) setMemIdx(tx *TX, add bool) {
	tid, err := tx.ID()
	if err != nil {
		panic(err)
	}
	tx.pool = add
	//存储已经消费的输出
	for _, in := range tx.Ins {
		ckv := &CoinKeyValue{}
		ckv.Index = in.OutIndex
		ckv.TxId = in.OutHash
		if add {
			err = p.mdb.Put(ckv.SpentKey(), tid[:])
		} else {
			err = p.mdb.Delete(ckv.SpentKey())
		}
		if err != nil {
			panic(err)
		}
	}
	//存储内存池中可用的金额
	for idx, out := range tx.Outs {
		ckv := &CoinKeyValue{}
		pkh, err := out.Script.GetPkh()
		if err != nil {
			panic(err)
		}
		ckv.Value = out.Value
		ckv.CPkh = pkh
		ckv.Index = VarUInt(idx)
		ckv.TxId = tid
		if add {
			err = p.mdb.Put(ckv.GetKey(), ckv.GetValue())
		} else {
			err = p.mdb.Delete(ckv.GetKey())
		}
		if err != nil {
			panic(err)
		}
	}
}

//移除引用了此交易的交易
func (p *TxPool) removeRefsTxs(id HASH256, ele *list.Element) {
	ids := map[HASH256]bool{}
	for _, ref := range p.tmap {
		//忽略自己
		if ref == ele {
			continue
		}
		tx := ref.Value.(*TX)
		tid, err := tx.ID()
		if err != nil {
			panic(err)
		}
		for _, in := range tx.Ins {
			if !in.OutHash.Equal(id) {
				continue
			}
			ids[tid] = true
		}
	}
	for key, _ := range ids {
		ele, has := p.tmap[key]
		if !has {
			continue
		}
		p.removeEle(ele)
	}
	ids = nil
}

//移除一个元素
func (p *TxPool) removeEle(ele *list.Element) {
	tx := ele.Value.(*TX)
	id, err := tx.ID()
	if err != nil {
		panic(err)
	}
	//引用了此交易的交易也应该被删除
	p.removeRefsTxs(id, ele)
	//移除自己
	p.setMemIdx(tx, false)
	p.tlis.Remove(ele)
	delete(p.tmap, id)
}

//移除多个交易
func (p *TxPool) DelTxs(txs []*TX) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, tx := range txs {
		id, err := tx.ID()
		if err != nil {
			panic(err)
		}
		ele, has := p.tmap[id]
		if !has {
			continue
		}
		p.removeEle(ele)
	}
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

//获取需要打包的交易并返回需要移除的交易
func (p *TxPool) gettxs(bi *BlockIndex, blk *BlockInfo) ([]*TX, []*list.Element, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	txs := []*TX{}
	size := 0
	buf := NewWriter()
	res := []*list.Element{}
	//获取用来打包区块的交易
	for cur := p.tlis.Front(); cur != nil; cur = cur.Next() {
		buf.Reset()
		tx := cur.Value.(*TX)
		//未到时间的交易忽略
		if err := tx.CheckLockTime(blk); err != nil {
			continue
		}
		err := tx.Check(bi, true)
		//检测失败的将会被删除
		if err != nil {
			res = append(res, cur)
			continue
		}
		err = tx.Encode(buf)
		if err != nil {
			panic(err)
		}
		size += buf.Len()
		if size > MAX_BLOCK_SIZE {
			break
		}
		txs = append(txs, tx)
	}
	return txs, res, nil
}

//取出符合区块blk的交易，大小不能超过限制
func (p *TxPool) GetTxs(bi *BlockIndex, blk *BlockInfo) ([]*TX, error) {
	//获取交易
	txs, res, err := p.gettxs(bi, blk)
	if err != nil {
		return nil, err
	}
	//移除检测失败的
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ele := range res {
		p.removeEle(ele)
	}
	return txs, nil
}

//是否存在可消费的coin
func (p *TxPool) HasCoin(coin *CoinKeyValue) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mdb.Contains(coin.GetKey())
}

//获取spkh相关的交易
func (p *TxPool) ListTxsWithID(bi *BlockIndex, spkh HASH160) (TxIndexs, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	idxs := TxIndexs{}
	for cur := p.tlis.Front(); cur != nil; cur = cur.Next() {
		tx := cur.Value.(*TX)
		id, err := tx.ID()
		if err != nil {
			return nil, err
		}
		vval := TxValue{
			TxIdx: VarUInt(0),
			BlkId: ZERO, //引用自内存中的交易
		}
		//交易中有哪些账户
		ids := map[HASH256]bool{}
		for _, in := range tx.Ins {
			if in.IsCoinBase() {
				continue
			}
			out, err := in.LoadTxOut(bi)
			if err != nil {
				return nil, err
			}
			pkh, err := out.Script.GetPkh()
			if err != nil {
				return nil, err
			}
			if pkh.Equal(spkh) {
				ids[id] = true
			}
		}
		for _, out := range tx.Outs {
			pkh, err := out.Script.GetPkh()
			if err != nil {
				return nil, err
			}
			if pkh.Equal(spkh) {
				ids[id] = true
			}
		}
		for tid, _ := range ids {
			vv := &TxIndex{}
			vv.TxId = tid
			vv.Value = vval
			idxs = append(idxs, vv)
		}
	}
	return idxs, nil
}

//获取pkh在交易池中可用的金额
func (p *TxPool) ListCoins(spkh HASH160, limit ...Amount) (Coins, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	coins := Coins{}
	if len(limit) > 0 && limit[0] <= 0 {
		return coins, nil
	}
	key := getDBKey(COINS_PREFIX, spkh[:])
	iter := p.mdb.NewIterator(util.BytesPrefix(key))
	defer iter.Release()
	sum := Amount(0)
	for iter.Next() {
		tk := &CoinKeyValue{}
		err := tk.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		tk.pool = true
		//过滤已经被交易池消费的
		if p.mdb.Contains(tk.SpentKey()) {
			continue
		}
		coins = append(coins, tk)
		sum += tk.Value
		if len(limit) > 0 && sum >= limit[0] {
			return coins, nil
		}
	}
	return coins, nil
}

//一笔钱是否已经在内存交易池中某个交易消费
func (p *TxPool) IsSpentCoin(coin *CoinKeyValue) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mdb.Contains(coin.SpentKey())
}

//交易池是否存在某个交易
func (p *TxPool) Has(id HASH256) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, has := p.tmap[id]
	return has
}

//获取交易
func (p *TxPool) Get(id HASH256) (*TX, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if ele, has := p.tmap[id]; has {
		tx := ele.Value.(*TX)
		return tx, nil
	}
	return nil, fmt.Errorf("txpool not found tx = %v", id)
}

//获取交易池交易数量
func (p *TxPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tlis.Len()
}

//添加进去一笔交易放入最后
//交易必须是校验过的
func (p *TxPool) PushTx(bi *BlockIndex, tx *TX) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tlis.Len() >= MAX_TX_POOL_SIZE {
		return errors.New("tx pool full,ignore push back")
	}
	if err := bi.lptr.OnTxPool(tx); err != nil {
		return err
	}
	id, err := tx.ID()
	if err != nil {
		return err
	}
	if _, has := p.tmap[id]; has {
		return errors.New("tx exists")
	}
	p.setMemIdx(tx, true)
	ele := p.tlis.PushBack(tx)
	p.tmap[id] = ele
	return nil
}
