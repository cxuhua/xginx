package xginx

import (
	"bytes"
	"log"
	"testing"

	"github.com/willf/bitset"
)

var (
	amap = map[HASH256]int{}
)

func TestByteMap(t *testing.T) {
	id1 := HASH256{0}
	log.Println(id1)
	amap[id1] = 1

	id2 := HASH256{1}
	amap[id2] = 2

	log.Println(amap[HASH256{0}], amap[HASH256{1}])
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
	a := []HASH256{}
	for i := 0; i < 7; i++ {
		tmp := HASH256{byte(i)}
		a = append(a, tmp)
	}
	bs := bitset.New(uint(len(a)))
	bs.Set(6)

	tree := NewMerkleTree(len(a))
	tree.Build(a, bs)

	nt := GetMerkleTree(tree.Trans(), tree.Hashs(), tree.Bits())
	root, hashs, c1 := nt.Extract()
	if len(c1) != 1 || c1[0] != 6 {
		t.Errorf("test extrace error")
	}
	log.Println(root, hashs)
}
