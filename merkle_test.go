package xginx

import (
	"bytes"
	"log"
	"testing"

	"github.com/willf/bitset"
)

var (
	amap = map[HashID]int{}
)

func TestByteMap(t *testing.T) {
	id1 := HashID{0}
	log.Println(id1)
	amap[id1] = 1

	id2 := HashID{1}
	amap[id2] = 2

	log.Println(amap[HashID{0}], amap[HashID{1}])
}

func TestNewBitSet(t *testing.T) {
	d := []byte{1, 2, 3, 4, 5}
	bs := NewBitSet(d)
	v := FromBitSet(bs)
	if !bytes.Equal(d, v) {
		t.Errorf("test newbitset failed")
	}
}

func TestMerkleArray(t *testing.T) {
	a := []HashID{}
	for i := 0; i < 21; i++ {
		tmp := HashID{byte(i)}
		a = append(a, tmp)
	}
	bs := bitset.New(uint(len(a)))
	bs.Set(20)

	tree := NewMerkleTree(len(a))
	tree.Build(a, bs)

	nt := GetMerkleTree(tree.Trans(), tree.Hashs(), tree.Bits())
	_, _, c1 := nt.Extract()
	if len(c1) != 1 || c1[0] != 20 {
		t.Errorf("test extrace error")
	}
}
