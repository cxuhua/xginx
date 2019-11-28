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
	b := blk.Header.Bytes()
	for i := uint32(0); ; i++ {
		b.SetNonce(i)
		id := b.Hash()
		if CheckProofOfWork(id, blk.Header.Bits) {
			blk.Header = b.Header()
			break
		}
	}
	if bi.Len() == 0 {
		conf.genesis, _ = blk.ID()
	}
	return blk
}

func getTestBi() *BlockIndex {
	conf = LoadConfig("test.json")
	lis := newListener(conf.WalletDir)
	InitBlockIndex(lis)
	lis.OnStartup()

	bi := GetBlockIndex()
	if bi.Len() > 0 {
		conf.genesis, _ = bi.First().ID()
	}
	return bi
}

//从h开始生成n个区块头
func makehs(h BlockHeader, n int) []BlockHeader {
	hs := []BlockHeader{h}
	for i := 0; i < n; i++ {
		time.Sleep(time.Second)
		v := h
		v.Time = uint32(time.Now().Unix())
		v.Prev, _ = h.ID()
		b := v.Bytes()
		for i := uint32(0); ; i++ {
			b.SetNonce(i)
			id := b.Hash()
			if CheckProofOfWork(id, v.Bits) {
				v = b.Header()
				break
			}
		}
		hs = append(hs, v)
		h = v
	}
	return hs
}

func TestMergeChain(t *testing.T) {
	bi := getTestBi()
	defer bi.Close()

	iter := bi.NewIter()
	iter.SeekHeight(3)

	vvs := makehs(iter.Curr().BlockHeader, 12)

	log.Println(bi.MergeHead(vvs))
}

func TestBlockChain(t *testing.T) {
	bi := getTestBi()
	defer bi.Close()
	testnum := uint32(5)
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
	mi := bi.EmptyMulTransInfo()
	mi.Src = []Address{"st1qcenzwakw5mfmh93thjzk4deveeue2n8yrw526v"}
	mi.Keep = 0
	mi.Dst = []Address{"st1qr9k57te9vvxr7wpy8ua25jj9f02k0kr6vqzl9w"}
	mi.Amts = []Amount{0}
	mi.Fee = 1 * COIN
	mi.Ext = []byte{}
	//A -> B
	tx, err := mi.NewTx(false)
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
	log.Println(blk.GetFee(bi))
	log.Println(blk.CoinbaseFee())
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
