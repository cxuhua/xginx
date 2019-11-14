package xginx

import (
	"errors"
	"log"
	"testing"
	"time"
)

var (
	//测试用矿工私钥
	TestMinerPrivateKey = "L4eSSzfWoTB9Y3eZo4Wp9TBPBsTJCcwmbioRcda3cM86MnUMrXhN"
	TestMinePri, _      = LoadPrivateKey(TestMinerPrivateKey)
	//测试用客户端key
	TestCliPrivateKey = "KzVa4aqLziZWuiKFkPRkM46ZTrdzJhfuUxbe8pmxgosjoEYYnZuM"
	TestCliPri, _     = LoadPrivateKey(TestCliPrivateKey)
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
	script, err := NewStdLockedScript(TestMinePri.PublicKey())
	if err != nil {
		return err
	}
	//设置base out script
	//创建coinbase tx
	tx := &TX{}
	tx.Ver = 1

	//base tx
	in := &TxIn{}
	in.ExtBytes = []byte("ext data test")
	in.Script = blk.CoinbaseScript([]byte("Test Block"))
	tx.Ins = []*TxIn{in}

	out := &TxOut{}
	out.Value = blk.CoinbaseReward()
	out.Script = script
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
func (lis *tlis) OnPrivateKey(bi *BlockIndex, blk *BlockInfo, out *TxOut) (*PrivateKey, error) {
	return TestMinePri, nil
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
	err = blk.Check(bi)
	if err != nil {
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
	if bi.Len() != int(testnum) {
		t.Errorf("add main chain error")
		t.FailNow()
	}
	bi.db.Sync()
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
	ds, err := bi.ListCoinsWithID(TestMinePri.PublicKey().Hash())
	if err != nil {
		panic(err)
	}

	b, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	defer b.Clean(bi)
	//组装交易
	tx1 := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script, _ = NewStdLockedScript(TestMinePri.PublicKey())
	for _, v := range ds {
		ins = append(ins, v.GetTxIn())
		txout.Value += v.Value.ToAmount()
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
	in2.OutHash = tx1.Hash()
	in2.OutIndex = 0
	in2.ExtBytes = []byte{1, 1, 9}

	ins2 := []*TxIn{in2}

	out2 := &TxOut{}
	out2.Script, _ = NewStdLockedScript(TestMinePri.PublicKey())
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
	err = b.Check(bi)
	if err != nil {
		panic(err)
	}
	err = bi.LinkTo(b)
	if err != nil {
		panic(err)
	}
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
	ds, err := bi.ListCoinsWithID(TestMinePri.PublicKey().Hash())
	if err != nil {
		panic(err)
	}

	b, err := bi.NewBlock(1)
	//组装交易
	tx := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script, _ = NewStdLockedScript(TestMinePri.PublicKey())
	for _, v := range ds {
		ins = append(ins, v.GetTxIn())
		txout.Value += v.Value.ToAmount()
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
	err = b.Check(bi)
	if err != nil {
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
	for {
		bv := bi.GetBestValue()
		if !bv.IsValid() {
			log.Println("not has best block")
			break
		}
		last := bi.Last()
		if !bv.Id.Equal(last.ID()) {
			panic(errors.New("best id error"))
		}
		if bv.Height != last.Height {
			panic(errors.New("best height error"))
		}
		b, err := bi.LoadBlock(last.ID())
		if err != nil {
			panic(err)
		}
		err = bi.Unlink(b)
		if err != nil {
			panic(err)
		}
	}
	bi.db.Sync()
}
