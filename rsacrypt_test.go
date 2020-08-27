package xginx

import (
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRSADumpLoad(t *testing.T) {
	r, err := NewRSAPrivateKey()
	require.NoError(t, err)
	str, err := r.Dump("123")
	require.NoError(t, err)
	r1, err := LoadRSAPrivateKey(str, "123")
	require.NoError(t, err)
	p1 := r1.PublicKey()
	str1, err := p1.Dump()
	require.NoError(t, err)
	p2, err := LoadRSAPublicKey(str1)
	require.NoError(t, err)
	str2, err := p2.Dump()
	log.Println(str2, len(str2))
	require.NoError(t, err)
	assert.Equal(t, str1, str2)
}

func TestRSAEncryptDecrypt(t *testing.T) {
	r, err := NewRSAPrivateKey()
	require.NoError(t, err)
	p := r.PublicKey()
	//大数据测试
	str1 := strings.Repeat("838278234344", 1024)
	bb1, err := p.Encrypt([]byte(str1))
	require.NoError(t, err)
	bb2, err := r.Decrypt(bb1)
	require.NoError(t, err)
	assert.Equal(t, str1, string(bb2))
}

func TestRSASignVerity(t *testing.T) {
	r, err := NewRSAPrivateKey()
	require.NoError(t, err)
	p := r.PublicKey()
	str1 := "ksjfk---sdfk(&8382--78234344"
	bb1, err := r.Sign([]byte(str1))
	require.NoError(t, err)
	assert.Equal(t, 256, len(bb1))
	err = p.Verify([]byte(str1), bb1)
	require.NoError(t, err)
}
