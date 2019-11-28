package xginx

import (
	"errors"
	"math"
	"sync"
)

const (
	MAX_BLOOM_FILTER_SIZE = 36000
	MAX_HASH_FUNCS        = 50
	LN2SQUARED            = 0.4804530139182014246671025263266649717305529515945455
)

type BloomFilter struct {
	mu     sync.RWMutex
	filter []byte
	funcs  uint32
	tweak  uint32
}

func uint32min(v1 uint32, v2 uint32) uint32 {
	if v1 < v2 {
		return v1
	} else {
		return v2
	}
}

//结算需要的存储量和hash次数
func CalcBloomFilterSize(elements int, fprate float64) (uint32, uint32) {
	if fprate > 1.0 {
		fprate = 1.0
	}
	if fprate < 1e-9 {
		fprate = 1e-9
	}

	dlen := uint32(-1 * float64(elements) * math.Log(fprate) / LN2SQUARED)
	dlen = uint32min(dlen, MAX_BLOOM_FILTER_SIZE*8) / 8

	funcs := uint32(float64(dlen*8) / float64(elements) * math.Ln2)
	funcs = uint32min(funcs, MAX_HASH_FUNCS)

	return dlen, funcs
}

func NewBloomFilter(funcs uint32, tweak uint32, filter []byte) (*BloomFilter, error) {
	if len(filter) > MAX_BLOOM_FILTER_SIZE/8 {
		return nil, errors.New("filter size too big")
	}
	if funcs > MAX_HASH_FUNCS {
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

//获取一个过滤器加载消息
func (b *BloomFilter) NewMsgFilterLoad() *MsgFilterLoad {
	m := &MsgFilterLoad{}
	m.Filter = b.filter
	m.Funcs = b.funcs
	m.Tweak = b.tweak
	return m
}

func (b *BloomFilter) Hash(n int, key []byte) uint32 {
	mm := MurmurHash(uint32(n)*0xFBA4C795+b.tweak, key)
	return mm % (uint32(len(b.filter)) << 3)
}

func (b *BloomFilter) Add(key []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for n := 0; n < int(b.funcs); n++ {
		idx := b.Hash(n, key)
		b.filter[idx>>3] |= (1 << (7 & idx))
	}
}

func (b *BloomFilter) SetFilter(filter []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.filter = filter
}

func (b *BloomFilter) GetFilter() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter
}

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
