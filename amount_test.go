package xginx

import (
	"bytes"
	"testing"
)

func TestUseMaxMoney(t *testing.T) {
	if uint64(MAX_MONEY) > MAX_COMPRESS_UINT {
		t.Errorf("can't use amount compress")
	}
}

func TestAmountPut(t *testing.T) {
	a := Amount(100000)
	b := a.Bytes()
	c := Amount(0)
	c.From(b)
	if a != c {
		t.Error("test bytes from error")
	}
}

func TestAmountDecodeEncode(t *testing.T) {
	buf := &bytes.Buffer{}
	a := MAX_MONEY
	err := a.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	b := Amount(0)
	err = b.Decode(buf)
	if err != nil {
		t.Error(err)
	}
	if a != b {
		t.Errorf("MAX_MONEY equal test error")
	}
}
