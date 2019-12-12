package xginx

import (
	"errors"
	"fmt"
	"testing"
)

func calcHash(blk *BlockInfo) {
	b := blk.Header.Bytes()
	for i := uint32(0); ; i++ {
		b.SetNonce(i)
		id := b.Hash()
		if CheckProofOfWork(id, blk.Header.Bits) {
			blk.Header = b.Header()
			break
		}
	}
}

func NewTestBlock(bi *BlockIndex) *BlockInfo {
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	calcHash(blk)
	if bi.Len() == 0 {
		conf.genesis, _ = blk.ID()
	}
	return blk
}

//从id开始创建一批区块头用来测试的
func NewTestHeaders(bi *BlockIndex, self bool, limit int, id HASH256) Headers {
	hs := Headers{}
	ele, err := bi.GetEle(id)
	if err != nil {
		panic(err)
	}
	if self {
		hs.Add(ele.BlockHeader)
	}
	for i := 0; i < limit; i++ {
		hv := BlockHeader{}
		hv.Ver = 1
		hv.Prev = id
		hv.Merkle = Hash256From([]byte{byte(i)})
		hv.Bits = bi.CalcBits(ele.Height + uint32(i) + 1)
		hv.Time = bi.lptr.TimeNow()
		b := hv.Bytes()
		for i := uint32(0); ; i++ {
			b.SetNonce(i)
			id = b.Hash()
			if CheckProofOfWork(id, hv.Bits) {
				hs.Add(b.Header())
				break
			}
		}
	}
	return hs
}

//移除所有区块
func removeAll(bi *BlockIndex) {
	cnt := bi.Len()
	for num := bi.Len() - 1; num >= 0; num-- {
		err := bi.UnlinkLast()
		if err != nil {
			panic(err)
		}
	}
	LogInfof("remove %d block success", cnt)
}

func createBlock(bi *BlockIndex, num int) {
	testnum := uint32(num)
	for i := uint32(0); i < testnum; i++ {
		cb := NewTestBlock(bi)
		err := bi.LinkBlk(cb)
		if err != nil {
			panic(err)
		}
	}
	LogInfof("create %d block success", testnum)
}

func TestUnlik1(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	//生成5个区块
	createBlock(bi, 5)
	//从3开始生成
	iter := bi.NewIter()
	if !iter.SeekHeight(3) {
		panic(errors.New("3 miss"))
	}
	//所有区块头不在链中测试
	hs := NewTestHeaders(bi, false, 4, iter.Curr().MustID())
	err := bi.Unlink(hs)
	if err == nil {
		t.Error("all not in scope error")
	}
}

func TestUnlik2(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	//生成5个区块
	createBlock(bi, 5)
	//从3开始生成
	iter := bi.NewIter()
	if !iter.SeekHeight(3) {
		panic(errors.New("3 miss"))
	}
	//第一个在链中
	hs := NewTestHeaders(bi, true, 4, iter.Curr().MustID())
	err := bi.Unlink(hs)
	if err != nil {
		panic(err)
	}
	if bi.Len() != 4 {
		t.Error("error")
	}
}

func TestUnlik3(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	//生成5个区块
	createBlock(bi, 5)
	//从3开始生成
	iter := bi.NewIter()
	if !iter.SeekHeight(3) {
		panic(errors.New("3 miss"))
	}
	//第一个在链中,但数量不够
	hs := NewTestHeaders(bi, true, 1, iter.Curr().MustID())
	err := bi.Unlink(hs)
	if err == nil {
		t.Error("num not enough")
	}
}

func TestUnlik4(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	//生成5个区块
	createBlock(bi, 5)
	//从3开始生成
	iter := bi.NewIter()
	if !iter.SeekHeight(0) {
		panic(errors.New("3 miss"))
	}
	hs := Headers{}
	//全部在链中
	for iter.Next() {
		hs.Add(iter.Curr().BlockHeader)
	}
	err := bi.Unlink(hs)
	if err != nil {
		t.Error("all headers in scope test error")
	}
}

func TestUnlik5(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	//生成5个区块
	createBlock(bi, 5)
	//从3开始生成
	iter := bi.NewIter()
	if !iter.SeekHeight(0) {
		panic(errors.New("3 miss"))
	}
	hs := Headers{}
	//全部在链中，但比当前链多一个
	for iter.Next() {
		hs.Add(iter.Curr().BlockHeader)
	}
	ns := NewTestHeaders(bi, false, 1, hs[len(hs)-1].MustID())
	hs.Add(ns[0])
	err := bi.Unlink(hs)
	if err != nil {
		t.Error("all headers in scope test error")
	}
	if bi.Len() != 5 {
		t.Error("num error")
	}
}

func GetTestBlockIndex() *BlockIndex {
	conf = LoadConfig("test.json")
	lis := newListener()
	bi := InitBlockIndex(lis)
	lis.OnStartup()
	if bi.Len() > 0 {
		conf.genesis = bi.First().MustID()
	}
	return GetBlockIndex()
}

func checkBalance(bi *BlockIndex, addr Address, amt Amount) error {
	ads, err := bi.ListCoins(addr)
	if err != nil {
		return err
	}
	if b := ads.All.Balance(); b != amt {
		return fmt.Errorf("Balance=%d != %d", b, amt)
	}
	return nil
}

func TestBitsCalc(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	createBlock(bi, 2017)
	removeAll(bi)
}

func createtx(bi *BlockIndex, blk *BlockInfo, a Address, b Address, fee Amount, coin *CoinKeyValue, lt uint32, seq uint32) *TX {
	tx := NewTx()
	acc, err := NewAccount(1, 1, false)
	if err != nil {
		panic(err)
	}
	in, err := coin.NewTxIn(acc)
	if err != nil {
		panic(err)
	}
	in.Sequence = seq
	tx.Ins = append(tx.Ins, in)
	out, err := b.NewTxOut(fee)
	if err != nil {
		panic(err)
	}
	tx.Outs = append(tx.Outs, out)
	keep, err := a.NewTxOut(coin.Value - fee)
	if err != nil {
		panic(err)
	}
	tx.Outs = append(tx.Outs, keep)
	tx.LockTime = lt
	err = tx.Sign(bi)
	if err != nil {
		panic(err)
	}
	if err := bi.txp.PushTx(bi, tx); err != nil {
		panic(err)
	}
	return tx
}

func TestSequance(t *testing.T) {
	a := Address("st1qresg66j0t9c8c9awxfkeremk0fwgha06hwuw6q")
	b := Address("st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3")
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	defer removeAll(bi)
	createBlock(bi, 101)
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	//获取可用的金额
	ds, err := bi.ListCoins(a)
	if err != nil {
		panic(err)
	}
	if len(ds.Coins) == 0 {
		panic(errors.New("not coins"))
	}
	//获取一笔可用的金额
	var coin *CoinKeyValue
	for _, v := range ds.Coins {
		coin = v
		break
	}
	if coin == nil {
		panic(errors.New("not coin"))
	}

	tp := bi.GetTxPool()

	//按高度锁住
	createtx(bi, blk, a, b, 20*COIN, coin, 0, 1000&SEQUENCE_MASK)
	if tp.Len() != 1 {
		panic("tx pool size error")
	}
	err = checkBalance(bi, a, 101*50*COIN-20*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 20*COIN)
	if err != nil {
		panic(err)
	}

	err = blk.LoadTxs(bi)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	calcHash(blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		panic(err)
	}
	if len(blk.Txs) != 1 {
		t.Errorf("tx seq lock,no add to block")
	}
	err = checkBalance(bi, a, 30*COIN+101*50*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 20*COIN)
	if err != nil {
		panic(err)
	}
	removeAll(bi)
	err = checkBalance(bi, a, 30*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 20*COIN)
	if err != nil {
		panic(err)
	}
}

func TestLockTimeTx(t *testing.T) {
	a := Address("st1qresg66j0t9c8c9awxfkeremk0fwgha06hwuw6q")
	b := Address("st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3")
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	defer removeAll(bi)
	createBlock(bi, 101)
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	//获取可用的金额
	ds, err := bi.ListCoins(a)
	if err != nil {
		panic(err)
	}
	if len(ds.Coins) == 0 {
		panic(errors.New("not coins"))
	}
	//获取一笔可用的金额
	var coin *CoinKeyValue
	for _, v := range ds.Coins {
		coin = v
		break
	}
	if coin == nil {
		panic(errors.New("not coin"))
	}

	tp := bi.GetTxPool()

	//locktime=120 seq=0
	createtx(bi, blk, a, b, 10*COIN, coin, 120, 0)
	if tp.Len() != 1 {
		panic("tx pool size error")
	}
	err = checkBalance(bi, a, 101*50*COIN-10*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 10*COIN)
	if err != nil {
		panic(err)
	}
	//locktime=120 seq=SEQUENCE_FINAL
	createtx(bi, blk, a, b, 20*COIN, coin, 0, SEQUENCE_FINAL)
	if tp.Len() != 1 {
		panic("tx pool size error")
	}
	err = checkBalance(bi, a, 101*50*COIN-20*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 20*COIN)
	if err != nil {
		panic(err)
	}

	err = blk.LoadTxs(bi)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	calcHash(blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, a, 30*COIN+101*50*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 20*COIN)
	if err != nil {
		panic(err)
	}
	removeAll(bi)
	err = checkBalance(bi, a, 0)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 0)
	if err != nil {
		panic(err)
	}
}

func TestUnlinkTo(t *testing.T) {
	a := Address("st1qresg66j0t9c8c9awxfkeremk0fwgha06hwuw6q")
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	defer removeAll(bi)
	createBlock(bi, 5)
	txs, err := bi.ListTxs(a)
	if err != nil {
		panic(err)
	}
	if len(txs) != 5 {
		t.Errorf("createBlock tx error")
	}
	err = bi.UnlinkTo(conf.genesis)
	if err != nil {
		panic(err)
	}
	txs, err = bi.ListTxs(a)
	if err != nil {
		panic(err)
	}
	if len(txs) != 1 {
		t.Errorf("createBlock tx error")
	}
	if bi.Len() != 1 {
		t.Errorf("unlink to error")
	}
	if !bi.First().MustID().Equal(conf.genesis) {
		t.Errorf("unlink to error")
	}
}
