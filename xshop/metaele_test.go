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
	err := mtxt.Check(ctx)
	require.NoError(t, err)

	mhash := MetaEle{}
	mhash.Type = MetaEleHASH
	mhash.Body = xginx.ZERO256.String()
	err = mhash.Check(ctx)
	require.NoError(t, err)

	rsa, err := xginx.NewRSAPrivateKey()
	require.NoError(t, err)
	mrsa := MetaEle{}
	mrsa.Type = MetaEleRSA
	body, err := rsa.PublicKey().Dump()
	require.NoError(t, err)
	mrsa.Body = body
	err = mrsa.Check(ctx)
	require.NoError(t, err)

	urls, err := NewMetaUrl(ctx, "https://www.baidu.com/img/flexible/logo/pc/result.png")
	require.NoError(t, err)
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
	assert.Equal(t, mb.Type, mb2.Type)
	assert.Equal(t, mb.Tags, mb2.Tags)
	assert.Equal(t, mb.Eles, mb2.Eles)
}
