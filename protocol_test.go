package xginx

import (
	"bytes"
	"io"
	"log"
	"testing"
)

type ATest struct {
}

func (a ATest) Type() uint8 {
	return 100
}

func (a ATest) Encode(w io.Writer) error {
	return nil
}

func (a *ATest) Decode(w io.Reader) error {
	return nil
}

type BC ATest

func (a BC) Type() uint8 {
	return 101
}

func TestMsgExt(t *testing.T) {
	b := &BC{}
	log.Println(b.Type())
}

func TestMsgVersion(t *testing.T) {
	msg := &MsgVersion{}
	msg.Ver = conf.Ver
	msg.Service = SERVICE_NODE
	msg.Addr = conf.GetNetAddr()
	buf := &bytes.Buffer{}
	err := msg.Encode(buf)
	if err != nil {
		panic(err)
	}
	nb := make([]byte, buf.Len())
	copy(nb, buf.Bytes())
	m2 := &MsgVersion{}
	err = m2.Decode(buf)
	if err != nil {
		panic(err)
	}
	if m2.Type() != NT_VERSION {
		t.Errorf("type error")
	}
	if m2.Ver != conf.Ver {
		t.Errorf("ver error")
	}
	if m2.Service != msg.Service {
		t.Errorf("service error")
	}
	np := NetPackage{
		Type:  NT_VERSION,
		Bytes: nb,
	}
	buf.Reset()
	err = np.Encode(buf)
	if err != nil {
		panic(err)
	}
	np2 := NetPackage{}
	err = np2.Decode(buf)
	if err != nil {
		panic(err)
	}
	m3, err := np2.ToMsgIO()
	m4, ok := m3.(*MsgVersion)
	if !ok {
		t.Errorf("type error")
	}
	if m4.Type() != NT_VERSION {
		t.Errorf("type error")
	}
	if m4.Ver != conf.Ver {
		t.Errorf("ver error")
	}
	if m4.Service != msg.Service {
		t.Errorf("service error")
	}
}

func TestVarBytes(t *testing.T) {
	buf := &bytes.Buffer{}
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
	buf := &bytes.Buffer{}

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
