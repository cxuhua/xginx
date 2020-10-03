package xginx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"log"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUUID(t *testing.T) {
	id1 := NewDocumentID(1)
	id2 := id1.To(2)
	assert.Equal(t, id1.Type(), byte(1))
	assert.Equal(t, id2.Type(), byte(2))
	assert.Equal(t, id1[1:], id2[1:])
}

func TestHashDumpLoad(t *testing.T) {
	s1 := "sdfsdf978s9df7s0df7sdf"
	b, err := HashDump([]byte(s1))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := HashLoad(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(s2) != s1 {
		t.Error("hash error")
	}
	//带加密
	b, err = HashDump([]byte(s1), "xhKfd9fd789")
	if err != nil {
		t.Fatal(err)
	}
	s2, err = HashLoad(b, "xhKfd9fd789")
	if err != nil {
		t.Fatal(err)
	}
	if string(s2) != s1 {
		t.Error("hash error")
	}
}

func TestOne(t *testing.T) {
	var f ONCE

	fn := func(i int) {
		if !f.Running() {
			log.Println("fn isrunning", i)
			return
		}
		time.Sleep(time.Second)
		defer f.Reset()
	}

	for i := 0; i < 10; i++ {
		go func(x int) {
			fn(x)
		}(i)
	}
	time.Sleep(time.Second * 60)
}

func TestCompressAmount(t *testing.T) {
	for i := 0; i < 1000; i++ {
		v := uint64(0)
		binary.Read(rand.Reader, binary.LittleEndian, &v)
		//must < 60 bits
		v = v & (uint64(1<<60) - 1)
		v1 := CompressUInt(v)
		v2 := DecompressUInt(v1)
		if v2 != v {
			t.Errorf("error %x != %x  %x", v, v2, v1)
		}
	}
}

// y^2 = x^3 -3x + b
// y = sqrt(x^3 -3x + b)
func TestP256PublicCompress(t *testing.T) {
	c := elliptic.P256().Params()
	pri, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Errorf("make privake error %v", err)
	}
	log.Println("key = ", hex.EncodeToString(pri.D.Bytes()))
	log.Println("x=", hex.EncodeToString(pri.X.Bytes()))
	log.Println("y=", hex.EncodeToString(pri.Y.Bytes()))

	d := pri.Y.Bit(0)
	x := pri.X
	var y, x3b, x3 big.Int
	x3.SetInt64(3)
	x3.Mul(&x3, x)
	x3b.Mul(x, x)
	x3b.Mul(&x3b, x)
	x3b.Add(&x3b, c.B)
	x3b.Sub(&x3b, &x3)
	x3b.Mod(&x3b, c.P)
	y.ModSqrt(&x3b, c.P)
	if y.Bit(0) != d {
		y.Sub(c.P, &y)
	}
	if y.Cmp(pri.Y) != 0 {
		t.Errorf("failed")
	}
	log.Println("cy=", hex.EncodeToString(y.Bytes()), "ybit=", d)
}
