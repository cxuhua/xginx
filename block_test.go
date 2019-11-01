package xginx

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"
	"time"
)

func TestCreateGenesisBlock(t *testing.T) {
	pub, err := LoadPublicKey("8aKby6XxwmoaiYt6gUbS1u2RHco37iHfh6sAPstME33Qh6ujd9")
	if err != nil {
		panic(err)
	}
	b := &BlockInfo{
		Ver:    1,
		Prev:   HashID{},
		Merkle: HashID{},
		Time:   uint32(time.Now().Unix()),
		Bits:   0x1d00ffff,
		Uts:    []*Units{},
		Txs:    []*TX{},
	}

	tx := &TX{}
	tx.Ver = 1

	in := &TxIn{}
	in.Script = BaseScript([]byte("The value of a man should be seen in what he gives and not in what he is able to receive."))
	tx.Ins = []*TxIn{in}

	out := &TxOut{}
	out.Value = 529
	out.Script = LockedScript(pub)
	tx.Outs = []*TxOut{out}

	tx.Hash()
	b.Txs = []*TX{tx}

	//生成merkle root id
	if err := b.MerkleRoot(); err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}
	SetRandInt(&b.Nonce)
	err = b.EncodeHeader(buf)
	if err != nil {
		panic(err)
	}
	bb := buf.Bytes()
	nhash := HashID{}
	for i := uint64(0); ; i++ {
		Endian.PutUint32(bb[len(bb)-4:], b.Nonce)
		copy(nhash[:], HASH256(bb))
		if CheckProofOfWork(nhash, b.Bits) {
			log.Printf("%x %v\n", b.Nonce, nhash)
			break
		}
		if i%100000 == 0 {
			SetRandInt(&b.Nonce)
			log.Println(i, nhash, b.Nonce)
			continue
		}
		b.Nonce++
	}
}

func TestSaveBlockInfo(t *testing.T) {
	b := &BlockInfo{}
	b.Ver = 100
	id, meta, bb, err := b.ToTBMeta()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	err = store.UseSession(context.Background(), func(db DBImp) error {
		return db.SetBlock(id, meta, bb)
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	c := &BlockInfo{}
	err = store.UseSession(context.Background(), func(db DBImp) error {
		if !db.HasBlock(id) {
			return errors.New("not found")
		}
		return db.GetBlock(id, c)
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if c.Ver != b.Ver {
		t.Error("error")
	}
	err = store.UseSession(context.Background(), func(db DBImp) error {
		return db.DelBlock(id)
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	err = store.UseSession(context.Background(), func(db DBImp) error {
		if db.HasBlock(id) {
			return errors.New("del error")
		}
		return nil
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}

func TestValueScale(t *testing.T) {
	log.Println(Alloc(S721).Scale())
	log.Println(Alloc(S622).Scale())
	log.Println(Alloc(S631).Scale())
	log.Println(Alloc(S640).Scale())
	log.Println(Alloc(S550).Scale())
}

func TestCalcDistance(t *testing.T) {
	calcer := NewTokenCalcer()
	now := time.Now().UnixNano()
	//i1 first
	i1 := &Unit{}
	i1.TLoc.Set(104.0658044815, 30.5517656113)
	i1.CTime = now
	i1.CLoc.Set(104.0671670437, 30.5573090657)
	i1.STime = now
	i1.TPKH = UserID{1}
	i1.TASV = S631

	i2 := &Unit{}
	i2.Prev = i1.Hash()
	i2.TLoc.Set(104.0615880489, 30.5536596605)
	i2.CTime = now + int64(time.Hour)
	i2.CLoc.Set(104.0615880489, 30.5536596605)
	i2.STime = now + int64(time.Hour)
	i2.TPKH = UserID{2}
	i2.TASV = S622

	i3 := &Unit{}
	i3.Prev = i2.Hash()
	i3.Prev = i2.Hash()
	i3.TLoc.Set(104.0671670437, 30.5573090657)
	i3.CTime = now + int64(time.Hour*2)
	i3.CLoc.Set(104.0671670437, 30.5573090657)
	i3.STime = now + int64(time.Hour*2)
	i3.TPKH = UserID{3}
	i3.TASV = S721

	is := []*Unit{i1, i2, i3}
	err := calcer.Calc(is)
	log.Println(calcer, err)
}
