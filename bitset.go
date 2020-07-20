package xginx

import "errors"

//BitSet 位集合
type BitSet struct {
	b []byte
}

//BitSetFrom 从指定数据创建位集合
func BitSetFrom(b []byte) *BitSet {
	return &BitSet{b: b}
}

//NewBitSet 创建长度位l的位集合
func NewBitSet(l int) *BitSet {
	return &BitSet{
		b: make([]byte, (l+7)/8),
	}
}

//Bytes 获取位集合数据
func (bs *BitSet) Bytes() []byte {
	return bs.b
}

//Len 获取位集合长度
func (bs *BitSet) Len() int {
	return len(bs.b) * 8
}

//Set 设置位
func (bs *BitSet) Set(i int) {
	if i < 0 || i >= bs.Len() {
		panic(errors.New("i out bound"))
	}
	bs.b[i>>3] |= (1 << (7 & i))
}

//SetTo 设置位状态
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

//Test 测试位状态
func (bs *BitSet) Test(i int) bool {
	max := len(bs.b) * 8
	if i < 0 || i >= max {
		panic(errors.New("i out bound"))
	}
	return bs.b[i>>3]&(1<<(i&7)) != 0
}
