package xginx

import (
	"bytes"
	"testing"
)

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
