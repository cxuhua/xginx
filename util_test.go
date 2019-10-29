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
)

func TestGetDisRate(t *testing.T) {
	v := GetDisRate(750 + 500)
	log.Println(v == 0.5)
}

func TestGetRewardRate(t *testing.T) {
	v := GetRewardRate(0)
	if v != 1.0 {
		t.Errorf("error 1")
	}
	log.Println(v)
	v = GetRewardRate(210000)
	log.Println(v)
	v = GetRewardRate(210000 * 32)
	log.Println(v)
}

func TestCompressAmount(t *testing.T) {
	log.Println(CompressUInt(1000001))
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
