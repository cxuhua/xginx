package xginx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//测试用账号签名器
type accsigner struct {
	*Account
}

//签名交易as
func (signer *accsigner) SignTx(singer ISigner, pass ...string) error {
	//获取签名信息
	_, in, _, _ := singer.GetObjs()
	//从输入获取签名脚本
	wits, err := in.Script.ToWitness()
	if err != nil {
		return err
	}
	//获取签名hash
	hash, err := singer.GetSigHash()
	if err != nil {
		return err
	}
	//获取签名
	sigs, err := signer.SignAll(hash)
	if err != nil {
		return err
	}
	wits.Sig = sigs
	script, err := wits.ToScript()
	if err != nil {
		return err
	}
	in.Script = script
	return nil
}

func newaccsigner(acc *Account) ISignTx {
	return &accsigner{acc}
}

//测试交易中引用同交易的输出,是允许
func TestNewTx(t *testing.T) {
	NewTestConfig()
	bi := NewTestBlockIndex(100)
	defer CloseTestBlock(bi)

	lis := GetTestListener(bi)
	a := lis.ams[0]
	b := lis.ams[1]

	//获取链中一个可用金额
	coins, err := bi.ListCoinsWithAccount(a)
	require.NoError(t, err)
	require.Equal(t, 1, len(coins.Coins))
	coin := coins.Coins[0]
	tx := NewTx(0, DefaultTxScript)
	//a -> b 消耗一个可用的输出
	in1 := &TxIn{}
	in1.OutHash = coin.TxID
	in1.OutIndex = coin.Index
	sa, err := a.NewWitnessScript(DefaultInputScript).ToScript()
	require.NoError(t, err)
	in1.Script = sa
	in1.Sequence = FinalSequence

	out1 := &TxOut{}
	out1.Value = 2 * Coin
	sb, err := b.NewLockedScript("", DefaultLockedScript)
	require.NoError(t, err)
	out1.Script = sb

	tx.Ins = append(tx.Ins, in1)
	tx.Outs = append(tx.Outs, out1)

	err = tx.Sign(bi, newaccsigner(a))
	require.NoError(t, err)

	err = tx.Check(bi, true)
	require.NoError(t, err)
}
