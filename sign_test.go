package xginx

import (
	"testing"
)

func TestSign(t *testing.T) {
	bi := NewBlockIndex()
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}

	ds, err := bi.ListTokens(TestMinePri.PublicKey().Hash())
	if err != nil {
		panic(err)
	}
	ds, err = bi.ListTokens(TestCliPri.PublicKey().Hash())
	if err != nil {
		panic(err)
	}

	b := bi.NewBlock()
	//组装交易
	tx := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script = StdLockedScript(conf.minerpk)
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
	err = b.Finish(bi)
	if err != nil {
		panic(err)
	}
}
