package xginx

import (
	"errors"
	"log"
	"testing"
	"time"
)

var (
	//1-1
	//3-2
	a32 = "ioYe4X4Bwi3KxzVy45g6LvPgcXAx98H36csyb3WWt8VWZAkBX24toagwGEiXm9dhkj8rNQZmeerUkM8pQFKDQ5hKqW3UvEX394RTkf8KuMPtuLiACzNhRvW1UbwMXE3c7JBNkdFM9k5fKREcCoKLbCQMHbQbkQnLBioQeCKJM4YK61FPS22qqvTQuaZts8EV8L5XAMK7Nxf6TZaBR3J4VtKG37JjnaFucZo4BLyLs7cf8JvrNZCCbyqAmZWeZcacUseCwk7FoaudbL5x24wUmtrzv5DeuSVkgqDGGyLocVPKVTfxoZPqtkBTgt6DqRjPi4Bgb1G913fKFMKL1TvXDpS2adQNkakEGor1jfKdmPf8TiaGDymuiXuVnkRn5Q2Gn2ZfTD4LafiwaRBEKoZqmAt8NQtTPy8ZTSp74kzL1khUpkan5SgodxswtTwDAATWehZiBviG8ReewoABBDNNyWS5ViRUrf1peUH4fGQKG2XUj5aa3"
	//3-3签名
	a33 = "ioYe4X4Bwi3KxzVy45g6LvQJjs2nnxLdHbEWX88QunLuPW3oMj1zLCJfndQFqgJtzSUsuJd3iACTvig5LEEp7k92V8uCMrqpEUzpCJ1qtgByCS4mneJAoQiMVxKYnmGwngHrQRrCFi94qHFMS3QTinGPFdvxacjNqjj7mzWp5rRehF297VkvXp1gd8gDQtxz1qU4bFxqoS9i2uLzxAcJUSxe5Nu8DUPXgyAAg3bGzhBXGbdfAqHUutErw5Hj5pc97B9VGcBy1GfNWJqb8jCJL1FX5BrrgeubQaVVkUPwvBbLSMdy42qiRUgQ7CbWkeoU4xvwvDwC8CFo2GNgeaXpfgge61vwbSnCY8dGCGwtxQzP1aY6twcTZCWDRjmg3yPBjE8bSPHYrPADQZUETPGgoMb36DmpxiXAXp5SusRhtswxQvyDZV8NhHwGSVND89WVbLmXvAAtvgS5kyxm2VkDYxjBPBuunkJgtLLheQrHfsNvU1hXH"

	TestAccount, _ = LoadAccount(a32)
)

func TestBlockHeader(t *testing.T) {
	h := BlockHeader{}
	h.Time = 1
	h.Nonce = 2

	b := h.Bytes()
	b.SetTime(time.Now())
	b.SetNonce(4)

	h2 := b.Header()

	if h2.Time != 3 {
		t.Errorf("time set error")
	}

	if h2.Nonce != 4 {
		t.Errorf("nonce set error")
	}
}

//测试用监听器
type tlis struct {
}

func (lis *tlis) OnClose(bi *BlockIndex) {

}

func (lis *tlis) OnLinkBlock(bi *BlockIndex, blk *BlockInfo) {
}

//当块创建完毕
func (lis *tlis) OnNewBlock(bi *BlockIndex, blk *BlockInfo) error {

	//设置base out script
	//创建coinbase tx
	tx := &TX{}
	tx.Ver = 1

	//base tx
	in := &TxIn{}
	in.Script = blk.CoinbaseScript([]byte("Test Block"))
	tx.Ins = []*TxIn{in}

	out := &TxOut{}
	out.Value = blk.CoinbaseReward()
	out.Script = TestAccount.NewLockedScript()
	tx.Outs = []*TxOut{out}

	blk.Txs = []*TX{tx}

	return nil
}

//完成区块
func (lis *tlis) OnFinished(bi *BlockIndex, blk *BlockInfo) error {
	if len(blk.Txs) == 0 {
		return errors.New("txs miss")
	}
	btx := blk.Txs[0]
	if !btx.IsCoinBase() {
		return errors.New("coinbase tx miss")
	}
	fee, err := blk.GetFee(bi)
	if err != nil {
		return err
	}
	if fee == 0 {
		return nil
	}
	btx.Outs[0].Value += fee
	return blk.CheckTxs(bi)
}

//获取签名私钥
func (lis *tlis) GetAccount(bi *BlockIndex, blk *BlockInfo, out *TxOut) (*Account, error) {
	return TestAccount, nil
}

func NewTestBlock(bi *BlockIndex) *BlockInfo {
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	if err := blk.CalcPowHash(1000000, bi); err != nil {
		panic(err)
	}
	return blk
}

func TestBlockChain(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
	defer bi.Close()
	err := bi.LoadAll(func(pv uint) {
		log.Printf("load block chian %d%%\n", pv)
	})
	if err == EmptyBlockChain {
		log.Println(err)
	} else if err == ArriveFirstBlock {
		log.Println(err)
	} else if err != nil {
		panic(err)
	}

	testnum := uint32(1)
	for i := uint32(0); i < testnum; i++ {
		cb := NewTestBlock(bi)
		err := bi.LinkTo(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i > 0 && i%10000 == 0 {
			log.Println(i, "block")
		}
	}

	bi.db.Sync()
}

func TestMulTxInCostOneTxOut(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
	err := bi.LoadAll(func(pv uint) {
		log.Printf("load block chian %d%%\n", pv)
	})
	if err != nil {
		panic(err)
	}
	//获取矿工的所有输出
	ds, err := bi.ListCoinsWithID(TestAccount.GetPkh())
	if err != nil {
		panic(err)
	}
	b, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	//组装交易
	tx1 := &TX{Ver: 1}
	tx1.SetExt([]byte{1, 2, 3, 4, 5, 6})
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script = TestAccount.NewLockedScript()
	for _, v := range ds {
		in := v.GetTxIn(TestAccount)
		ins = append(ins, in)
		txout.Value += v.Value
	}
	txout.Value -= 1 * COIN //给点交易费
	outs := []*TxOut{txout}
	tx1.Ins = ins
	tx1.Outs = outs

	id1 := tx1.ID()
	//为每个输入添加签名
	err = tx1.Sign(bi, b)
	if err != nil {
		panic(err)
	}
	tx1.ResetAll()
	id2 := tx1.ID()
	if !id1.Equal(id2) {
		panic(errors.New("id error"))
	}
	err = b.AddTx(bi, tx1)
	if err != nil {
		panic(err)
	}
	if err := b.Finish(bi); err != nil {
		panic(err)
	}
	if err := b.CalcPowHash(1000000, bi); err != nil {
		panic(err)
	}
	err = bi.LinkTo(b)
	if err != nil {
		panic(err)
	}
}

//测试一个区块中消费同区块之前的交易
func TestBlockMulTXS(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
	err := bi.LoadAll(func(pv uint) {
		log.Printf("load block chian %d%%\n", pv)
	})
	if err != nil {
		panic(err)
	}
	//获取矿工的所有输出
	ds, err := bi.ListCoinsWithID(TestAccount.GetPkh())
	if err != nil {
		panic(err)
	}
	b, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	//组装交易
	tx1 := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script = TestAccount.NewLockedScript()
	for _, v := range ds {
		in := v.GetTxIn(TestAccount)
		ins = append(ins, in)
		txout.Value += v.Value
	}
	outs := []*TxOut{txout}
	tx1.Ins = ins
	tx1.Outs = outs
	//为每个输入添加签名
	err = tx1.Sign(bi, b)
	if err != nil {
		panic(err)
	}
	err = b.AddTx(bi, tx1)
	if err != nil {
		panic(err)
	}
	//交易2消费交易1
	tx2 := &TX{}
	tx2.Ver = 1

	in2 := &TxIn{}
	in2.OutHash = tx1.ID()
	in2.OutIndex = 0

	ins2 := []*TxIn{in2}

	out2 := &TxOut{}

	out2.Script = TestAccount.NewLockedScript()
	out2.Value = txout.Value

	outs2 := []*TxOut{out2}
	tx2.Ins = ins2
	tx2.Outs = outs2
	//为每个输入添加签名
	err = tx2.Sign(bi, b)
	if err != nil {
		panic(err)
	}
	err = b.AddTx(bi, tx2)
	if err != nil {
		panic(err)
	}
	if err := b.Finish(bi); err != nil {
		panic(err)
	}
	if err := b.CalcPowHash(1000000, bi); err != nil {
		panic(err)
	}
	err = bi.LinkTo(b)
	if err != nil {
		panic(err)
	}
}

func TestListAddressCoins(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
	err := bi.LoadAll(func(pv uint) {
		log.Printf("load block chian %d%%\n", pv)
	})
	if err != nil {
		panic(err)
	}
	//获取矿工的所有输出
	ds, err := bi.ListCoins("st1q363x0zvheem0a5f0r0z9qr9puj7l900jc8glh0")
	if err != nil {
		panic(err)
	}
	log.Println(ds)
}

func TestBlockSign(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
	err := bi.LoadAll(func(pv uint) {
		log.Printf("load block chian %d%%\n", pv)
	})
	if err != nil {
		panic(err)
	}
	//获取矿工的所有输出
	ds, err := bi.ListCoins(TestAccount.GetAddress())
	if err != nil {
		panic(err)
	}

	b, err := bi.NewBlock(1)
	//组装交易
	tx := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script = TestAccount.NewLockedScript()
	for _, v := range ds {
		ins = append(ins, v.GetTxIn(TestAccount))
		txout.Value += v.Value
	}
	outs := []*TxOut{}
	tx.Ins = ins
	tx.Outs = outs
	//为每个输入添加签名
	err = tx.Sign(bi, b)
	if err != nil {
		panic(err)
	}
	err = b.AddTx(bi, tx)
	if err != nil {
		panic(err)
	}
	if err := b.Finish(bi); err != nil {
		panic(err)
	}
	if err := b.CalcPowHash(1000000, bi); err != nil {
		panic(err)
	}
	err = bi.LinkTo(b)
	if err != nil {
		panic(err)
	}
}

func TestUnlinkBlock(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
	err := bi.LoadAll(func(pv uint) {
		log.Printf("load block chian %d%%\n", pv)
	})
	if err != nil {
		panic(err)
	}
	err = bi.UnlinkLast()
	if err != nil {
		panic(err)
	}
}
