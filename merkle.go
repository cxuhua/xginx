package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/willf/bitset"
)

type MerkleNode []byte

type MerkleTree struct {
	trans int
	vhash []Hash256
	bits  []bool
	bad   bool
}

func NewMerkleTree(num int) *MerkleTree {
	v := &MerkleTree{}
	v.trans = num
	v.vhash = []Hash256{}
	v.bits = []bool{}
	v.bad = false
	return v
}

func GetMerkleTree(num int, hashs []Hash256, bits *bitset.BitSet) *MerkleTree {
	v := &MerkleTree{}
	v.trans = num
	v.vhash = hashs
	v.bits = []bool{}
	for i := uint(0); i < bits.Len(); i++ {
		v.bits = append(v.bits, bits.Test(i))
	}
	v.bad = false
	return v
}

func (tree *MerkleTree) Trans() int {
	return tree.trans
}

func (tree *MerkleTree) Hashs() []Hash256 {
	return tree.vhash
}

func (tree *MerkleTree) Bits() *bitset.BitSet {
	ret := bitset.New(uint(len(tree.bits)))
	for i, v := range tree.bits {
		ret.SetTo(uint(i), v)
	}
	return ret
}

func (tree *MerkleTree) Hash(n1 Hash256, n2 Hash256) Hash256 {
	ret := Hash256{}
	v := append([]byte{}, n1[:]...)
	v = append(v, n2[:]...)
	copy(ret[:], HASH256(v))
	return ret
}

func (tree *MerkleTree) Height() int {
	h := 0
	for tree.TreeWidth(h) > 1 {
		h++
	}
	return h
}

func BuildMerkleTree(ids []Hash256) *MerkleTree {
	num := len(ids)
	tree := NewMerkleTree(num)
	vb := bitset.New(uint(num))
	h := tree.Height()
	tree.build(h, 0, ids, vb)
	return tree
}

func (tree *MerkleTree) Build(ids []Hash256, vb *bitset.BitSet) *MerkleTree {
	tree.bad = false
	tree.vhash = []Hash256{}
	tree.bits = []bool{}
	h := tree.Height()
	tree.build(h, 0, ids, vb)
	return tree
}

func (tree *MerkleTree) build(h int, pos int, ids []Hash256, vb *bitset.BitSet) {
	match := false
	for p := pos << h; p < (pos+1)<<h && p < tree.trans; p++ {
		if vb.Test(uint(p)) {
			match = true
		}
	}
	tree.bits = append(tree.bits, match)
	if h == 0 || !match {
		tree.vhash = append(tree.vhash, tree.CalcHash(h, pos, ids))
	} else {
		tree.build(h-1, pos*2, ids, vb)
		if pos*2+1 < tree.TreeWidth(h-1) {
			tree.build(h-1, pos*2+1, ids, vb)
		}
	}
}

func (tree *MerkleTree) Extract() (Hash256, []Hash256, []int) {
	ids := make([]Hash256, 0)
	idx := make([]int, 0)
	tree.bad = false
	if tree.trans == 0 {
		return Hash256{}, nil, nil
	}
	if len(tree.vhash) > tree.trans {
		return Hash256{}, nil, nil
	}
	if len(tree.bits) < len(tree.vhash) {
		return Hash256{}, nil, nil
	}
	h := tree.Height()
	nbits, nhash := 0, 0
	root := tree.extract(h, 0, &nbits, &nhash, &ids, &idx)
	if tree.bad {
		return Hash256{}, nil, nil
	}
	if (nbits+7)/8 != (len(tree.bits)+7)/8 {
		return Hash256{}, nil, nil
	}
	if nhash != len(tree.vhash) {
		return Hash256{}, nil, nil
	}
	return root, ids, idx
}

func (tree *MerkleTree) extract(h int, pos int, nbits *int, nhash *int, ids *[]Hash256, idx *[]int) Hash256 {
	if *nbits >= len(tree.bits) {
		tree.bad = true
		return Hash256{}
	}
	match := tree.bits[*nbits]
	*nbits++
	if h == 0 || !match {
		if *nhash >= len(tree.vhash) {
			tree.bad = true
			return Hash256{}
		}
		hash := tree.vhash[*nhash]
		*nhash++
		if h == 0 && match {
			*ids = append(*ids, hash)
			*idx = append(*idx, pos)
		}
		return hash
	} else {
		left, right := tree.extract(h-1, pos*2, nbits, nhash, ids, idx), Hash256{}
		if pos*2+1 < tree.TreeWidth(h-1) {
			right = tree.extract(h-1, pos*2+1, nbits, nhash, ids, idx)
			if left.Equal(right) {
				tree.bad = true
			}
		} else {
			right = left
		}
		return tree.Hash(left, right)
	}
}

func (tree *MerkleTree) TreeWidth(h int) int {
	return (tree.trans + (1 << h) - 1) >> h
}

func (tree *MerkleTree) CalcHash(h int, pos int, ids []Hash256) Hash256 {
	if len(ids) == 0 {
		panic(errors.New("empty merkle array"))
	}
	if h == 0 {
		return ids[pos]
	}
	left, right := tree.CalcHash(h-1, pos*2, ids), Hash256{}
	if pos*2+1 < tree.TreeWidth(h-1) {
		right = tree.CalcHash(h-1, pos*2+1, ids)
	} else {
		right = left
	}
	return tree.Hash(left, right)
}

func init() {
	bitset.LittleEndian()
}

func NewBitSet(d []byte) *bitset.BitSet {
	bl := uint(len(d) * 8)
	bits := bitset.New(bl)
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, Endian, uint64(bl))
	nl := ((len(d) + 7) / 8) * 8
	nb := make([]byte, nl)
	copy(nb, d)
	_ = binary.Write(buf, Endian, nb)
	_, _ = bits.ReadFrom(buf)
	return bits
}

func FromBitSet(bs *bitset.BitSet) []byte {
	buf := &bytes.Buffer{}
	_, _ = bs.WriteTo(buf)
	bl := uint64(0)
	_ = binary.Read(buf, Endian, &bl)
	bl = (bl + 7) / 8
	bb := make([]byte, bl)
	_ = binary.Read(buf, Endian, bb)
	return bb
}
