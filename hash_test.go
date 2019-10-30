package xginx

import (
	"testing"
)

func TestUIHashMake(t *testing.T) {
	xx := NewUIHash(0xff311223344)
	yy := NewHashID("ff311223344").ToUHash()
	if !xx.Equal(yy) {
		t.Errorf("test xx yy equal")
	}
	zz := NewUIHash("ff311223344")
	if !yy.Equal(zz) {
		t.Errorf("test yy zz equal")
	}
}

func TestU32HashMul(t *testing.T) {
	x1 := NewUIHash("7D1DE5EAF9B156D53208F033B5AA8122D2d2355d5e12292b121156cfdb4a529c")
	x2 := NewUIHash("7D1DE5EAF9B156D53208F033B5AA8122D2d2355d5e12292b121156cfdb4a529c")
	x := x1.Mul(x2)
	if x.String() != "62a38c0486f01e45879d7910a7761bf30d5237e9873f9bff3642a732c4d84f10" {
		t.Errorf("mul error")
	}
	//
	x1 = NewUIHash("7D1DE5EAF9B156D53208F033B5AA8122D2d2355d5e12292b121156cfdb4a529c")
	x2 = NewUIHash([]byte("\x70\x32\x1d\x7c\x47\xa5\x6b\x40\x26\x7e\x0a\xc3\xa6\x9c\xb6\xbf\x13\x30\x47\xa3\x19\x2d\xda\x71\x49\x13\x72\xf0\xb4\xca\x81\xd7"))
	x = x1.Mul(x2)
	if x.String() != "de37805e9986996cfba76ff6ba51c008df851987d9dd323f0e5de07760529c40" {
		t.Errorf("mul error")
	}
}

func TestSetCompact(t *testing.T) {

	v := NewUIHash(0)
	n, o := v.SetCompact(0x01123456)
	if v.String() != "0000000000000000000000000000000000000000000000000000000000000012" {
		t.Errorf("test set compact 1 failed")
	}
	if n != false || o != false {
		t.Errorf("test set compact 2 failed")
	}
	if v.Compact(n) != 0x01120000 {
		t.Errorf("test compact 1 failed")
	}

	v = NewUIHash(0x80)
	if v.Compact(false) != 0x02008000 {
		t.Errorf("test compact 2 failed")
	}

	n, o = v.SetCompact(0x01fedcba)
	if v.String() != "000000000000000000000000000000000000000000000000000000000000007e" {
		t.Errorf("test set compact 3 failed")
	}
	if n != true || o != false {
		t.Errorf("test set compact 4 failed")
	}

	if v.Compact(n) != 0x01fe0000 {
		t.Errorf("test compact 3 failed")
	}

	n, o = v.SetCompact(0xff123456)
	if n != false || o != true {
		t.Errorf("test set compact 5 failed")
	}

	n, o = v.SetCompact(0x20123456)
	if n != false || o != false {
		t.Errorf("test set compact 6 failed")
	}
	if v.String() != "1234560000000000000000000000000000000000000000000000000000000000" {
		t.Errorf("test set compact 7 failed")
	}

	if v.Compact(n) != 0x20123456 {
		t.Errorf("test compact 4 failed")
	}
}

func TestHashEqual(t *testing.T) {
	v1 := NewHashID("0101000000000000000000000000000000000000000000000000000000001234")
	v2 := v1.ToUHash().ToHashID()
	if !v1.Equal(v2) {
		t.Errorf("test Equal failed")
	}
}

func TestU32HashShift(t *testing.T) {
	s := "0000000000000000000000000000000000000000000000000000000000000001"
	one := NewUIHash(s)
	for i := uint(0); i < 254; i++ {
		one = one.Lshift(1)
	}
	for i := uint(0); i < 254; i++ {
		one = one.Rshift(1)
	}
	if one.String() != s {
		t.Errorf("test shift error")
	}
}

func TestU32HashBits(t *testing.T) {
	s := "0000000000000000000000000000000000000000000000000000000000000001"
	v1 := NewUIHash(s)
	if v1.String() != s {
		t.Errorf("string error")
	}
	if v1.Bits() != 1 {
		t.Errorf("bits error")
	}
	s = "8000000000000000000000000000000000000000000000000000000000000000"
	v1 = NewUIHash(s)
	if v1.String() != s {
		t.Errorf("string error")
	}
	if v1.Bits() != 256 {
		t.Errorf("bits error")
	}
}
