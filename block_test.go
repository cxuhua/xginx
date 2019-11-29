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
	bi := GetTestBlockIndex()
	defer bi.Close()

	iter := bi.NewIter()
	iter.SeekHeight(3)

	vvs := makehs(iter.Curr().BlockHeader, 3)

	log.Println(bi.MergeHead(vvs[1:]))
}

func TestBlockChain(t *testing.T) {
	bi := GetTestBlockIndex()
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
	bi := GetTestBlockIndex()
	defer bi.Close()
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	mi := bi.EmptyMulTransInfo()
	mi.Src = []Address{"st1qresg66j0t9c8c9awxfkeremk0fwgha06hwuw6q"}
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
	mi.Src = []Address{"st1q8rdl75cy8qsuy7lteyvrf6q92q2wfrrc5xdvp3"}
	mi.Keep = 0
	mi.Dst = []Address{"st1qm24876nvtcn83m8jlg7r4jsr223lcepn3g8wt3"}
	mi.Amts = []Amount{5 * COIN}
	mi.Fee = 1 * COIN
	mi.Ext = []byte{}
	//B -> C
	tx, err = mi.NewTx(false)
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
	_, err = bi.LinkHeader(blk.Header)
	if err != nil {
		panic(err)
	}
	err = bi.UpdateBlk(blk)
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
