package xginx

import (
	"log"
	"testing"
	"time"
)

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

func getTestBi() *BlockIndex {
	conf = LoadConfig("v10000.json")
	lis := newListener(conf.WalletDir)
	InitBlockIndex(lis)
	lis.OnStartup()
	return GetBlockIndex()
}

func TestBlockChain(t *testing.T) {
	bi := getTestBi()
	defer bi.Close()
	testnum := uint32(1)
	for i := uint32(0); i < testnum; i++ {
		time.Sleep(time.Second)
		cb := NewTestBlock(bi)
		_, err := bi.LinkHeader(cb.Header)
		if err != nil {
			panic(err)
		}
		err = bi.UpdateBlk(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i > 0 && i%10000 == 0 {
			log.Println(i, "block")
		}
	}
}

func TestTransfire(t *testing.T) {
	bi := getTestBi()
	w := bi.lptr.GetWallet()
	log.Println(w.ListAccount())
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	//A -> B
	tx, err := bi.Transfer("st1qhkwszvzcl0qza276afsr5wgfq2dcs03uuq2yuw", "st1qr9k57te9vvxr7wpy8ua25jj9f02k0kr6vqzl9w", 1*COIN, 0)
	if err != nil {
		panic(err)
	}
	err = blk.AddTx(bi, tx)
	if err != nil {
		panic(err)
	}
	//B -> C
	tx, err = bi.Transfer("st1qr9k57te9vvxr7wpy8ua25jj9f02k0kr6vqzl9w", "st1qqgndaafn6lmhnp5mvqm6erh5r35t0rul6wt2t6", 1*COIN, 0)
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
	_, err = bi.LinkHeader(blk.Header)
	if err != nil {
		panic(err)
	}
	err = bi.UpdateBlk(blk)
	if err != nil {
		panic(err)
	}
	ds, err := bi.ListCoins("st1qr9k57te9vvxr7wpy8ua25jj9f02k0kr6vqzl9w")
	if err != nil {
		panic(err)
	}
	log.Println(ds)
}

func TestUnlinkBlock(t *testing.T) {
	conf = LoadConfig("v10000.json")
	InitBlockIndex(newListener(conf.WalletDir))
	bi := GetBlockIndex()
	err := bi.UnlinkLast()
	if err != nil {
		panic(err)
	}
}
