package xginx

import "errors"

type BitSet struct {
	b []byte
}

func BitSetFrom(b []byte) *BitSet {
	return &BitSet{b: b}
}

func NewBitSet(l int) *BitSet {
	return &BitSet{
		b: make([]byte, (l+7)/8),
	}
}

func (bs *BitSet) Bytes() []byte {
	return bs.b
}

func (bs *BitSet) Len() int {
	return len(bs.b) * 8
}

func (bs *BitSet) Set(i int) {
	if i < 0 || i >= bs.Len() {
		panic(errors.New("i out bound"))
	}
	bs.b[i>>3] |= (1 << (7 & i))
}

func (bs *BitSet) SetTo(i int, v bool) {
	if i < 0 || i >= bs.Len() {
		panic(errors.New("i out bound"))
	}
	if v {
		bs.b[i>>3] |= (1 << (7 & i))
	} else {
		bs.b[i>>3] &= ^(1 << (7 & i))
	}
}

func (bs *BitSet) Test(i int) bool {
	max := len(bs.b) * 8
	if i < 0 || i >= max {
		panic(errors.New("i out bound"))
	}
	return bs.b[i>>3]&(1<<(i&7)) != 0
}
