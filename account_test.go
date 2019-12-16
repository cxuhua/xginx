package xginx

import (
	"bytes"
	"log"
	"testing"
)

func TestLoadDumpAccount(t *testing.T) {
	a, err := NewAccount(3, 2, false)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	s, err := a.Dump(true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	log.Println(s)
	b := Account{}
	err = b.Load(s)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if a.Num != b.Num && a.Less != b.Less {
		t.Error("num error")
		t.FailNow()
	}
	if a.Arb != b.Arb {
		t.Error("num error")
		t.FailNow()
	}
	if len(a.Pris) != len(b.Pris) {
		t.Error("pris num error")
		t.FailNow()
	}
	if len(a.Pubs) != len(b.Pubs) {
		t.Error("pubs num error")
		t.FailNow()
	}
	for i, v := range a.Pubs {
		if !v.Equal(b.Pubs[i].GetPks().Bytes()) {
			t.Error("pubs error")
			t.FailNow()
		}
	}
	for i, v := range a.Pris {
		if !bytes.Equal(v.Encode(), b.Pris[i].Encode()) {
			t.Error("pris error")
			t.FailNow()
		}
	}
}
