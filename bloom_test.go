package xginx

import "testing"

func TestBloomFilter(t *testing.T) {
	b := NewBloomFilter()
	b.Add([]byte{1})
	b.Add([]byte{4})
	b.Add([]byte{5})
	b.Add([]byte{10, 11, 32})
	if b.Has([]byte{10, 11, 32}) {
		t.Errorf("build before")
	}
	b.Build()
	if !b.Has([]byte{10, 11, 32}) {
		t.Errorf("build after miss")
	}
	if b.Has([]byte{10, 32}) {
		t.Errorf("build after miss")
	}

	d := b.Dump()

	c := NewBloomFilter()
	c.Load(d)
	if !c.Has([]byte{10, 11, 32}) {
		t.Errorf("new load bloom miss")
	}
	if c.Has([]byte{10, 32}) {
		t.Errorf("new load bloom error")
	}
}
