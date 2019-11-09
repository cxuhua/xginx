package xginx

import (
	"log"
	"testing"
)

func TestSign(t *testing.T) {
	bi := NewBlockIndex()
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}
	ds, err := bi.ListTokens(conf.minerpk.Hash())
	if err != nil {
		panic(err)
	}
	//组装交易
	tx := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script = StdLockedScript(conf.minerpk)
	for _, v := range ds {
		ins = append(ins, v.GetTxIn())
		txout.Value += v.Value
	}
	outs := []*TxOut{txout}
	tx.Ins = ins
	tx.Outs = outs

	err = tx.Sign(bi)
	if err != nil {
		panic(err)
	}
	last := bi.Last()

	b := NewBlock(last.Height+1, last.ID())
	b.Txs = append(b.Txs, tx)
	err = b.SetMerkle()
	if err != nil {
		panic(err)
	}
	ele, err := bi.LinkTo(b)
	if err != nil {
		panic(err)
	}
	log.Println(ele)
}
