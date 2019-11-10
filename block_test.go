package xginx

import (
	"errors"
	"log"
	"testing"
)

var (
	//测试用矿工私钥
	TestMinerPrivateKey = "L4eSSzfWoTB9Y3eZo4Wp9TBPBsTJCcwmbioRcda3cM86MnUMrXhN"
	TestMinePri, _      = LoadPrivateKey(TestMinerPrivateKey)
	//测试用客户端key
	TestCliPrivateKey = "KzVa4aqLziZWuiKFkPRkM46ZTrdzJhfuUxbe8pmxgosjoEYYnZuM"
	TestCliPri, _     = LoadPrivateKey(TestCliPrivateKey)
)

func NewTestBlock(bi *BlockIndex) *BlockInfo {
	b := bi.NewBlock()
	out := &TxOut{}
	out.Value = GetCoinbaseReward(b.Meta.Height)
	out.Script = StdLockedScript(TestMinePri.PublicKey())
	b.Txs[0].Outs = []*TxOut{out}
	err := b.Finish(bi)
	if err != nil {
		panic(err)
	}
	return b
}

func TestBlockChain(t *testing.T) {
	bi := NewBlockIndex()
	testnum := uint32(10)
	fb := NewTestBlock(bi)
	conf.genesisId = fb.ID()
	log.Println("genesis_block=", fb.ID())
	_, err := bi.LinkTo(fb)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	for i := uint32(1); i < testnum; i++ {
		cb := NewTestBlock(bi)
		_, err = bi.LinkTo(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i%10000 == 0 {
			log.Println(i, "block")
		}
		fb = cb
	}
	if bi.Len() != int(testnum) {
		t.Errorf("add main chain error")
		t.FailNow()
	}
	bi.db.Sync()
}

func TestLRUCache(t *testing.T) {
	c := NewCache(4)
	id := HASH256{1}
	v1 := c.Get(id, func() (size int, value Value) {
		log.Println("a1")
		return 5, 100
	})

	log.Println(v1.Value())

	v1 = c.Get(HASH256{3}, func() (size int, value Value) {
		log.Println("a2")
		return 5, 101
	})
	v1.Release()

	v2 := c.Get(HASH256{3}, func() (size int, value Value) {
		log.Println("a3")
		return 5, 102
	})
	log.Println(v2.Value())
}

func TestBlockSign(t *testing.T) {

	bi := NewBlockIndex()
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}

	//获取矿工的所有输出
	ds, err := bi.ListTokens(TestMinePri.PublicKey().Hash())
	if err != nil {
		panic(err)
	}
	//获取标签的输出
	//ds, err = bi.ListTokens(TestTagPri.PublicKey().Hash())
	//if err != nil {
	//	panic(err)
	//}
	////获取用户的输出
	//ds, err = bi.ListTokens(TestCliPri.PublicKey().Hash())
	//if err != nil {
	//	panic(err)
	//}

	b := bi.NewBlock()
	//组装交易
	tx := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script = StdLockedScript(TestMinePri.PublicKey())
	for _, v := range ds {
		ins = append(ins, v.GetTxIn())
		txout.Value += v.Value.ToAmount()
	}
	outs := []*TxOut{txout}
	tx.Ins = ins
	tx.Outs = outs
	//添加签名
	err = tx.Sign(bi)
	if err != nil {
		panic(err)
	}
	err = b.AddTx(bi, tx)
	if err != nil {
		panic(err)
	}
}

func TestUnlinkBlock(t *testing.T) {
	bi := NewBlockIndex()
	err := bi.LoadAll()
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
