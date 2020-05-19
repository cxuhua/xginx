package xginx

import (
	"bytes"
	"log"
	"testing"
)

func TestLoadAccount(t *testing.T) {
	file := "st1qwgpep2ge4jj6tkpmxq5dy0wjmlk5qdd7srrz3n"
	acc, err := LoadAccountWithFile("f:/" + file)
	log.Println(acc, err)
}

func TestArbSign(t *testing.T) {
	a, err := NewAccount(3, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if a.Arb != 2 {
		t.Fatal("arb error")
	}
	signhash := []byte{1, 2, 3}
	sig, err := a.Sign(int(a.Arb), signhash)
	if err != nil {
		t.Fatal(err)
	}
	err = a.VerifyAll(signhash, []SigBytes{sig})
	if err != nil {
		t.Fatal(err)
	}
}

func TestErrorHashSign(t *testing.T) {
	a, err := NewAccount(3, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.Arb != InvalidArb {
		t.Fatal("arb error")
	}
	shash := []byte{1, 2, 3}
	sig0, err := a.Sign(0, shash)
	if err != nil {
		t.Fatal(err)
	}
	sig1, err := a.Sign(1, shash)
	if err != nil {
		t.Fatal(err)
	}
	vhash := []byte{1, 2, 4}
	err = a.VerifyAll(vhash, []SigBytes{sig0, sig1})
	if err == nil {
		t.Fatal("sig hash error")
	}
	err = a.VerifyAll(shash, []SigBytes{sig1, sig0})
	if err == nil {
		t.Fatal("sig index error")
	}
}

func TestNoArbSign(t *testing.T) {
	a, err := NewAccount(3, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.Arb != InvalidArb {
		t.Fatal("arb error")
	}
	signhash := []byte{1, 2, 3}
	sig0, err := a.Sign(0, signhash)
	if err != nil {
		t.Fatal(err)
	}
	sig1, err := a.Sign(1, signhash)
	if err != nil {
		t.Fatal(err)
	}
	err = a.VerifyAll(signhash, []SigBytes{sig0, sig1})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadDumpAccount(t *testing.T) {
	a, err := NewAccount(3, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.Arb != InvalidArb {
		t.Fatal("arb error")
	}
	s, err := a.Dump(true)
	if err != nil {
		t.Fatal(err)
	}
	b := Account{}
	err = b.Load(s)
	if err != nil {
		t.Fatal(err)
	}
	if a.Num != b.Num && a.Less != b.Less {
		t.Fatal("num error")
	}
	if a.Arb != b.Arb {
		t.Fatal("num error")
	}
	if len(a.Pris) != len(b.Pris) {
		t.Fatal("pris num error")
	}
	if len(a.Pubs) != len(b.Pubs) {
		t.Fatal("pubs num error")
	}
	for i, v := range a.Pubs {
		if !v.Equal(b.Pubs[i].GetPks().Bytes()) {
			t.Fatal("pubs error")
		}
	}
	for i, v := range a.Pris {
		if !bytes.Equal(v.Encode(), b.Pris[i].Encode()) {
			t.Fatal("pris error")
		}
	}
}
