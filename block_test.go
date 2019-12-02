package xginx

import (
	"log"
	"testing"
	"time"
)

func calcHash(blk *BlockInfo) {
	b := blk.Header.Bytes()
	for i := uint32(0); ; i++ {
		b.SetNonce(i)
		id := b.Hash()
		if CheckProofOfWork(id, blk.Header.Bits) {
			blk.Header = b.Header()
			break
		}
	}
}

func NewTestBlock(bi *BlockIndex) *BlockInfo {
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	calcHash(blk)
	if bi.Len() == 0 {
		conf.genesis, _ = blk.ID()
	}
	return blk
}

func GetTestBlockIndex() *BlockIndex {
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

func TestBlockChain(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	testnum := uint32(2017)
	for i := uint32(0); i < testnum; i++ {
		time.Sleep(time.Second)
		cb := NewTestBlock(bi)
		err := bi.LinkBlk(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i > 0 && i%10000 == 0 {
			log.Println(i, "block")
		}
	}
}

func getacc(bi *BlockIndex, addr Address) *Account {
	acc, err := bi.lptr.GetWallet().GetAccount(addr)
	if err != nil {
		panic(err)
	}
	return acc
}

func TestTransfire(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	mi := bi.EmptyMulTransInfo()
	mi.Acts = []*Account{getacc(bi, "st1qresg66j0t9c8c9awxfkeremk0fwgha06hwuw6q")}
	mi.Keep = 0
	mi.Dst = []Address{"st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3"}
	mi.Amts = []Amount{15 * COIN}
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

	ds, err := bi.ListCoins("st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3")
	if err != nil {
		panic(err)
	}
	log.Println(ds)

	mi = bi.EmptyMulTransInfo()
	mi.Acts = []*Account{getacc(bi, "st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3")}
	mi.Keep = 0
	mi.Dst = []Address{"st1qm24876nvtcn83m8jlg7r4jsr223lcepn3g8wt3"}
	mi.Amts = []Amount{5 * COIN}
	mi.Fee = 1 * COIN
	mi.Ext = []byte{}
	//B -> C
	tx, err = mi.NewTx(true)
	if err != nil {
		panic(err)
	}

	err = blk.AddTx(bi, tx)
	if err != nil {
		panic(err)
	}

	ds, err = bi.ListCoins("st1qm24876nvtcn83m8jlg7r4jsr223lcepn3g8wt3")
	if err != nil {
		panic(err)
	}
	log.Println(ds)

	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	err = blk.Check(bi)
	if err != nil {
		panic(err)
	}
	calcHash(blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		panic(err)
	}
	ds, err = bi.ListCoins("st1qm24876nvtcn83m8jlg7r4jsr223lcepn3g8wt3")
	if err != nil {
		panic(err)
	}
	log.Println(ds)
}

func TestUnlinkBlock(t *testing.T) {
	bi := GetTestBlockIndex()
	defer bi.Close()
	err := bi.UnlinkLast()
	if err != nil {
		panic(err)
	}
}
