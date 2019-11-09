package xginx

import (
	"errors"
	"log"
	"testing"
)

func TestSign(t *testing.T) {
	bi := NewBlockIndex()
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}

	bv := bi.db.GetBestValue()
	if !bv.IsValid() {
		panic("aa")
	}
	last := bi.Last()
	if !bv.Id.Equal(last.ID()) {
		panic(errors.New("best id error"))
	}
	if bv.Height != last.Height {
		panic(errors.New("best height error"))
	}
	b, err := bi.LoadBlock(last.ID())
	bi.Unlink(b)

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
	last = bi.Last()

	bb := NewBlock(last.Height+1, last.ID())
	b.Txs = append(b.Txs, tx)
	err = b.SetMerkle()
	if err != nil {
		panic(err)
	}
	ele, err := bi.LinkTo(bb)
	if err != nil {
		panic(err)
	}
	log.Println(ele)
}
