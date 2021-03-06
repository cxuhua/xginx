package xginx

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicHash(t *testing.T) {
	bb := []byte{1, 2, 3, 4}
	hv1 := Hash256From(bb)
	//pk1q3hj89c3ejcgt42nlsjzq237dgz2rfcclt5aaw8jdj3ljswr5l8qq9j7v4g
	id, err := EncodePublicHash(hv1)
	assert.NoError(t, err, id)
	hv2, err := DecodePublicHash(id)
	assert.NoError(t, err)
	assert.Equal(t, hv1, hv2)
}

func BenchmarkVerify(b *testing.B) {
	for i := 0; i < b.N; i++ {
		msg := make([]byte, rand.Uint32()%500)
		rand.Read(msg)
		hash := Hash256([]byte(msg))
		pk1, err := NewPrivateKey()
		if err != nil {
			b.Errorf("DecodePrivateKey error %v", err)
		}
		sig1, err := pk1.Sign(hash)
		if err != nil {
			b.Errorf("sign 1 error %v", err)
		}
		pub1 := pk1.PublicKey()
		if !pub1.Verify(hash, sig1) {
			b.Errorf("Verify 1 error")
		}
	}
}

func TestSignVerify(t *testing.T) {
	msg := "Very deterministic message"
	hash := Hash256([]byte(msg))
	pk1, err := NewPrivateKey()
	if err != nil {
		t.Errorf("DecodePrivateKey error %v", err)
	}
	ds, err := pk1.Dump()
	if err != nil {
		t.Error(err)
	}
	pk2, err := LoadPrivateKey(ds)
	if err != nil {
		t.Error(err)
	}
	sig1, err := pk2.Sign(hash)
	if err != nil {
		t.Errorf("sign 1 error %v", err)
	}
	sig2, err := NewSigValue(sig1.Encode())
	if err != nil {
		t.Error(err)
	}
	pub1 := pk1.PublicKey()
	if !pub1.Verify(hash, sig2) {
		t.Errorf("Verify 1 error")
	}
	pub2, err := NewPublicKey(pub1.Encode())
	if err != nil {
		t.Errorf("encode pub2 error %v", err)
	}
	if !pub2.Verify(hash, sig1) {
		t.Errorf("Verify 2 error")
	}
}
