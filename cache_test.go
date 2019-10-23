package xginx

import (
	"testing"
)

func TestCacher(t *testing.T) {
	c := newTxs()
	tx1 := &TTagInfo{UID: []byte{1}}
	c.Set(tx1)
	c.Push()
	tx2 := &TTagInfo{UID: []byte{2}}
	c.Set(tx2)
	if v, err := c.Get(TagUID{2}); err != nil {
		t.Errorf("get error 1")
	} else if v != tx2 {
		t.Errorf("get error 2")
	}
	c.Pop()
	if v, err := c.Get(TagUID{1}); err != nil {
		t.Errorf("get error 3")
	} else if v != tx1 {
		t.Errorf("get error 4")
	}
}
