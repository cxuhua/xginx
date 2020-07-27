package xginx

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestListDB(t *testing.T) {
	kdb, err := OpenKeysDB(NewTempDir())
	require.NoError(t, err)
	defer kdb.Close()
	for i := 0; i < 20; i++ {
		id, err := kdb.NewPrivateKey()
		require.NoError(t, err)
		ka := &AddressInfo{
			Num:  1,
			Less: 1,
			Arb:  InvalidArb, //不启用
			Pks:  []string{id},
			Desc: "这是个测试账户",
		}
		_, err = kdb.SaveAddressInfo(ka)
		require.NoError(t, err)
	}
	addrs, lkey := kdb.ListAddress(10)
	assert.Equal(t, 10, len(addrs))
	addrs, lkey = kdb.ListAddress(10, lkey)
	assert.Equal(t, 10, len(addrs))
	addrs, lkey = kdb.ListAddress(10, lkey)
	assert.Equal(t, 0, len(addrs))

	ids, lkey := kdb.ListPrivate(10)
	assert.Equal(t, 10, len(ids))
	ids, lkey = kdb.ListPrivate(10, lkey)
	assert.Equal(t, 10, len(ids))
	ids, lkey = kdb.ListPrivate(10, lkey)
	assert.Equal(t, 0, len(ids))

}

func TestPrivateNewLoad(t *testing.T) {
	kdb, err := OpenKeysDB(NewTempDir())
	require.NoError(t, err)
	defer kdb.Close()
	ka, err := kdb.NewAddressInfo("aaa")
	require.NoError(t, err)
	acc, err := ka.ToAccount(kdb)
	require.NoError(t, err)
	log.Println(acc)
	id, err := kdb.NewPrivateKey()
	require.NoError(t, err)
	pri, err := kdb.LoadPrivateKey(id)
	require.NoError(t, err)
	lid, err := pri.PublicKey().ID()
	require.NoError(t, err)
	assert.Equal(t, id, lid)
}

func TestPrivateNewLoadWithKey(t *testing.T) {
	kdb, err := OpenKeysDB(NewTempDir(), "11113344")
	require.NoError(t, err)
	defer kdb.Close()

	id, err := kdb.NewPrivateKey()
	require.NoError(t, err)
	pri, err := kdb.LoadPrivateKey(id)
	require.NoError(t, err)
	lid, err := pri.PublicKey().ID()
	require.NoError(t, err)
	assert.Equal(t, id, lid)
}

func TestLoadAddressInfo(t *testing.T) {
	kdb, err := OpenKeysDB(NewTempDir())
	require.NoError(t, err)
	defer kdb.Close()

	id1, err := kdb.NewPrivateKey()
	require.NoError(t, err)

	id2, err := kdb.NewPrivateKey()
	require.NoError(t, err)

	//检测控制权
	req := &CtrlPrivateKeyReq{
		ID:      id1,
		RandStr: "32432423423dfgdg",
	}
	res, err := kdb.HasKeyPrivileges(req)
	require.NoError(t, err)
	err = res.Check(req)
	require.NoError(t, err)

	ka := &AddressInfo{
		Num:  2,
		Less: 2,
		Arb:  InvalidArb, //不启用
		Pks:  []string{id1, id2},
		Desc: "这是个测试账户",
	}
	id3, err := ka.ID()
	require.NoError(t, err)

	id4, err := kdb.SaveAddressInfo(ka)
	require.NoError(t, err)
	assert.Equal(t, id3, id4)

	ka2, err := kdb.LoadAddressInfo(id3)
	require.NoError(t, err)
	id5, err := ka2.ID()
	require.NoError(t, err)
	assert.Equal(t, id5, id4)
	//签名验证检测
	hv := []byte{1, 2, 3, 4}
	wits, err := kdb.NewWitnessScript(id5)
	require.NoError(t, err)
	err = kdb.Sign(id5, hv, wits)
	require.NoError(t, err)
	acc, err := wits.ToAccount()
	require.NoError(t, err)
	err = acc.VerifyAll(hv, wits.Sig)
	require.NoError(t, err)
}
