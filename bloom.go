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
	LN2                   = 0.6931471805599453094172321214581765680755001343602552
)

type BloomFilter struct {
	mu     sync.RWMutex
	filter []byte
	funcs  int
	tweak  uint32
}

func CalcBloomFilterSize(elements int, fprate float64) (int, int) {
	if fprate <= 0 {
		fprate = 0.1
	}
	max := MAX_BLOOM_FILTER_SIZE * 8
	dsiz := int(-1 / LN2SQUARED * float64(elements) * math.Log(fprate))
	if dsiz > max {
		dsiz = max
	}
	fsize := int(float64(dsiz) / float64(elements) * LN2)
	if fsize > MAX_HASH_FUNCS {
		fsize = MAX_HASH_FUNCS
	}
	return dsiz / 8, fsize
}

func NewBloomFilter(funcs int, tweak uint32, filter []byte) (*BloomFilter, error) {
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
	m.Funcs = uint8(b.funcs)
	m.Tweak = b.tweak
	return m
}

func (b *BloomFilter) Hash(n int, key []byte) uint32 {
	h := MurmurHash(uint32(n)*0xFBA4C795+b.tweak, key)
	return h % (uint32(len(b.filter)) * 8)
}

func (b *BloomFilter) Add(key []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for n := 0; n < b.funcs; n++ {
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

func (b *BloomFilter) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

}

func (b *BloomFilter) Has(key []byte) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for n := 0; n < b.funcs; n++ {
		idx := b.Hash(n, key)
		if b.filter[idx>>3]&(1<<(7&idx)) == 0 {
			return false
		}
	}
	return true
}
