package xginx

import (
	"log"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
)

func TestCache(t *testing.T) {
	c := cache.New(time.Second*3, time.Second*3)
	c.Set("aa", 111, time.Second)
	time.Sleep(time.Second * 2)
	log.Println(c.Get("aa"))
}

func TestWalletEnc(t *testing.T) {
	w, err := NewLevelDBWallet("/Users/xuhua/wtest")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	defer w.Close()
	ds := w.ListAccount()
	if len(ds) != 0 {
		t.Error("address error")
		t.FailNow()
	}
	addr, err := w.NewAccount(3, 3, false)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	err = w.Encryption(addr, "1223232")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	_, err = w.GetAccount(addr)
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
	_, err = w.GetAccount(addr)
	if err == nil {
		t.Errorf("address exp")
		t.FailNow()
	}
}

func TestWallet(t *testing.T) {
	w, err := NewLevelDBWallet("/Users/xuhua/wtest")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	defer w.Close()
	ds := w.ListAccount()
	if len(ds) != 0 {
		t.Error("address error")
		t.FailNow()
	}
	addr, err := w.NewAccount(3, 2, true)
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
	ds = w.ListAccount()
	for _, v := range ds {
		acc, err := w.GetAccount(v)
		if err == nil {
			t.Error(err)
			t.FailNow()
		}
		err = w.Decryption(v, "1223232", time.Second*20)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		acc, err = w.GetAccount(v)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if addr, err := acc.GetAddress(); err != nil {
			t.Error(err)
			t.FailNow()
		} else if addr != v {
			t.Error("address data error")
			t.FailNow()
		}
		err = w.RemoveAccount(v)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
	ds = w.ListAccount()
	if len(ds) != 0 {
		t.Error("remove address error")
		t.FailNow()
	}
	w.Close()
}
