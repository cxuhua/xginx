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
	tmap map[HASH256]*list.Element
	imap map[HASH256]txpoolin //txin -> tx 索引
	mdb  *memdb.DB
}

//NewTxPool 创建交易池
func NewTxPool() *TxPool {
	return &TxPool{
		tlis: list.New(),
		tmap: map[HASH256]*list.Element{},
		imap: map[HASH256]txpoolin{},
		mdb:  memdb.New(comparer.DefaultComparer, 1024*4),
	}
}

//Close 关闭交易池
func (p *TxPool) Close() {
	p.mdb.Reset()
}

func (p *TxPool) deltx(bi *BlockIndex, tx *TX) {
	id, err := tx.ID()
	if err != nil {
		panic(err)
	}
	p.del(bi, id)
}

func (p *TxPool) del(bi *BlockIndex, id HASH256) {
	if ele, has := p.tmap[id]; has {
		p.removeEle(bi, nil, ele)
	}
}

//Del 移除交易
func (p *TxPool) Del(bi *BlockIndex, id HASH256) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.del(bi, id)
}

//PushTxs 加入其他节点过来的多个交易数据
func (p *TxPool) PushTxs(bi *BlockIndex, msg *MsgTxPool) {
	bl := p.Len()
	for _, tx := range msg.Txs {
		id, err := tx.ID()
		if err != nil {
			continue
		}
		//已经被打包
		if bi.HasTxValue(id) {
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

//NewMsgGetTxPool 发送获取交易池数据包,并告知本节点拥有的
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

//NewMsgTxPool 获取交易池子数据
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

//获取输入引用的tx只能在txpool内部使用
func (p *TxPool) loadTxOut(bi *BlockIndex, in *TxIn) (*TX, *TxOut, error) {
	otx, err := bi.LoadTX(in.OutHash)
	if err != nil {
		otx, err = p.get(in.OutHash) //如果在交易池中
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

//GetRefsTxs 获取交易引用的交易id
func (p *TxPool) GetRefsTxs(id HASH256) []HASH256 {
	prefix := GetDBKey(RefTxPrefix, id[:])
	iter := p.mdb.NewIterator(util.BytesPrefix(prefix))
	defer iter.Release()
	ids := []HASH256{}
	for iter.Next() {
		id := HASH256{}
		copy(id[:], iter.Key()[len(prefix):])
		ids = append(ids, id)
	}
	return ids
}

//检测引用的交易是否存在
func (p *TxPool) checkRefs(bi *BlockIndex, tx *TX) error {
	for _, in := range tx.Ins {
		_, _, err := p.loadTxOut(bi, in)
		if err != nil {
			return fmt.Errorf("ref tx miss %w", err)
		}
	}
	return nil
}

//设置内存消费金额索引
func (p *TxPool) setMemIdx(bi *BlockIndex, tx *TX, add bool) error {
	txid, err := tx.ID()
	if err != nil {
		return err
	}
	tx.pool = add
	refs := map[HASH256]bool{}
	vps := map[HASH160]bool{}
	//存储已经消费的输出
	for _, in := range tx.Ins {
		ref, out, err := p.loadTxOut(bi, in)
		if err != nil {
			return err
		}
		if ref.pool {
			refs[ref.MustID()] = add
		}
		pkh, err := out.Script.GetPkh()
		if err != nil {
			return err
		}
		ckv := &CoinKeyValue{}
		ckv.Index = in.OutIndex
		ckv.TxID = in.OutHash
		vps[pkh] = add
		if add {
			p.imap[in.OutKey()] = txpoolin{tx: tx, in: in} //存放in对应的tx和位置
			err = p.mdb.Put(ckv.SpentKey(), txid[:])       //存放消耗的金额
		} else {
			delete(p.imap, in.OutKey())
			err = p.mdb.Delete(ckv.SpentKey())
		}
		if err != nil {
			return err
		}
	}
	//存储内存池中可用的金额
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
		ckv.Height = 0
		vps[pkh] = add
		if add {
			err = p.mdb.Put(ckv.MustKey(), ckv.MustValue()) //存储输出到内存池的金额
		} else {
			err = p.mdb.Delete(ckv.MustKey())
		}
		if err != nil {
			return err
		}
	}
	//存储哪些交易引用到了当前交易
	for ref := range refs {
		key := GetDBKey(RefTxPrefix, ref[:], txid[:])
		if add {
			err = p.mdb.Put(key, VarUInt(len(refs)).Bytes())
		} else {
			err = p.mdb.Delete(key)
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
			err = p.mdb.Put(key, vbys)
		} else {
			err = p.mdb.Delete(key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

//移除引用了此交易的交易，返回移除了的交易
func (p *TxPool) removeRefsTxs(bi *BlockIndex, refs *[]*TX, id HASH256) {
	ids := p.GetRefsTxs(id)
	for _, id := range ids {
		ele, has := p.tmap[id]
		if !has {
			continue
		}
		p.removeEle(bi, refs, ele)
		if refs != nil {
			*refs = append(*refs, ele.Value.(*TX))
		}
	}
}

//移除一个元素
func (p *TxPool) removeEle(bi *BlockIndex, refs *[]*TX, ele *list.Element) {
	ps := GetPubSub()
	tx, ok := ele.Value.(*TX)
	if !ok {
		panic(errors.New("txpool save type error"))
	}
	id := tx.MustID()
	//引用了此交易的交易也应该被删除
	p.removeRefsTxs(bi, refs, id)
	//移除自己
	err := p.setMemIdx(bi, tx, false)
	if err != nil {
		panic(err)
	}
	p.tlis.Remove(ele)
	delete(p.tmap, id)
	//广播交易从内存池移除
	ps.Pub(id, TxPoolDelTxTopic)
	LogInfof("remove tx %v success from txpool len=%d", id, p.tlis.Len())
}

//GetDelTxs 返回已经删除的引用的交易
func (p *TxPool) GetDelTxs(bi *BlockIndex, txs []*TX) []*TX {
	p.mu.Lock()
	defer p.mu.Unlock()
	refs := []*TX{}
	for _, tx := range txs {
		id := tx.MustID()
		ele, has := p.tmap[id]
		if !has {
			continue
		}
		p.removeEle(bi, &refs, ele)
	}
	return refs
}

//DelTxs 当区块打包时，移除多个交易
func (p *TxPool) DelTxs(bi *BlockIndex, txs []*TX) {
	//移除并返回删除了的交易
	refs := p.GetDelTxs(bi, txs)
	//这些被删除的引用是否恢复?反向恢复
	for i := len(refs) - 1; i >= 0; i-- {
		tx := refs[i]
		err := tx.Check(bi, true)
		if err != nil {
			continue
		}
		err = p.PushTx(bi, tx)
		if err != nil {
			LogError("repush tx error", err)
		}
	}
}

//AllTxs 获取所有的tx
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
		err := tx.Check(bi, true)
		//检测失败的将会被删除，除了seq检查错误
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
		p.removeEle(bi, nil, ele)
	}
	return txs, nil
}

//HasCoin 是否存在可消费的coin
func (p *TxPool) HasCoin(coin *CoinKeyValue) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mdb.Contains(coin.MustKey())
}

//ListTxsWithID 获取spkh相关的交易
func (p *TxPool) ListTxsWithID(bi *BlockIndex, spkh HASH160, limit ...int) (TxIndexs, error) {
	prefix := GetDBKey(TxpPrefix, spkh[:])
	idxs := TxIndexs{}
	iter := p.mdb.NewIterator(util.BytesPrefix(prefix))
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
func (p *TxPool) GetCoin(pkh HASH160, txid HASH256, idx VarUInt) (*CoinKeyValue, error) {
	key := GetDBKey(CoinsPrefix, pkh[:], txid[:], idx.Bytes())
	val, err := p.mdb.Get(key)
	if err != nil {
		return nil, fmt.Errorf("get coin from txpool error %w", err)
	}
	ckv := &CoinKeyValue{pool: true}
	err = ckv.From(key, val)
	return ckv, err
}

//ListCoins 获取pkh在交易池中可用的金额
//这些金额一般是交易转账找零剩下的金额
func (p *TxPool) ListCoins(spkh HASH160, limit ...Amount) (Coins, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	coins := Coins{}
	if len(limit) > 0 && limit[0] <= 0 {
		return coins, nil
	}
	key := GetDBKey(CoinsPrefix, spkh[:])
	iter := p.mdb.NewIterator(util.BytesPrefix(key))
	defer iter.Release()
	sum := Amount(0)
	for iter.Next() {
		ckv := &CoinKeyValue{pool: true}
		err := ckv.From(iter.Key(), iter.Value())
		if err != nil {
			return nil, err
		}
		ckv.spent = p.mdb.Contains(ckv.SpentKey())
		coins = append(coins, ckv)
		sum += ckv.Value
		if len(limit) > 0 && sum >= limit[0] {
			return coins, nil
		}
	}
	return coins, nil
}

//IsSpent 一笔钱是否已经在内存交易池中某个交易消费
func (p *TxPool) IsSpent(skey []byte) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mdb.Contains(skey)
}

//IsSpentCoin 一笔钱是否已经在内存交易池中某个交易消费
func (p *TxPool) IsSpentCoin(coin *CoinKeyValue) bool {
	return p.IsSpent(coin.SpentKey())
}

//Has 交易池是否存在某个交易
func (p *TxPool) Has(id HASH256) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, has := p.tmap[id]
	return has
}

//获取交易
func (p *TxPool) get(id HASH256) (*TX, error) {
	if ele, has := p.tmap[id]; has {
		tx := ele.Value.(*TX)
		return tx, nil
	}
	return nil, fmt.Errorf("txpool not found tx = %v", id)
}

//Get 获取交易
func (p *TxPool) Get(id HASH256) (*TX, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.get(id)
}

//Len 获取交易池交易数量
func (p *TxPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tlis.Len()
}

func (p *TxPool) replace(bi *BlockIndex, old *TX, new *TX) error {
	bi.lptr.OnTxPoolRep(old, new)
	p.deltx(bi, old)
	return nil
}

//如果有重复引用了同一笔输出，根据条件 Sequence 进行覆盖
func (p *TxPool) replaceTx(bi *BlockIndex, tx *TX) error {
	for _, in := range tx.Ins {
		//获取有相同引用的交易
		if _, has := p.imap[in.OutKey()]; !has {
			continue
		}
		//引用了相同的输出并且不能覆盖不能进入交易池
		return errors.New("sequence < old seq error")
	}
	return nil
}

//PushTx 添加进去一笔交易放入最后
//交易必须是校验过的
func (p *TxPool) PushTx(bi *BlockIndex, tx *TX) error {
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
	//执行失败不会进入交易池
	if err := tx.ExecScript(bi, OptPushTxPool); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tlis.Len() >= MaxTxPoolSize {
		return errors.New("tx pool full,ignore push back")
	}
	if err := p.checkRefs(bi, tx); err != nil {
		return err
	}
	if err := p.replaceTx(bi, tx); err != nil {
		return err
	}
	if err := bi.lptr.OnTxPool(tx); err != nil {
		return err
	}
	if _, has := p.tmap[id]; has {
		return errors.New("tx exists")
	}
	if err := p.setMemIdx(bi, tx, true); err != nil {
		return err
	}
	ele := p.tlis.PushBack(tx)
	p.tmap[id] = ele
	return nil
}
