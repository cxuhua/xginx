package main

import (
	"context"
	"testing"

	"github.com/cxuhua/xginx"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestNewShopMeta(t *testing.T) {
	ctx := context.Background()
	mtxt := MetaEle{}
	mtxt.Type = MetaEleTEXT
	mtxt.Body = "test ele 11"
	mtxt.Size = len(mtxt.Body)
	mtxt.Sum = MetaHash(mtxt.Body)
	err := mtxt.Check(ctx)
	require.NoError(t, err)

	mhash := MetaEle{}
	mhash.Type = MetaEleHASH
	mhash.Body = xginx.ZERO256.String()
	mhash.Size = len(mhash.Body)
	err = mhash.Check(ctx)
	require.NoError(t, err)

	rsa, err := xginx.NewRSAPrivateKey()
	require.NoError(t, err)
	mrsa := MetaEle{}
	mrsa.Type = MetaEleRSA
	body, err := rsa.PublicKey().Dump()
	require.NoError(t, err)
	mrsa.Body = body
	mrsa.Size = len(mrsa.Body)
	err = mrsa.Check(ctx)
	require.NoError(t, err)

	urls := MetaEle{
		Type: MetaEleURL,
		Size: 6617,
		Sum:  "f16ed9adfaf03651720e091e5259c8aeb4f509c6",
		Body: "https://www.baidu.com/img/flexible/logo/pc/result.png",
	}
	err = urls.Check(ctx)
	require.NoError(t, err)

	mb := &MetaBody{
		Type: MetaTypeSell,
		Tags: []string{"11", "22"},
		Eles: []MetaEle{mtxt, mhash, mrsa, urls},
	}
	sm, err := NewShopMeta(ctx, mb)
	require.NoError(t, err)
	mb2, err := sm.To()
	require.NoError(t, err)
	assert.Equal(t, mb.Sum, mb2.Sum)
	assert.Equal(t, mb.Type, mb2.Type)
	assert.Equal(t, mb.Tags, mb2.Tags)
	assert.Equal(t, mb.Eles, mb2.Eles)
}
