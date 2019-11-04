package xginx

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"testing"
	"time"
)

func TestReadUnit(t *testing.T) {
	var unit1 *Unit
	var unit2 *Unit
	var unit3 *Unit
	err := store.UseSession(context.Background(), func(db DBImp) error {
		u1 := &TUnit{}
		id1, err := base64.StdEncoding.DecodeString("2Yu0LH3xiVKlcYK6PjQr9KaLFd8mExd5/PC6WwDCicE=")
		if err != nil {
			return err
		}
		err = db.GetUnit(id1, u1)
		if err != nil {
			return err
		}
		unit1 = u1.ToUnit()

		id2, err := base64.StdEncoding.DecodeString("iPHrbxZAKMdmdEdtrvme4m0Lt+e+IBdwB/b4EmCm1/U=")
		if err != nil {
			return err
		}
		err = db.GetUnit(id2, u1)
		if err != nil {
			return err
		}
		unit2 = u1.ToUnit()

		if !unit2.Prev.Equal(unit1.Hash()) {
			return errors.New("errors")
		}

		id3, err := base64.StdEncoding.DecodeString("8NVIH3ymO+TwEnOrnN4EckEeTKTOM7sv65NWL8Sv7y4=")
		if err != nil {
			return err
		}
		err = db.GetUnit(id3, u1)
		if err != nil {
			return err
		}
		unit3 = u1.ToUnit()

		if !unit3.Prev.Equal(unit2.Hash()) {
			return errors.New("errors")
		}
		return err
	})
	if err != nil {
		panic(err)
	}

	bits := NewUINT256(conf.PowLimit).Compact(false)
	calcer := NewTokenCalcer()

	is := &Units{unit1, unit2, unit3}
	err = calcer.Calc(bits, is)
	log.Println(calcer, err)
}

func TestCreateGenesisBlock(t *testing.T) {
	pub, err := LoadPublicKey("8aKby6XxwmoaiYt6gUbS1u2RHco37iHfh6sAPstME33Qh6ujd9")
	if err != nil {
		panic(err)
	}

	b := &BlockInfo{
		Ver:    1,
		Prev:   HASH256{},
		Merkle: HASH256{},
		Time:   1572669878,
		Bits:   0x1d00ffff,
		Nonce:  0x58f3e185,
		Uts:    []*Units{},
		Txs:    []*TX{},
	}

	tx := &TX{}

	tx.Ver = 1
	in := &TxIn{}
	in.Script = BaseScript(0, []byte("The value of a man should be seen in what he gives and not in what he is able to receive."))
	tx.Ins = []*TxIn{in}

	out := &TxOut{}
	out.Value = 529
	out.Script = StdLockedScript(pub)

	tx.Outs = []*TxOut{out}
	tx.Hash()
	b.Txs = []*TX{tx}
	//生成merkle root id
	if err := b.SetMerkle(); err != nil {
		panic(err)
	}
	tx2 := &TTx{}
	err = store.UseSession(context.Background(), func(db DBImp) error {

		return db.GetTX(tx.Hash().Bytes(), tx2)
	})
	tx3 := tx2.ToTx()
	log.Println(err, b.Hash(), tx3.Hash().Equal(tx.Hash()))
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
