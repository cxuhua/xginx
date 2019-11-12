package xginx

import (
	"log"
	"testing"
	"time"
)

func TestWalletEnc(t *testing.T) {
	w, err := NewLevelDBWallet("wallet")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	ds := w.ListAddress()
	if len(ds) != 0 {
		t.Error("address error")
		t.FailNow()
	}
	addr, err := w.NewAddress()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	err = w.Encryption(addr, "1223232")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	_, err = w.GetPrivate(addr)
	if err == nil {
		t.Errorf("address encryption,can't get")
		t.FailNow()
	}
	err = w.Decryption(addr, "1223232", time.Second*3)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	time.Sleep(time.Second * 5)
	_, err = w.GetPrivate(addr)
	if err == nil {
		t.Errorf("address exp")
		t.FailNow()
	}
	w.Close()
}

func TestWallet(t *testing.T) {
	w, err := NewLevelDBWallet("wallet")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	ds := w.ListAddress()
	if len(ds) != 0 {
		t.Error("address error")
		t.FailNow()
	}
	addr, err := w.NewAddress()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	err = w.Encryption(addr, "1223232")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	log.Println("new addr = ", addr)
	ds = w.ListAddress()
	for _, v := range ds {
		pri, err := w.GetPrivate(v)
		if err == nil {
			t.Error(err)
			t.FailNow()
		}
		err = w.Decryption(v, "1223232", time.Second*20)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		pri, err = w.GetPrivate(v)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		if pri.PublicKey().Address() != v {
			t.Error("address data error")
			t.FailNow()
		}
		err = w.RemoveAddress(v)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
	ds = w.ListAddress()
	if len(ds) != 0 {
		t.Error("remove address error")
		t.FailNow()
	}
	w.Close()
}
