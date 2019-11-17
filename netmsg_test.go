package xginx

import (
	"testing"
)

func TestMsgAddrs(t *testing.T) {
	gm := &MsgGetAddrs{}
	if gm.Type() != NT_GET_ADDRS {
		t.Errorf("get typ error")
	}

	a1 := NetAddrForm("1.2.1.1:1")
	a2 := NetAddrForm("1.2.1.1:2")
	if a1.Equal(a2) {
		t.Errorf("equal error")
	}
	a3 := NetAddrForm("1.2.1.1:1")
	if !a1.Equal(a3) {
		t.Errorf("equal error")
	}

	m := &MsgAddrs{}
	m.Add(NetAddrForm("1.2.1.1:1"))
	m.Add(NetAddrForm("1.2.1.1:2"))
	m.Add(NetAddrForm("1.2.1.1:3"))
	m.Add(NetAddrForm("1.2.1.1:4"))
	buf := NewReadWriter()
	err := m.Encode(buf)
	if err != nil {
		panic(err)
	}
	m2 := &MsgAddrs{}
	err = m2.Decode(buf)
	if err != nil {
		panic(err)
	}
	if m.Type() != NT_ADDRS {
		t.Errorf("type 1 error")
	}
	if len(m.Addrs) != len(m2.Addrs) {
		t.Errorf("num error")
	}
	if m2.Type() != NT_ADDRS {
		t.Errorf("type 2 error")
	}
	for i := 0; i < len(m2.Addrs); i++ {
		if !m2.Addrs[i].Equal(m.Addrs[i]) {
			t.Errorf("data error")
		}
	}
}
