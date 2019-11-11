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

//测试用监听器
type tlis struct {
}

//当块创建完毕
func (lis *tlis) OnNewBlock(bi *BlockIndex, blk *BlockInfo) error {
	script, err := NewStdLockedScript(TestMinePri.PublicKey())
	if err != nil {
		return err
	}
	//设置base out script
	//创建coinbase tx
	btx := &TX{}
	btx.Ver = 1

	//base tx
	in := &TxIn{}
	in.Script = BaseScript(blk.Meta.Height, []byte("Test Block"))
	btx.Ins = []*TxIn{in}

	out := &TxOut{}
	out.Value = GetCoinbaseReward(blk.Meta.Height)
	out.Script = script
	btx.Outs = []*TxOut{out}

	blk.Txs = []*TX{btx}

	return nil
}

//完成区块
func (lis *tlis) OnFinished(bi *BlockIndex, blk *BlockInfo) error {
	return nil
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
	err = blk.Check(bi)
	if err != nil {
		panic(err)
	}
	return blk
}

func TestBlockChain(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
	testnum := uint32(210001)
	for i := uint32(0); i < testnum; i++ {
		cb := NewTestBlock(bi)
		_, err := bi.LinkTo(cb)
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

func TestBlockSign(t *testing.T) {

	bi := NewBlockIndex(&tlis{})
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}
	//获取矿工的所有输出
	ds, err := bi.ListTokens(TestMinePri.PublicKey().Hash())
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
	for i, v := range ds {
		ins = append(ins, v.GetTxIn())
		txout.Value += v.Value.ToAmount()
		if i > 500 {
			break
		}
	}
	outs := []*TxOut{txout}
	tx.Ins = ins
	tx.Outs = outs
	//添加签名
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
	_, err = bi.LinkTo(b)
	if err != nil {
		panic(err)
	}
}

func TestUnlinkBlock(t *testing.T) {
	bi := NewBlockIndex(&tlis{})
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
