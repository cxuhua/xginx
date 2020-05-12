package xginx

import (
	"bytes"
	"testing"
)

func TestAes(t *testing.T) {
	tkey := []byte{1, 2, 3}
	ttxt := "9988776"
	b := NewAESCipher(tkey)
	r1, err := AesEncrypt(b, []byte(ttxt))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := AesDecrypt(b, r1)
	if string(r2) != ttxt {
		t.Fatal("encrypt decrypt error")
	}
	b1 := NewAESCipher([]byte{4, 5, 6})
	r3, err := AesDecrypt(b1, r1)
	if bytes.Equal(r3, []byte(ttxt)) {
		t.Fatal("decrypt wrong key error", string(r3))
	}
}
