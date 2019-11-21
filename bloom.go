package xginx

import (
	"sync"

	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type BloomFilter struct {
	bloom  filter.Filter
	gener  filter.FilterGenerator
	filter []byte
	buf    *util.Buffer
	mu     sync.RWMutex
}

func NewBloomFilter() *BloomFilter {
	bloom := filter.NewBloomFilter(10)
	return &BloomFilter{
		bloom: bloom,
		buf:   &util.Buffer{},
		gener: bloom.NewGenerator(),
	}
}

func (h *BloomFilter) Add(key []byte, build ...bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gener.Add(key)
	if len(build) > 0 && build[0] {
		h.Build()
	}
}

func (h *BloomFilter) Load(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.filter = b
}

func (h *BloomFilter) Dump() []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.filter
}

func (h *BloomFilter) Build() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf.Reset()
	h.gener.Generate(h.buf)
	h.filter = h.buf.Bytes()
}

func (h *BloomFilter) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.filter = nil
}

func (h *BloomFilter) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.filter)
}

func (h *BloomFilter) Has(key []byte) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.bloom.Contains(h.filter, key)
}
