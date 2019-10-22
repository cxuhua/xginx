package xginx

import (
	"bytes"
	"crypto/rand"
	"log"
	"testing"
)

func TestTagEncode(t *testing.T) {
	tag := TagRecord{}
	tag.TVer = 1
	tag.TLoc.Set(180.14343, -85.2343434)
	_, _ = rand.Read(tag.TPK[:])
	s, err := tag.EncodeTag()
	if err != nil {
		panic(err)
	}
	ntag := TagRecord{}
	err = ntag.Decode(s)
	if err != nil {
		panic(err)
	}
	if !ntag.TEqual(tag) {
		t.Errorf("test equal error")
	}
	log.Println(tag.EncodeTag())
	log.Println(ntag.EncodeTag())
}

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
		if err := v.Write(buf); err != nil {
			b.Error(err)
		}
		v2 := VarInt(0)
		if err := v2.Read(buf); err != nil {
			b.Error(err)
		}
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
