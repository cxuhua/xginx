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
	lis := newListener(conf.WalletDir)
	bi := InitBlockIndex(lis)
	lis.OnStartup()
	if bi.Len() > 0 {
		conf.genesis = bi.First().MustID()
	}
	return GetBlockIndex()
}

func getacc(bi *BlockIndex, addr Address) *Account {
	acc, err := bi.lptr.GetWallet().GetAccount(addr)
	if err != nil {
		panic(err)
	}
	return acc
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
	acc, err := bi.lptr.GetWallet().GetAccount(a)
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

func TestTransfire(t *testing.T) {
	var err error
	a := Address("st1qresg66j0t9c8c9awxfkeremk0fwgha06hwuw6q")
	b := Address("st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3")
	c := Address("st1qm24876nvtcn83m8jlg7r4jsr223lcepn3g8wt3")
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	defer removeAll(bi)
	//开始余额应该全是0
	err = checkBalance(bi, a, 0)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 0)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, c, 0)
	if err != nil {
		panic(err)
	}
	//开始应该都没有交易数据
	txs, err := bi.ListTxs(a)
	if err != nil {
		panic(err)
	}
	if len(txs) != 0 {
		t.Error("a txs error")
	}
	txs, err = bi.ListTxs(b)
	if err != nil {
		panic(err)
	}
	if len(txs) != 0 {
		t.Error("b txs error")
	}
	txs, err = bi.ListTxs(c)
	if err != nil {
		panic(err)
	}
	if len(txs) != 0 {
		t.Error("c txs error")
	}
	//生成5个区块
	createBlock(bi, 100)

	err = checkBalance(bi, a, 100*50*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 0)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, c, 0)
	if err != nil {
		panic(err)
	}

	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	//A -> B = 15 fee=1
	mi := bi.NewMulTrans()
	mi.Acts = []*Account{getacc(bi, a)}
	mi.Keep = 0
	mi.Spent = blk.Meta.Height
	mi.Dst = []Address{b}
	mi.Amts = []Amount{15 * COIN}
	mi.Fee = 1 * COIN
	mi.Ext = []byte{}
	tx1, err := mi.NewTx(true)
	if err != nil {
		panic(err)
	}
	err = blk.AddTx(bi, tx1)
	if err != nil {
		panic(err)
	}
	//A转给B15个后剩余，加上扣除的交易费
	err = checkBalance(bi, a, 100*50*COIN-15*COIN-1*COIN)
	if err != nil {
		panic(err)
	}
	//B得到15个
	err = checkBalance(bi, b, 15*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, c, 0)
	if err != nil {
		panic(err)
	}
	//
	txs, err = bi.ListTxs(a)
	if err != nil {
		panic(err)
	}
	//5个coinbase+一个交易
	if len(txs) != 101 {
		t.Error("a txs error")
	}
	if !txs[100].IsPool() {
		t.Error("5 must is pool")
	}
	txs, err = bi.ListTxs(b)
	if err != nil {
		panic(err)
	}
	if len(txs) != 1 {
		t.Error("b txs error")
	}
	if !txs[0].IsPool() {
		t.Error("b 0 must is pool")
	}
	txs, err = bi.ListTxs(c)
	if err != nil {
		panic(err)
	}
	if len(txs) != 0 {
		t.Error("c txs error")
	}

	//B -> C =5 fee=1
	mi = bi.NewMulTrans()
	mi.Acts = []*Account{getacc(bi, b)}
	mi.Keep = 0
	mi.Spent = blk.Meta.Height
	mi.Dst = []Address{c}
	mi.Amts = []Amount{5 * COIN}
	mi.Fee = 1 * COIN
	mi.Ext = []byte{}

	tx2, err := mi.NewTx(true)
	if err != nil {
		panic(err)
	}
	err = blk.AddTx(bi, tx2)
	if err != nil {
		panic(err)
	}

	//a剩余
	err = checkBalance(bi, a, 100*50*COIN-15*COIN-1*COIN)
	if err != nil {
		panic(err)
	}
	//b剩余 扣除了交费费用
	err = checkBalance(bi, b, 15*COIN-5*COIN-1*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, c, 5*COIN)
	if err != nil {
		panic(err)
	}

	txs, err = bi.ListTxs(a)
	if err != nil {
		panic(err)
	}
	if len(txs) != 101 {
		t.Error("a txs error")
	}
	if !txs[100].IsPool() {
		t.Error("5 must is pool")
	}
	txs, err = bi.ListTxs(b)
	if err != nil {
		panic(err)
	}
	if len(txs) != 2 {
		t.Error("b txs error")
	}
	if !txs[0].IsPool() {
		t.Error("b 0 must is pool")
	}
	if !txs[1].IsPool() {
		t.Error("b 0 must is pool")
	}
	txs, err = bi.ListTxs(c)
	if err != nil {
		panic(err)
	}
	if len(txs) != 1 {
		t.Error("c txs error")
	}
	if !txs[0].IsPool() {
		t.Error("c 0 must is pool")
	}

	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	calcHash(blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		panic(err)
	}
	//打包后检查a 新块奖励50+交易费2
	err = checkBalance(bi, a, 100*50*COIN-15*COIN-1*COIN+50*COIN+2*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 15*COIN-5*COIN-1*COIN)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, c, 5*COIN)
	if err != nil {
		panic(err)
	}

	txs, err = bi.ListTxs(a)
	if err != nil {
		panic(err)
	}
	//a新增了coinbasetx
	if len(txs) != 102 {
		t.Error("a txs error")
	}
	if txs[6].IsPool() {
		t.Error("5 not is pool")
	}
	txs, err = bi.ListTxs(b)
	if err != nil {
		panic(err)
	}
	if len(txs) != 2 {
		t.Error("b txs error")
	}
	if txs[0].IsPool() {
		t.Error("b 0 not is pool")
	}
	if txs[1].IsPool() {
		t.Error("b 0 not is pool")
	}
	txs, err = bi.ListTxs(c)
	if err != nil {
		panic(err)
	}
	if len(txs) != 1 {
		t.Error("c txs error")
	}
	if txs[0].IsPool() {
		t.Error("c 0 must is pool")
	}

	//清空后都应该没余额
	removeAll(bi)

	err = checkBalance(bi, a, 0)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, b, 0)
	if err != nil {
		panic(err)
	}
	err = checkBalance(bi, c, 0)
	if err != nil {
		panic(err)
	}

	// all 0
	txs, err = bi.ListTxs(a)
	if err != nil {
		panic(err)
	}
	if len(txs) != 0 {
		t.Error("a txs error")
	}
	txs, err = bi.ListTxs(b)
	if err != nil {
		panic(err)
	}
	if len(txs) != 0 {
		t.Error("b txs error")
	}
	txs, err = bi.ListTxs(c)
	if err != nil {
		panic(err)
	}
	if len(txs) != 0 {
		t.Error("c txs error")
	}
}

func TestRefTxDels(t *testing.T) {
	var err error
	a := Address("st1qresg66j0t9c8c9awxfkeremk0fwgha06hwuw6q")
	b := Address("st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3")
	c := Address("st1qm24876nvtcn83m8jlg7r4jsr223lcepn3g8wt3")
	bi := GetTestBlockIndex()
	defer bi.Close()
	removeAll(bi)
	defer removeAll(bi)

	//生成5个区块
	createBlock(bi, 100)

	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	//A -> B = 15 fee=1
	mi := bi.NewMulTrans()
	mi.Acts = []*Account{getacc(bi, a)}
	mi.Keep = 0
	mi.Spent = blk.Meta.Height
	mi.Dst = []Address{b}
	mi.Amts = []Amount{15 * COIN}
	mi.Fee = 1 * COIN
	mi.Ext = []byte{}
	tx1, err := mi.NewTx(true)
	if err != nil {
		panic(err)
	}
	err = blk.AddTx(bi, tx1)
	if err != nil {
		panic(err)
	}
	//B -> C =5 fee=1
	mi = bi.NewMulTrans()
	mi.Acts = []*Account{getacc(bi, b)}
	mi.Keep = 0
	mi.Spent = blk.Meta.Height
	mi.Dst = []Address{c}
	mi.Amts = []Amount{5 * COIN}
	mi.Fee = 1 * COIN
	mi.Ext = []byte{}

	tx2, err := mi.NewTx(true)
	if err != nil {
		panic(err)
	}
	err = blk.AddTx(bi, tx2)
	if err != nil {
		panic(err)
	}

	//移除tx1 将会移除tx2，因为tx2引用了tx1
	bi.txp.Del(bi, tx1.MustID())

	if bi.txp.Len() != 0 {
		t.Errorf("TestRefTxDels error")
	}
}
