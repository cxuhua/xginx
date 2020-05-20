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

//交易池最大数量
const (
	MaxTxPoolSize = 4096 * 4
)

type txpoolin struct {
	tx *TX
	in *TxIn
}

//TxPool 交易池，存放签名成功，未确认的交易
//当区块连接后需要把区块中的交易从这个池子删除
//交易池加入交易后会记录消费输出，也会记录交易池中可用的金额
type TxPool struct {
	mu   sync.RWMutex
	tlis *list.List
	tmap map[HASH256]*list.Element //按交易id存储
	imap map[HASH256]txpoolin      //输入引用的交易，txin -> tx 索引
	mdb  *memdb.DB
}

//NewTxPool 创建交易池
func NewTxPool() *TxPool {
	return &TxPool{
		tlis: list.New(),
		tmap: map[HASH256]*list.Element{},
		imap: map[HASH256]txpoolin{},
		mdb:  memdb.New(comparer.DefaultComparer, MaxTxPoolSize),
	}
}

//Close 关闭交易池
func (pool *TxPool) Close() {
	pool.mdb.Reset()
}

func (pool *TxPool) deltx(bi *BlockIndex, tx *TX) {
	id, err := tx.ID()
	if err != nil {
		panic(err)
	}
	pool.del(bi, id)
}

func (pool *TxPool) del(bi *BlockIndex, id HASH256) {
	if ele, has := pool.tmap[id]; has {
		pool.removeEle(bi, nil, ele)
	}
}

//Del 移除交易
func (pool *TxPool) Del(bi *BlockIndex, id HASH256) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.del(bi, id)
}

//PushTxs 加入其他节点过来的多个交易数据
func (pool *TxPool) PushTxs(bi *BlockIndex, msg *MsgTxPool) {
	bl := pool.Len()
	for _, tx := range msg.Txs {
		err := pool.PushTx(bi, tx)
		if err != nil {
			LogWarnf("push tx to txpool error,ignore tx", err)
		}
	}
	if pool.Len() > bl {
		LogInfof("tx pool new add %d tx", pool.Len()-bl)
	}
}

//NewMsgGetTxPool 发送获取交易池所有的交易ID
func (pool *TxPool) NewMsgGetTxPool() *MsgGetTxPool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	msg := &MsgGetTxPool{}
	for _, ele := range pool.tmap {
		tx := ele.Value.(*TX)
		msg.Add(tx.MustID())
	}
	return msg
}

//NewMsgTxPool 获取交易池数据，并忽略对方有的交易
func (pool *TxPool) NewMsgTxPool(m *MsgGetTxPool) *MsgTxPool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	msg := &MsgTxPool{}
	for cur := pool.tlis.Front(); cur != nil; cur = cur.Next() {
		tx := cur.Value.(*TX)
		//忽略对方有的
		if m.Has(tx.MustID()) {
			continue
		}
		msg.Add(tx)
	}
	return msg
}

//获取输入引用的tx只能在txpool内部使用
func (pool *TxPool) loadTxOut(bi *BlockIndex, in *TxIn) (*TX, *TxOut, error) {
	otx, err := bi.LoadTX(in.OutHash)
	if err != nil {
		otx, err = pool.get(in.OutHash) //如果在交易池中
	}
	if err != nil {
		return nil, nil, fmt.Errorf("txin outtx miss %w", err)
	}
	oidx := in.OutIndex.ToInt()
	if oidx < 0 || oidx >= len(otx.Outs) {
		return nil, nil, fmt.Errorf("outindex out of bound")
	}
	out := otx.Outs[oidx]
	out.pool = otx.pool
	return otx, out, nil
}

//设置内存消费金额索引
func (pool *TxPool) setMemIdx(bi *BlockIndex, tx *TX, add bool) error {
	txid, err := tx.ID()
	if err != nil {
		return err
	}
	tx.pool = add
	vps := map[HASH160]bool{}
	//存储已经消费的输出
	for _, in := range tx.Ins {
		//获取引用的交易
		out, err := in.LoadTxOut(bi)
		if err != nil {
			return err
		}
		//如果引用的是交易池将失败
		if out.IsPool() {
			return fmt.Errorf("ref txpool tx error")
		}
		pkh, err := out.Script.GetPkh()
		if err != nil {
			return err
		}
		ckv := &CoinKeyValue{}
		ckv.Index = in.OutIndex
		ckv.TxID = in.OutHash
		vps[pkh] = add
		skv := ckv.SpentKey()
		if add {
			pool.imap[in.OutKey()] = txpoolin{tx: tx, in: in} //存放in对应的tx和位置
			err = pool.mdb.Put(skv, txid[:])                  //存放消耗的金额
		} else {
			//移除时删除
			delete(pool.imap, in.OutKey())
			err = pool.mdb.Delete(skv)
		}
		if err != nil {
			return err
		}
	}
	//存储交易池中可用的金额
	for idx, out := range tx.Outs {
		ckv := &CoinKeyValue{}
		pkh, err := out.Script.GetPkh()
		if err != nil {
			return err
		}
		ckv.Value = out.Value
		ckv.CPkh = pkh
		ckv.Index = VarUInt(idx)
		ckv.TxID = txid
		ckv.Base = 0
		ckv.Height = 0 //交易池中的金额始终为0
		vps[pkh] = add
		if add {
			err = pool.mdb.Put(ckv.MustKey(), ckv.MustValue()) //存储输出到内存池的金额
		} else {
			err = pool.mdb.Delete(ckv.MustKey())
		}
		if err != nil {
			return err
		}
	}
	//写入账户相关的交易
	for pkh := range vps {
		//pkh相关的内存中的交易
		vval := TxValue{
			TxIdx: 0,
			BlkID: ZERO256,
		}
		vbys, err := vval.Bytes()
		if err != nil {
			return err
		}
		//交易池中的交易设置为无效的高度
		key := GetDBKey(TxpPrefix, pkh[:], []byte{0xff, 0xff, 0xff, 0xff}, txid[:])
		if add {
			err = pool.mdb.Put(key, vbys)
		} else {
			err = pool.mdb.Delete(key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

//移除一个元素
func (pool *TxPool) removeEle(bi *BlockIndex, refs *[]*TX, ele *list.Element) {
	ps := GetPubSub()
	tx, ok := ele.Value.(*TX)
	if !ok {
		panic(errors.New("txpool save type error"))
	}
	id := tx.MustID()
	//移除自己
	err := pool.setMemIdx(bi, tx, false)
	if err != nil {
		panic(err)
	}
	pool.tlis.Remove(ele)
	delete(pool.tmap, id)
	//广播交易从内存池移除
	ps.Pub(id, TxPoolDelTxTopic)
	LogInfof("remove tx %v success from txpool len=%d", id, pool.tlis.Len())
}

//GetDelTxs 返回已经删除的引用的交易
func (pool *TxPool) GetDelTxs(bi *BlockIndex, txs []*TX) []*TX {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	refs := []*TX{}
	for _, tx := range txs {
		id := tx.MustID()
		ele, has := pool.tmap[id]
		if !has {
			continue
		}
		pool.removeEle(bi, &refs, ele)
	}
	return refs
}

//DelTxs 当区块打包时，移除多个交易
func (pool *TxPool) DelTxs(bi *BlockIndex, txs []*TX) {
	//移除并返回删除了的交易
	refs := pool.GetDelTxs(bi, txs)
	//这些被删除的引用是否恢复?反向恢复
	//为什么重新加入交易池恢复：交易加入区块链后，引用这个交易的其他交易会变得可用
	//所以可以重新加入交易池进行处理
	for i := len(refs) - 1; i >= 0; i-- {
		tx := refs[i]
		err := tx.Check(bi, true)
		if err != nil {
			continue
		}
		err = pool.PushTx(bi, tx)
		if err != nil {
			LogError("repush tx error", err)
		}
	}
}

//AllTxs 获取所有的tx
func (pool *TxPool) AllTxs() []*TX {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	txs := []*TX{}
	//获取用来打包区块的交易
	for cur := pool.tlis.Front(); cur != nil; cur = cur.Next() {
		tx := cur.Value.(*TX)
		txs = append(txs, tx)
	}
	return txs
}

//获取需要打包的交易并返回需要移除的交易
func (pool *TxPool) gettxs(bi *BlockIndex, blk *BlockInfo) ([]*TX, []*list.Element, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	txs := []*TX{}
	size := 0
	buf := NewWriter()
	res := []*list.Element{}
	//获取用来打包区块的交易
	for cur := pool.tlis.Front(); cur != nil; cur = cur.Next() {
		buf.Reset()
		tx := cur.Value.(*TX)
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
		if size > MaxBlockSize {
			break
		}
		txs = append(txs, tx)
	}
	return txs, res, nil
}

//GetTxs 取出符合区块blk的交易，大小不能超过限制
func (pool *TxPool) GetTxs(bi *BlockIndex, blk *BlockInfo) ([]*TX, error) {
	//获取交易
	txs, res, err := pool.gettxs(bi, blk)
	if err != nil {
		return nil, err
	}
	//移除检测失败的
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for _, ele := range res {
		pool.removeEle(bi, nil, ele)
	}
	return txs, nil
}

//HasCoin 是否存在可消费的coin
func (pool *TxPool) HasCoin(coin *CoinKeyValue) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return pool.mdb.Contains(coin.MustKey())
}

//ListTxsWithID 获取spkh相关的交易
func (pool *TxPool) ListTxsWithID(bi *BlockIndex, spkh HASH160, limit ...int) (TxIndexs, error) {
	prefix := GetDBKey(TxpPrefix, spkh[:])
	idxs := TxIndexs{}
	iter := pool.mdb.NewIterator(util.BytesPrefix(prefix))
	defer iter.Release()
	if iter.Last() {
		iv, err := NewTxIndex(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		iv.pool = true
		idxs = append(idxs, iv)
		if len(limit) > 0 && len(idxs) >= limit[0] {
			return idxs, nil
		}
	}
	for iter.Prev() {
		iv, err := NewTxIndex(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		iv.pool = true
		idxs = append(idxs, iv)
		if len(limit) > 0 && len(idxs) >= limit[0] {
			return idxs, nil
		}
	}
	return idxs, nil
}

//GetCoin 获取交易相关的金额
func (pool *TxPool) GetCoin(pkh HASH160, txid HASH256, idx VarUInt) (*CoinKeyValue, error) {
	key := GetDBKey(CoinsPrefix, pkh[:], txid[:], idx.Bytes())
	val, err := pool.mdb.Get(key)
	if err != nil {
		return nil, fmt.Errorf("get coin from txpool error %w", err)
	}
	ckv := &CoinKeyValue{pool: true}
	err = ckv.From(key, val)
	return ckv, err
}

//ListCoins 获取pkh在交易池中可用的金额
//这些金额一般是交易转账找零剩下的金额
func (pool *TxPool) ListCoins(spkh HASH160) (Coins, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	coins := Coins{}
	key := GetDBKey(CoinsPrefix, spkh[:])
	iter := pool.mdb.NewIterator(util.BytesPrefix(key))
	defer iter.Release()
	for iter.Next() {
		ckv := &CoinKeyValue{pool: true}
		err := ckv.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		coins = append(coins, ckv)
	}
	return coins, nil
}

//IsSpent 一笔钱是否已经在内存交易池中某个交易消费
func (pool *TxPool) IsSpent(skey []byte) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return pool.mdb.Contains(skey)
}

//IsSpentCoin 一笔钱是否已经在内存交易池中某个交易消费
func (pool *TxPool) IsSpentCoin(coin *CoinKeyValue) bool {
	return pool.IsSpent(coin.SpentKey())
}

//Has 交易池是否存在某个交易
func (pool *TxPool) Has(id HASH256) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	_, has := pool.tmap[id]
	return has
}

//获取交易
func (pool *TxPool) get(id HASH256) (*TX, error) {
	if ele, has := pool.tmap[id]; has {
		tx := ele.Value.(*TX)
		return tx, nil
	}
	return nil, fmt.Errorf("txpool not found tx = %v", id)
}

//Get 获取交易
func (pool *TxPool) Get(id HASH256) (*TX, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return pool.get(id)
}

//Len 获取交易池交易数量
func (pool *TxPool) Len() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return pool.tlis.Len()
}

func (pool *TxPool) replace(bi *BlockIndex, old *TX, new *TX) error {
	bi.lptr.OnTxPoolRep(old, new)
	pool.deltx(bi, old)
	return nil
}

//如果有重复引用了同一笔输出，根据条件 Sequence 进行覆盖
func (pool *TxPool) replaceTx(bi *BlockIndex, tx *TX) error {
	for _, in := range tx.Ins {
		//获取有相同引用的交易
		val, has := pool.imap[in.OutKey()]
		if !has {
			continue
		}
		if tx.IsReplace(val.tx) {
			return pool.replace(bi, val.tx, tx)
		}
		//如果不能替换就是引用重复了
		return fmt.Errorf("ref out repeat error")
	}
	return nil
}

//PushTx 添加进去一笔交易放入最后
//交易必须是校验过的
func (pool *TxPool) PushTx(bi *BlockIndex, tx *TX) error {
	id, err := tx.ID()
	if err != nil {
		return err
	}
	//如果交易已经在区块中忽略
	if bi.HasTxValue(id) {
		return errors.New("tx in block idnex")
	}
	//coinbase不允许进入交易池
	if tx.IsCoinBase() {
		return errors.New("coinbase push to txpool error")
	}
	//检测交易是否合法
	if err := tx.Check(bi, true); err != nil {
		return err
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.tlis.Len() >= MaxTxPoolSize {
		return errors.New("tx pool full,ignore push back")
	}
	//执行失败不会进入交易池
	if err := tx.ExecScript(bi, OptPushTxPool); err != nil {
		return err
	}
	if err := pool.replaceTx(bi, tx); err != nil {
		return err
	}
	if err := bi.lptr.OnTxPool(tx); err != nil {
		return err
	}
	if _, has := pool.tmap[id]; has {
		return errors.New("tx exists")
	}
	if err := pool.setMemIdx(bi, tx, true); err != nil {
		return err
	}
	ele := pool.tlis.PushBack(tx)
	pool.tmap[id] = ele
	return nil
}
