package xginx

import (
	"testing"
	"time"
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
	ds, err = bi.ListTokens(TestTagPri.PublicKey().Hash())
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
		txout.Value += v.Value
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

	cli := TestCliPri.PublicKey()

	cliBestId, _ := bi.GetCliBestId(cli.Hash())

	u1 := &Unit{}
	u1.CPks.Set(cli)
	u1.Prev = cliBestId
	SetRandInt(&u1.Nonce)
	u1.STime = time.Now().UnixNano()

	u2 := &Unit{}
	u2.CPks.Set(cli)
	u2.Prev = u1.Hash()
	SetRandInt(&u2.Nonce)
	u2.STime = time.Now().UnixNano()

	us := &Units{u1, u2}

	err = b.AddUnits(bi, us)
	if err != nil {
		panic(err)
	}

	calcer := NewTokenCalcer(TestMinePri.PublicKey().Hash())
	err = b.Finish(bi, calcer)
	if err != nil {
		panic(err)
	}
}
