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
	a32 = "XrxPtJaoXqJWg79S931ceyMsYcgMCUXousiVm2UB7PnVfWCD9RB5gpQ5mWGYGgemTUTXfcpGv4G4x8v2ErgW5TstcLPRW927Jxzj1tyqQmcCDdB5xFgKv4c6xXEHfnUzThURZh26L5tzScWSJ62NFkT4gPoosrkTFMdUvxZT1mxiX6WBa6PL4YVxnhFtinYn9FSqqeF7VKEbDWQC4jPJt9Df86TndKcB2sfPzX5LfAVcBPieDVKRXPm93SBp2hTJXZXyDRV1U8c2j6n2V5pyYhpRGDbH3iFpWJFuQGDCKto3dP8aM9hxTbXWHf95Zywg5ZZnTXwobhxwz7khcn1JWhdwni9cXqzhddzhGZVmYS7mFqJY5hGR814uDrsB18yCFCyBRMELRq6GMTaafoo7MaBgjzxKYkfeneXrCx4XrX2G3QjiThhkSrX1HQGeCWdp8iNSsjE1zjrZFivgamUJQ46JVWNxd8LChKjuLfuexcG4uDairUptYqUm7hUT"
	//3-3签名
	a33 = "XrxPtJaoXqJWg79S931ceyMsYcgMCUXousiVm2UB7PnVfWCD9RB5gpQ5mWGYGgemTUTXfcpGv4G4x8v2ErgW5TstcLPRW927Jxzj1tyqQmcCDdB5xFgKv4c6xXEHfnUzThURZh26L5tzScWSJ62NFkT4gPoosrkTFMdUvxZT1mxiX6WBa6PL4YVxnhFtinYn9FSqqeF7VKEbDWQC4jPJt9Df86TndKcB2sfPzX5LfAVcBPieDVKRXPm93SBp2hTJXZXyDRV1U8c2j6n2V5pyYhpRGDbH3iFpWJFuQGDCKto3dP8aM9hxTbXWHf95Zywg5ZZnTXwobhxwz7khcn1JWhdwni9cXqzhddzhGZVmYS7mFqJY5hGR814uDrsB18yCFCyBRMELRq6GMTaafoo7MaBgjzxKYkfeneXrCx4XrX2G3QjiThhkSrX1HQGeCWdp8iNSsjE1zjrZFivgamUJQ46JVWNxd8LChKjuLfuexcG4uDairUptYqUm7hUT"

	TestAccount, _ = LoadAccount(a32)
	DstAccount, _  = LoadAccount(a33)
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
	if script, err := TestAccount.NewLockedScript(); err != nil {
		return err
	} else {
		out.Script = script
	}
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
	//addr, err := out.Script.GetAddress()
	//if err != nil {
	//	return nil, err
	//}
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

	testnum := uint32(5)
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
	pkh, err := TestAccount.GetPkh()
	if err != nil {
		panic(err)
	}
	ds, err := bi.ListCoinsWithID(pkh)
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
	if script, err := TestAccount.NewLockedScript(); err != nil {
		panic(err)
	} else {
		txout.Script = script
	}
	for _, v := range ds {
		in, err := v.GetTxIn(TestAccount)
		if err != nil {
			panic(err)
		}
		ins = append(ins, in)
		txout.Value += v.Value
	}
	txout.Value -= 1 * COIN //给点交易费
	outs := []*TxOut{txout}
	tx1.Ins = ins
	tx1.Outs = outs

	id1, err := tx1.ID()
	if err != nil {
		panic(err)
	}
	//为每个输入添加签名
	err = tx1.Sign(bi, b)
	if err != nil {
		panic(err)
	}
	tx1.ResetAll()
	id2, err := tx1.ID()
	if err != nil {
		panic(err)
	}
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

func TestTransfire(t *testing.T) {
	addr, err := DstAccount.GetAddress()
	if err != nil {
		panic(err)
	}
	bi := NewBlockIndex(&tlis{})
	err = bi.LoadAll(func(pv uint) {
		log.Printf("load block chian %d%%\n", pv)
	})
	if err != nil {
		panic(err)
	}
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}

	tx, err := bi.Transfre(blk, TestAccount, addr, 3*COIN, 1*COIN)
	if err != nil {
		panic(err)
	}
	err = blk.AddTx(bi, tx)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	if err := blk.CalcPowHash(1000000, bi); err != nil {
		panic(err)
	}
	err = bi.LinkTo(blk)
	if err != nil {
		panic(err)
	}
	ds, err := bi.ListCoins(addr)
	if err != nil {
		panic(err)
	}
	log.Println(ds)
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
	pkh, err := TestAccount.GetPkh()
	if err != nil {
		panic(err)
	}
	ds, err := bi.ListCoinsWithID(pkh)
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
	if script, err := TestAccount.NewLockedScript(); err != nil {
		panic(err)
	} else {
		txout.Script = script
	}
	for _, v := range ds {
		in, err := v.GetTxIn(TestAccount)
		if err != nil {
			panic(err)
		}
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
	in2.OutHash, _ = tx1.ID()
	in2.OutIndex = 0

	ins2 := []*TxIn{in2}

	out2 := &TxOut{}

	if script, err := TestAccount.NewLockedScript(); err != nil {
		panic(err)
	} else {
		out2.Script = script
	}
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
	addr, err := TestAccount.GetAddress()
	if err != nil {
		panic(err)
	}
	//获取矿工的所有输出
	ds, err := bi.ListCoins(addr)
	if err != nil {
		panic(err)
	}

	b, err := bi.NewBlock(1)
	//组装交易
	tx := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	script, err := TestAccount.NewLockedScript()
	if err != nil {
		panic(err)
	}
	txout.Script = script
	for _, v := range ds {
		in, err := v.GetTxIn(TestAccount)
		if err != nil {
			panic(err)
		}
		ins = append(ins, in)
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
