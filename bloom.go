package xginx

import (
	"errors"
	"math"
	"sync"
)

//布隆过滤器定义
const (
	MaxBloomFilterSize = 36000
	MaxHashFuncs       = 50
	Ln2Squared         = 0.4804530139182014246671025263266649717305529515945455
)

//BloomFilter 布隆过滤器
type BloomFilter struct {
	mu     sync.RWMutex
	filter []byte
	funcs  uint32
	tweak  uint32
}

func uint32min(v1 uint32, v2 uint32) uint32 {
	if v1 < v2 {
		return v1
	}
	return v2
}

//CalcBloomFilterSize 结算需要的存储量和hash次数
func CalcBloomFilterSize(elements int, fprate float64) (uint32, uint32) {
	if fprate > 1.0 {
		fprate = 1.0
	}
	if fprate < 1e-9 {
		fprate = 1e-9
	}

	dlen := uint32(-1 * float64(elements) * math.Log(fprate) / Ln2Squared)
	dlen = uint32min(dlen, MaxBloomFilterSize*8) / 8

	funcs := uint32(float64(dlen*8) / float64(elements) * math.Ln2)
	funcs = uint32min(funcs, MaxHashFuncs)

	return dlen, funcs
}

//NewBloomFilter 创建指定参数的布隆过滤器
func NewBloomFilter(funcs uint32, tweak uint32, filter []byte) (*BloomFilter, error) {
	if len(filter) > MaxBloomFilterSize {
		return nil, errors.New("filter size too big")
	}
	if funcs > MaxHashFuncs {
		return nil, errors.New("funcs too big")
	}
	if funcs == 0 {
		return nil, errors.New("funcs too little")
	}
	b := &BloomFilter{
		filter: filter,
		funcs:  funcs,
		tweak:  tweak,
	}
	return b, nil
}

//NewMsgFilterLoad 获取一个过滤器加载消息
func (b *BloomFilter) NewMsgFilterLoad() *MsgFilterLoad {
	m := &MsgFilterLoad{}
	m.Filter = b.filter
	m.Funcs = b.funcs
	m.Tweak = b.tweak
	return m
}

//Hash 计算hash
func (b *BloomFilter) Hash(n int, key []byte) uint32 {
	mm := MurmurHash(uint32(n)*0xFBA4C795+b.tweak, key)
	return mm % (uint32(len(b.filter)) << 3)
}

//Add  添加一个数据到过滤器
func (b *BloomFilter) Add(key []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for n := 0; n < int(b.funcs); n++ {
		idx := b.Hash(n, key)
		b.filter[idx>>3] |= (1 << (7 & idx))
	}
}

//SetFilter 设置过滤数据
func (b *BloomFilter) SetFilter(filter []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.filter = filter
}

//GetFilter 获取过滤器数据
func (b *BloomFilter) GetFilter() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter
}

//Has 检测是否存在指定的key数据
func (b *BloomFilter) Has(key []byte) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for n := 0; n < int(b.funcs); n++ {
		idx := b.Hash(n, key)
		if b.filter[idx>>3]&(1<<(idx&7)) == 0 {
			return false
		}
	}
	return true
}
