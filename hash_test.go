package xginx

import (
	"log"
	"testing"
)

func TestMurmurHash(t *testing.T) {
	type DS struct {
		H uint32
		S uint32
		D string
	}
	ds := []DS{}
	ds = append(ds, DS{0x00000000, 0x00000000, ""})
	ds = append(ds, DS{0x00000000, 0x00000000, ""})
	ds = append(ds, DS{0x6a396f08, 0xFBA4C795, ""})
	ds = append(ds, DS{0x81f16f39, 0xffffffff, ""})
	ds = append(ds, DS{0x514e28b7, 0x00000000, "00"})
	ds = append(ds, DS{0xea3f0b17, 0xFBA4C795, "00"})
	ds = append(ds, DS{0xfd6cf10d, 0x00000000, "ff"})
	ds = append(ds, DS{0x16c6b7ab, 0x00000000, "0011"})
	ds = append(ds, DS{0x8eb51c3d, 0x00000000, "001122"})
	ds = append(ds, DS{0xb4471bf8, 0x00000000, "00112233"})
	ds = append(ds, DS{0xe2301fa8, 0x00000000, "0011223344"})
	ds = append(ds, DS{0xfc2e4a15, 0x00000000, "001122334455"})
	ds = append(ds, DS{0xb074502c, 0x00000000, "00112233445566"})
	ds = append(ds, DS{0x8034d2a0, 0x00000000, "0011223344556677"})
	ds = append(ds, DS{0xb4698def, 0x00000000, "001122334455667788"})

	for i, d := range ds {
		h := MurmurHash(d.S, HexToBytes(d.D))
		if h != d.H {
			t.Errorf("test %d error h=%x d=%x %s", i, h, d.H, d.D)
		}
	}
}

func TestPksToUINT256(t *testing.T) {
	pri, err := NewPrivateKey()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	pks := PKBytes{}
	pks.Set(pri.PublicKey())
	u := NewUINT256(pks[:])
	log.Println(u)
}

func TestUINT256Make(t *testing.T) {
	xx := NewUINT256(0xff311223344)
	yy := NewHASH256("ff311223344").ToU256()
	if !xx.Equal(yy) {
		t.Errorf("test xx yy equal")
	}
	zz := NewUINT256("ff311223344")
	if !yy.Equal(zz) {
		t.Errorf("test yy zz equal")
	}
}

func TestU32HashMul(t *testing.T) {
	x1 := NewUINT256("7D1DE5EAF9B156D53208F033B5AA8122D2d2355d5e12292b121156cfdb4a529c")
	x2 := NewUINT256("7D1DE5EAF9B156D53208F033B5AA8122D2d2355d5e12292b121156cfdb4a529c")
	x := x1.Mul(x2)
	if x.String() != "62a38c0486f01e45879d7910a7761bf30d5237e9873f9bff3642a732c4d84f10" {
		t.Errorf("mul error")
	}
	//
	x1 = NewUINT256("7D1DE5EAF9B156D53208F033B5AA8122D2d2355d5e12292b121156cfdb4a529c")
	x2 = NewUINT256([]byte("\x70\x32\x1d\x7c\x47\xa5\x6b\x40\x26\x7e\x0a\xc3\xa6\x9c\xb6\xbf\x13\x30\x47\xa3\x19\x2d\xda\x71\x49\x13\x72\xf0\xb4\xca\x81\xd7"))
	x = x1.Mul(x2)
	if x.String() != "de37805e9986996cfba76ff6ba51c008df851987d9dd323f0e5de07760529c40" {
		t.Errorf("mul error")
	}
}

func TestSetCompact(t *testing.T) {

	v := NewUINT256(0)
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

	v = NewUINT256(0x80)
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
	v1 := NewHASH256("0101000000000000000000000000000000000000000000000000000000001234")
	v2 := v1.ToU256().ToHASH256()
	if !v1.Equal(v2) {
		t.Errorf("test Equal failed")
	}
}

func TestU32HashShift(t *testing.T) {
	s := "0000000000000000000000000000000000000000000000000000000000000001"
	one := NewUINT256(s)
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
	v1 := NewUINT256(s)
	if v1.String() != s {
		t.Errorf("string error")
	}
	if v1.Bits() != 1 {
		t.Errorf("bits error")
	}
	s = "8000000000000000000000000000000000000000000000000000000000000000"
	v1 = NewUINT256(s)
	if v1.String() != s {
		t.Errorf("string error")
	}
	if v1.Bits() != 256 {
		t.Errorf("bits error")
	}
}
