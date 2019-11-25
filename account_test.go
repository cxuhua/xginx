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
	if a.num != b.num && a.less != b.less {
		t.Error("num error")
		t.FailNow()
	}
	if a.arb != b.arb {
		t.Error("num error")
		t.FailNow()
	}
	if len(a.pris) != len(b.pris) {
		t.Error("pris num error")
		t.FailNow()
	}
	if len(a.pubs) != len(b.pubs) {
		t.Error("pubs num error")
		t.FailNow()
	}
	for i, v := range a.pubs {
		if !v.Equal(b.pubs[i].GetPks().Bytes()) {
			t.Error("pubs error")
			t.FailNow()
		}
	}
	for i, v := range a.pris {
		if !bytes.Equal(v.Encode(), b.pris[i].Encode()) {
			t.Error("pris error")
			t.FailNow()
		}
	}
}
