package xginx

import (
	"log"
	"testing"
	"time"
)

func TestBlockSave(t *testing.T) {
	best := GetBestBlock()
	b := &BlockInfo{}
	SetRandInt(&b.Header.Ver)
	SetRandInt(&b.Header.Nonce)
	SetRandInt(&b.Header.Time)
	_, err := b.LinkBack()
	if err != nil {
		panic(err)
	}
	b.Check()
	err = b.Load(best)
	if err != nil {
		panic(err)
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
