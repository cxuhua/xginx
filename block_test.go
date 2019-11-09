package xginx

import (
	"errors"
	"log"
	"math/rand"
	"testing"
	"time"
)

func NewBlock(bi *BlockIndex) *BlockInfo {
	b := bi.NewBlock()

	u1 := &Unit{}
	SetRandInt(&u1.Nonce)
	u1.STime = time.Now().UnixNano()
	u2 := &Unit{}
	SetRandInt(&u2.Nonce)
	u2.STime = time.Now().UnixNano()

	us := &Units{u1, u2}
	b.Uts = []*Units{us}

	tx := &TX{}
	txin := &TxIn{
		Script: BaseScript(b.Meta.Height, []byte("test script")),
	}
	txout := &TxOut{
		Value:  VarUInt(rand.Uint32() % 1000),
		Script: StdLockedScript(conf.minerpk),
	}
	tx.Ins = []*TxIn{txin}
	tx.Outs = []*TxOut{txout}
	b.Txs = []*TX{tx}

	err := b.SetMerkle()
	if err != nil {
		panic(err)
	}
	return b
}

func TestBlockChain(t *testing.T) {
	bi := NewBlockIndex()
	testnum := uint32(10)
	fb := NewBlock(bi)
	conf.genesisId = fb.ID()
	log.Println("genesis_block=", fb.ID())
	_, err := bi.LinkTo(fb)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	for i := uint32(1); i < testnum; i++ {
		cb := NewBlock(bi)
		_, err = bi.LinkTo(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i%10000 == 0 {
			log.Println(i, "block")
		}
		fb = cb
	}
	if bi.Len() != int(testnum) {
		t.Errorf("add main chain error")
		t.FailNow()
	}
	bi.db.Sync()
}

func TestUnlinkBlock(t *testing.T) {
	bi := NewBlockIndex()
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}
	for {
		bv := bi.db.GetBestValue()
		if !bv.IsValid() {
			log.Println("not has best block")
			break
		}
		last := bi.Last()
		if !bv.Id.Equal(last.ID()) {
			panic(errors.New("best id error"))
		}
		if bv.Height != last.Height {
			panic(errors.New("best height error"))
		}
		b, err := bi.LoadBlock(last.ID())
		if err != nil {
			panic(err)
		}
		err = bi.Unlink(b)
		if err != nil {
			panic(err)
		}
	}
	bi.db.Sync()
}

func TestValueScale(t *testing.T) {
	log.Println(S721.Scale())
	log.Println(S622.Scale())
	log.Println(S631.Scale())
	log.Println(S640.Scale())
	log.Println(S550.Scale())
}

func TestCalcDistance(t *testing.T) {
	bits := NewUINT256(conf.PowLimit).Compact(false)
	calcer := NewTokenCalcer()
	now := time.Now().UnixNano()
	//i1 first
	i1 := &Unit{}
	i1.TLoc.Set(104.0658044815, 30.5517656113)
	i1.CTime = now
	i1.CLoc.Set(104.0671670437, 30.5573090657)
	i1.STime = now
	i1.TPKH = HASH160{1}
	i1.TASV = S631

	i2 := &Unit{}
	i2.Prev = i1.Hash()
	i2.TLoc.Set(104.0615880489, 30.5536596605)
	i2.CTime = now + int64(time.Hour)
	i2.CLoc.Set(104.0615880489, 30.5536596605)
	i2.STime = now + int64(time.Hour)
	i2.TPKH = HASH160{2}
	i2.TASV = S622

	i3 := &Unit{}
	i3.Prev = i2.Hash()
	i3.Prev = i2.Hash()
	i3.TLoc.Set(104.0671670437, 30.5573090657)
	i3.CTime = now + int64(time.Hour*2)
	i3.CLoc.Set(104.0671670437, 30.5573090657)
	i3.STime = now + int64(time.Hour*2)
	i3.TPKH = HASH160{3}
	i3.TASV = S721

	is := &Units{i1, i2, i3}
	err := calcer.Calc(bits, is)
	log.Println(calcer, err)
}
