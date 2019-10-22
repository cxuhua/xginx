package xginx

import (
	"bytes"
	"log"
	"testing"
)

func TestMaxBits(t *testing.T) {
	for i := uint(0); i < 64; i++ {
		xx := uint64(1 << i)
		if MaxBits(xx) != i {
			t.Errorf("error %x", xx)
		}
	}
}

func BenchmarkVarInt(b *testing.B) {
	buf := &bytes.Buffer{}
	for i := -b.N; i < b.N; i++ {
		v := VarInt(i)
		buf.Reset()
		v.Write(buf)
		v2 := VarInt(0)
		v2.Read(buf)
		if v != v2 {
			b.Errorf("error %d %d", v, v2)
			break
		}
	}
}

func TestUInt24(t *testing.T) {
	x := &UInt24{}
	for i := uint32(0); i > 1024; i++ {
		x.SetUInt32(i)
		if x.ToUInt32() != i {
			t.Errorf("test error")
		}
	}
}

func TestLocation(t *testing.T) {
	l1 := Location{}
	l1.Set(180.14343, -85.2343434)
	log.Println(l1)
	log.Println(l1.Get())
}
