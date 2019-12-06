package xginx

import (
	"errors"
)

type MerkleTree struct {
	trans int
	vhash []HASH256
	bits  []bool
	bad   bool
}

func NewMerkleTree(num int) *MerkleTree {
	v := &MerkleTree{}
	v.trans = num
	v.vhash = []HASH256{}
	v.bits = []bool{}
	v.bad = false
	return v
}

func GetMerkleTree(num int, hashs []HASH256, vb *BitSet) *MerkleTree {
	v := &MerkleTree{}
	v.trans = num
	v.vhash = hashs
	v.bits = []bool{}
	for i := 0; i < vb.Len(); i++ {
		v.bits = append(v.bits, vb.Test(i))
	}
	v.bad = false
	return v
}

func (tree *MerkleTree) Trans() int {
	return tree.trans
}

func (tree *MerkleTree) Hashs() []HASH256 {
	return tree.vhash
}

func (tree *MerkleTree) Bits() *BitSet {
	ret := NewBitSet(len(tree.bits))
	for i, v := range tree.bits {
		ret.SetTo(i, v)
	}
	return ret
}

func (tree *MerkleTree) Hash(n1 HASH256, n2 HASH256) HASH256 {
	v := append([]byte{}, n1[:]...)
	v = append(v, n2[:]...)
	return Hash256From(v)
}

func (tree *MerkleTree) Height() int {
	h := 0
	for tree.TreeWidth(h) > 1 {
		h++
	}
	return h
}

func BuildMerkleTree(ids []HASH256) *MerkleTree {
	num := len(ids)
	tree := NewMerkleTree(num)
	vb := NewBitSet(num)
	h := tree.Height()
	tree.build(h, 0, ids, vb)
	return tree
}

func (tree MerkleTree) IsBad() bool {
	return tree.bad
}

func (tree *MerkleTree) Build(ids []HASH256, vb *BitSet) *MerkleTree {
	tree.bad = false
	tree.vhash = []HASH256{}
	tree.bits = []bool{}
	h := tree.Height()
	tree.build(h, 0, ids, vb)
	return tree
}

func (tree *MerkleTree) build(h int, pos int, ids []HASH256, vb *BitSet) {
	match := false
	for p := pos << h; p < (pos+1)<<h && p < tree.trans; p++ {
		if vb.Test(p) {
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

func (tree *MerkleTree) ExtractRoot() (HASH256, error) {
	root := ZERO256
	ids := make([]HASH256, 0)
	idx := make([]int, 0)
	tree.bad = false
	if tree.trans == 0 {
		return root, errors.New("trans miss")
	}
	if len(tree.vhash) > tree.trans {
		return root, errors.New("hash count error")
	}
	if len(tree.bits) < len(tree.vhash) {
		return root, errors.New("bits len error")
	}
	height := tree.Height()
	nbits, nhash := 0, 0
	root = tree.extract(height, 0, &nbits, &nhash, &ids, &idx)
	if tree.bad {
		return root, errors.New("extract bad")
	}
	if (nbits+7)/8 != (len(tree.bits)+7)/8 {
		return root, errors.New("nbits len error")
	}
	if nhash != len(tree.vhash) {
		return root, errors.New("hash count != tree hash count")
	}
	return root, nil
}

func (tree *MerkleTree) Extract() (HASH256, []HASH256, []int) {
	hash := ZERO256
	ids := make([]HASH256, 0)
	idx := make([]int, 0)
	tree.bad = false
	if tree.trans == 0 {
		return hash, nil, nil
	}
	if len(tree.vhash) > tree.trans {
		return hash, nil, nil
	}
	if len(tree.bits) < len(tree.vhash) {
		return hash, nil, nil
	}
	height := tree.Height()
	nbits, nhash := 0, 0
	hash = tree.extract(height, 0, &nbits, &nhash, &ids, &idx)
	if tree.bad {
		return hash, nil, nil
	}
	if (nbits+7)/8 != (len(tree.bits)+7)/8 {
		return hash, nil, nil
	}
	if nhash != len(tree.vhash) {
		return hash, nil, nil
	}
	return hash, ids, idx
}

func (tree *MerkleTree) extract(height int, pos int, nbits *int, nhash *int, ids *[]HASH256, idx *[]int) HASH256 {
	if *nbits >= len(tree.bits) {
		tree.bad = true
		return HASH256{}
	}
	match := tree.bits[*nbits]
	*nbits++
	if height == 0 || !match {
		if *nhash >= len(tree.vhash) {
			tree.bad = true
			return HASH256{}
		}
		hash := tree.vhash[*nhash]
		*nhash++
		if height == 0 && match {
			*ids = append(*ids, hash)
			*idx = append(*idx, pos)
		}
		return hash
	} else {
		left, right := tree.extract(height-1, pos*2, nbits, nhash, ids, idx), HASH256{}
		if pos*2+1 < tree.TreeWidth(height-1) {
			right = tree.extract(height-1, pos*2+1, nbits, nhash, ids, idx)
			if left.Equal(right) {
				tree.bad = true
			}
		} else {
			right = left
		}
		return tree.Hash(left, right)
	}
}

func (tree *MerkleTree) TreeWidth(height int) int {
	return (tree.trans + (1 << height) - 1) >> height
}

func (tree *MerkleTree) CalcHash(height int, pos int, ids []HASH256) HASH256 {
	if len(ids) == 0 {
		panic(errors.New("empty merkle array"))
	}
	if height == 0 {
		return ids[pos]
	}
	left, right := tree.CalcHash(height-1, pos*2, ids), HASH256{}
	if pos*2+1 < tree.TreeWidth(height-1) {
		right = tree.CalcHash(height-1, pos*2+1, ids)
	} else {
		right = left
	}
	return tree.Hash(left, right)
}
