package xginx

import (
	"errors"
	"log"
	"testing"
	"time"
)

func NewBlock(h uint32, prev HASH256) *BlockInfo {
	b := &BlockInfo{}
	b.Header.Ver = 1
	b.Header.Prev = prev
	b.Header.Bits = 0x1d00ffff
	b.Header.Time = uint32(time.Now().Unix())
	SetRandInt(&b.Header.Nonce)
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
		Script: BaseScript(h, []byte{}),
	}
	txout := &TxOut{
		Value:  0,
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
	chain := NewBlockIndex()
	testnum := uint32(100000)
	fb := NewBlock(0, HASH256{})
	conf.genesisId = fb.ID()
	log.Println(fb.ID())
	_, err := chain.LinkTo(fb)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	//100万个数据
	for i := uint32(1); i < testnum; i++ {
		cb := NewBlock(i, fb.ID())
		_, err = chain.LinkTo(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i%10000 == 0 {
			log.Println(i, "block")
		}
		fb = cb
	}
	if chain.Len() != int(testnum) {
		t.Errorf("add main chain error")
		t.FailNow()
	}
	chain.store.Sync()
}

func TestUnlinkBlock(t *testing.T) {
	chain := NewBlockIndex()
	err := chain.LoadAll()
	if err != nil {
		panic(err)
	}
	for {
		bv := chain.store.GetBestValue()
		if !bv.IsValid() {
			log.Println("not has best block")
			break
		}
		last := chain.Last()
		if !bv.Id.Equal(last.ID()) {
			panic(errors.New("best id error"))
		}
		if bv.Height != last.Height {
			panic(errors.New("best height error"))
		}
		b, err := chain.LoadBlock(last.ID())
		if err != nil {
			panic(err)
		}
		err = chain.Unlink(b)
		if err != nil {
			panic(err)
		}
	}
	chain.store.Sync()
}

func TestLoadAllBlock(t *testing.T) {
	chain := NewBlockIndex()
	err := chain.LoadAll()
	if err != nil {
		panic(err)
	}
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
