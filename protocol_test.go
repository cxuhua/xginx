package xginx

import (
	"encoding/hex"
	"log"
	"testing"
)

func TestMsgVersion(t *testing.T) {
	conf = LoadConfig("test.json")
	defer conf.Close()
	msg := &MsgVersion{}
	msg.Ver = 1
	msg.Service = SERVICE_NODE
	msg.Addr = NetAddrForm("127.0.0.1:9000")

	buf := NewWriter()
	if err := msg.Encode(buf); err != nil {
		panic(err)
	}
	bb := buf.Bytes()
	log.Println(hex.EncodeToString(bb))
	pd := &NetPackage{
		Flags: conf.Flags,
		Type:  msg.Type(),
		Bytes: bb,
	}
	buf = NewWriter()
	err := pd.Encode(buf)
	if err != nil {
		panic(err)
	}

	r := NewReader(buf.Bytes())

	pd2 := &NetPackage{}
	err = pd2.Decode(r)
	if err != nil {
		panic(err)
	}
	log.Println(pd2.ToMsgIO())
}

func TestVarBytes(t *testing.T) {
	buf := NewReadWriter()
	b := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	v1 := VarBytes(b)
	err := v1.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	v2 := VarBytes{}
	err = v2.Decode(buf)
	if err != nil {
		t.Error(err)
	}
	if !v1.Equal(v2) {
		t.Errorf("test error")
	}
}

func TestNetPackage(t *testing.T) {
	buf := NewReadWriter()

	p1 := NetPackage{Bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8}}

	err := p1.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	p2 := NetPackage{}
	err = p2.Decode(buf)
	if err != nil {
		t.Error(err)
	}
	if !p1.Bytes.Equal(p2.Bytes) {
		t.Errorf("test error")
	}
}
