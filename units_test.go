package xginx

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"sort"
	"testing"
)

func TestCliUnits_Push(t *testing.T) {

	pks := PKBytes{1, 3, 4}

	v := NewCliUnits(pks.Hash())

	uss := []*Unit{}
	var u0 *Unit
	for i := 0; i < 1000; i++ {
		u1 := &Unit{}
		u1.CPks = pks
		u1.STime = int64(i)
		uss = append(uss, u1)
		if u0 == nil {
			u0 = u1
		} else {
			u1.Prev = u0.Hash()
			u0 = u1
		}
		rv := uint32(0)
		binary.Read(rand.Reader, binary.BigEndian, &rv)
		if rv%10 == 0 {
			u1.Prev = HASH256{}
		}
	}

	for i := 0; i < 1000; i++ {
		sort.Slice(uss, func(i, j int) bool {
			rv := uint32(0)
			binary.Read(rand.Reader, binary.BigEndian, &rv)
			a := rv % 2
			return a == 0
		})
	}

	for _, uv := range uss {
		v.Push(uv)
	}

	max := v.MaxList()
	if max == nil {
		return
	}
	for cur := max.Front(); cur != nil; cur = cur.Next() {
		v := cur.Value.(*Unit)
		log.Println(v.STime)
	}
}
