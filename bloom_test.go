package xginx

import (
	"log"
	"testing"
)

func TestCalcBloomFilterSize(t *testing.T) {
	a, b := CalcBloomFilterSize(10, 0.000001)
	log.Println(a, b)
}

func TestBloomFilter(t *testing.T) {
	fb := make([]byte, 100)
	b, err := NewBloomFilter(10, 0, fb)
	if err != nil {
		panic(err)
	}
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

	c, err := NewBloomFilter(10, 0, fb)
	if err != nil {
		panic(err)
	}
	c.SetFilter(d)
	if !c.Has([]byte{10, 11, 32}) {
		t.Errorf("new load bloom miss")
	}
	if c.Has([]byte{10, 32}) {
		t.Errorf("new load bloom error")
	}
}
