package xginx

import (
	"log"
	"testing"
)

func TestCliUnits_Push(t *testing.T) {

	pks := PKBytes{1, 3, 4}

	v := NewCliUnits(pks.Hash())

	u1 := &Unit{}
	u1.CPks = pks
	u1.STime = 1

	u2 := &Unit{}
	u2.CPks = pks
	//u2.Prev = u1.Hash()
	u2.STime = 2

	u3 := &Unit{}
	u3.Prev = u2.Hash()
	u3.CPks = pks
	u3.STime = 3

	u4 := &Unit{}
	u4.CPks = pks
	u4.STime = 4

	u1.Prev = u4.Hash()

	u5 := &Unit{}
	u5.CPks = pks
	u5.STime = 5
	//u5.Prev = u1.Hash()

	u2.Prev = u5.Hash()

	v.Push(u5)
	v.Push(u1)
	v.Push(u3)
	v.Push(u2)
	v.Push(u4)

	max := v.MaxList()
	if max == nil {
		return
	}
	for cur := max.Front(); cur != nil; cur = cur.Next() {
		v := cur.Value.(*Unit)
		log.Println(v.STime)
	}
}
