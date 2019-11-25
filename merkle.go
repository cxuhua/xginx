package xginx

import (
	"errors"

	"github.com/willf/bitset"
)

type MerkleNode []byte

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

func GetMerkleTree(num int, hashs []HASH256, bits *bitset.BitSet) *MerkleTree {
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

func (tree *MerkleTree) Hashs() []HASH256 {
	return tree.vhash
}

func (tree *MerkleTree) Bits() *bitset.BitSet {
	ret := bitset.New(uint(len(tree.bits)))
	for i, v := range tree.bits {
		ret.SetTo(uint(i), v)
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
	vb := bitset.New(uint(num))
	h := tree.Height()
	tree.build(h, 0, ids, vb)
	return tree
}

func (tree MerkleTree) IsBad() bool {
	return tree.bad
}

func (tree *MerkleTree) Build(ids []HASH256, vb *bitset.BitSet) *MerkleTree {
	tree.bad = false
	tree.vhash = []HASH256{}
	tree.bits = []bool{}
	h := tree.Height()
	tree.build(h, 0, ids, vb)
	return tree
}

func (tree *MerkleTree) build(h int, pos int, ids []HASH256, vb *bitset.BitSet) {
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

func (tree *MerkleTree) ExtractRoot() (HASH256, error) {
	root := HASH256{}
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
	h := tree.Height()
	nbits, nhash := 0, 0
	root = tree.extract(h, 0, &nbits, &nhash, &ids, &idx)
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
	hash := HASH256{}
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
	h := tree.Height()
	nbits, nhash := 0, 0
	hash = tree.extract(h, 0, &nbits, &nhash, &ids, &idx)
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

func (tree *MerkleTree) extract(h int, pos int, nbits *int, nhash *int, ids *[]HASH256, idx *[]int) HASH256 {
	if *nbits >= len(tree.bits) {
		tree.bad = true
		return HASH256{}
	}
	match := tree.bits[*nbits]
	*nbits++
	if h == 0 || !match {
		if *nhash >= len(tree.vhash) {
			tree.bad = true
			return HASH256{}
		}
		hash := tree.vhash[*nhash]
		*nhash++
		if h == 0 && match {
			*ids = append(*ids, hash)
			*idx = append(*idx, pos)
		}
		return hash
	} else {
		left, right := tree.extract(h-1, pos*2, nbits, nhash, ids, idx), HASH256{}
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

func (tree *MerkleTree) CalcHash(h int, pos int, ids []HASH256) HASH256 {
	if len(ids) == 0 {
		panic(errors.New("empty merkle array"))
	}
	if h == 0 {
		return ids[pos]
	}
	left, right := tree.CalcHash(h-1, pos*2, ids), HASH256{}
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
	w := NewReadWriter()
	_ = w.TWrite(uint64(bl))
	nl := ((len(d) + 7) / 8) * 8
	nb := make([]byte, nl)
	copy(nb, d)
	_ = w.TWrite(nb)
	_, _ = bits.ReadFrom(w)
	return bits
}

func FromBitSet(bs *bitset.BitSet) []byte {
	buf := NewReadWriter()
	_, _ = bs.WriteTo(buf)
	bl := uint64(0)
	_ = buf.TRead(&bl)
	bl = (bl + 7) / 8
	bb := make([]byte, bl)
	_ = buf.TRead(bb)
	return bb
}
