package xginx

import "testing"

func TestTxPool(t *testing.T) {
	tp := NewTxPool()

	tx1 := &TX{}
	tx1.Ver = 1
	tx1.Ins = []*TxIn{}
	out1 := &TxOut{}
	out1.Value = 100
	out1.Script, _ = NewLockedScript(HASH160{1})
	tx1.Outs = []*TxOut{out1}
	tx1.LockTime = 0

	err := tp.PushBack(tx1)
	if err != nil {
		panic(err)
	}
	ck := &CoinKeyValue{}
	ck.CPkh = HASH160{1}
	ck.Index = 0
	ck.TxId, _ = tx1.ID()
	if !tp.HasCoin(ck) {
		t.Error("has coin error")
	}
	ds, _ := tp.ListCoins(HASH160{1})
	if len(ds) != 1 {
		t.Error("list coins error")
	}
}
