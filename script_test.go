package xginx

import (
	"bytes"
	"net"
	"testing"
)

func TestCoinbase(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	vb := []byte{1, 2, 3, 4}
	s := NewCoinbaseScript(10, ip, vb)
	if s.Height() != 10 {
		t.Fatal("set height error")
	}
	sip := s.IP()
	if !bytes.Equal(ip, sip) {
		t.Fatal("ip save error")
	}
	svb := s.Data()
	if !bytes.Equal(vb, svb) {
		t.Fatal("coinbase data save error")
	}
}

func TestScriptSer(t *testing.T) {
	s := Script([]byte{1, 2, 3, 4, 5})
	buf := NewReadWriter()
	err := s.Encode(buf)
	if err != nil {
		t.Errorf("encode error")
	}
	b := Script{}
	err = b.Decode(buf)
	if err != nil {
		t.Errorf("Decode error")
	}
	if !bytes.Equal(s, b) {
		t.Errorf("test error")
	}
}
