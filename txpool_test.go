package xginx

import "testing"

//测试从交易池移除交易
func TestTxPoolRemoveTx(t *testing.T) {
	bi := NewTestBlockIndex(100)
	defer bi.Close()
	lis := bi.lptr.(*TestLis)
	mi := bi.NewMulTrans()
	txs := []*TX{}
	ref := ZERO256
	dst, _ := lis.ams[1].GetAddress()
	for i := 0; i < 5; i++ {
		//0 -> 1
		mi.Acts = []*Account{lis.ams[0]}
		mi.Dst = []Address{dst}
		mi.Amts = []Amount{2 * COIN}
		mi.Fee = 1 * COIN
		//创建交易
		tx, err := mi.NewTx(true)
		if err != nil {
			t.Fatal(err)
		}
		err = tx.Check(bi, true)
		if err != nil {
			t.Fatal(err)
		}
		txs = append(txs, tx)
		if i == 0 {
			ref = tx.MustID()
		}
	}
	if bi.txp.Len() != 5 {
		t.Fatal("tx pool count error")
	}
	ds, err := bi.ListCoins(dst)
	if err != nil {
		t.Fatal(err)
	}
	//目标应该有5笔金额
	if len(ds.All) != 5 {
		t.Fatal("dst coins error")
	}
	//移除交易池，其他交易应该是引用此交易的，会被自动删除
	bi.txp.Del(bi, ref)
	if bi.txp.Len() != 0 {
		t.Fatal("tx pool count error")
	}
}
