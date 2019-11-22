package xginx

import (
	"testing"
)

func TestBloomFilter(t *testing.T) {
	b := NewBloomFilterWithNumber(500, 10, 0.6, 0x11)
	b.Add([]byte{1})
	b.Add([]byte{4})
	b.Add([]byte{5})
	if b.Has([]byte{10, 11, 32}) {
		t.Errorf("build before")
	}
	b.Add([]byte{10, 11, 32})
	if !b.Has([]byte{10, 11, 32}) {
		t.Errorf("build after miss")
	}
	if b.Has([]byte{10, 32}) {
		t.Errorf("build after miss")
	}

	d := b.GetFilter()

	c := NewBloomFilterWithNumber(500, 10, 0.6, 0x11)
	c.SetFilter(d)
	if !c.Has([]byte{10, 11, 32}) {
		t.Errorf("new load bloom miss")
	}
	if c.Has([]byte{10, 32}) {
		t.Errorf("new load bloom error")
	}
}
