package xginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalSeq(t *testing.T) {
	v := VarUInt(FinalSequence)
	assert.Equal(t, len(v.Bytes()), 4)
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
	buf := NewReadWriter()
	a := MaxMoney
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

func TestParseIntMoney(t *testing.T) {
	num, err := ParseMoney("1.01")
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, num, Amount(1010))
	require.Equal(t, "1.01", num.String())

	num, err = ParseMoney("10.0")
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, num, Amount(10)*Coin)
	require.Equal(t, "10", num.String())

	num, err = ParseMoney("10.00001")
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, num, Amount(10)*Coin)
	require.Equal(t, "10", num.String())
}
