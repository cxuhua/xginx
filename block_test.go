package xginx

import (
	"log"
	"testing"
	"time"
)

func TestCalcDistance(t *testing.T) {

	now := time.Now().UnixNano()
	//i1 first
	i1 := UnitBlock{}
	i1.TLoc.Set(104.0658044815, 30.5517656113)
	i1.CTime = now
	i1.CLoc.Set(104.0671670437, 30.5573090657)
	i1.STime = now

	i2 := UnitBlock{}
	i2.Prev = i1.Hash()
	i2.TLoc.Set(104.0615880489, 30.5536596605)
	i2.CTime = now + int64(time.Hour)
	i2.CLoc.Set(104.0615880489, 30.5536596605)
	i2.STime = now + int64(time.Hour)

	i3 := UnitBlock{}
	i3.Prev = i2.Hash()
	i3.TLoc.Set(104.0671670437, 30.5573090657)
	i3.CTime = now + int64(time.Hour*2)
	i3.CLoc.Set(104.0671670437, 30.5573090657)
	i3.STime = now + int64(time.Hour*2)

	is := []UnitBlock{i1, i2, i3}
	dis, err := CalcDistance(is)

	log.Println(dis, err)
}
