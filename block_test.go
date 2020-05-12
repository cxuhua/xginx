package xginx

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type BlockTestSuite struct {
	suite.Suite
	bi *BlockIndex
}

func (suite *BlockTestSuite) SetupTest() {
	NewTestConfig()
	suite.bi = NewTestBlockIndex(100)

}

//测试用转账listener
type transListner struct {
	ITransListener
	bi  *BlockIndex
	src *Account
	dst *Account
}

//获取金额对应的账户方法
func (lis *transListner) GetAcc(ckv *CoinKeyValue) (*Account, error) {
	return lis.src, nil
}

//获取输出地址的扩展不同的地址可以返回不同的扩展信息
func (lis *transListner) GetExt(addr Address) []byte {
	return nil
}

//签名交易
func (lis *transListner) SignTx(singer ISigner, pass ...string) error {
	//获取签名信息
	_, in, _, _ := singer.GetObjs()
	//获取签名hash
	hash, err := singer.GetSigHash()
	if err != nil {
		return err
	}
	//获取签名
	sigs, err := lis.src.SignAll(hash)
	if err != nil {
		return err
	}
	//从输入获取签名脚本
	wits, err := in.Script.ToWitness()
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

//获取使用的金额列表
func (lis *transListner) GetCoins() Coins {
	addr, err := lis.src.GetAddress()
	if err != nil {
		panic(err)
	}
	cs, err := lis.bi.ListCoins(addr)
	if err != nil {
		panic(err)
	}
	return cs.Coins
}

//获取找零地址
func (lis *transListner) GetKeep() Address {
	addr, err := lis.src.GetAddress()
	if err != nil {
		panic(err)
	}
	return addr
}

func (suite *BlockTestSuite) TestTxLockTime() {
	req := suite.Require()
	req.NotNil(suite.bi)

	lis := GetTestListener(suite.bi)
	//获取矿工账户
	src := lis.GetAccount(0)
	saddr, err := src.GetAddress()
	req.NoError(err)
	coins, err := suite.bi.ListCoins(saddr)
	req.NoError(err)
	//矿工创建100个区块，奖励总数100*50，但只有50 *Coin可用
	//因为矿工奖励的必须满100高度才可用
	req.Equal(50*Coin, coins.Coins.Balance())
	//转账目标
	dst := lis.GetAccount(1)
	daddr, err := dst.GetAddress()
	req.NoError(err)

	tlis := &transListner{bi: suite.bi, src: src, dst: dst}
	//生成交易
	mi := suite.bi.NewTrans(tlis)
	//向dst转账1COIN
	mi.Add(daddr, 1*Coin)
	//1000作为交易费
	mi.Fee = 1000
	tx, err := mi.NewTx(300)
	req.NoError(err)
	bp := suite.bi.GetTxPool()
	req.NotNil(bp)
	err = bp.PushTx(suite.bi, tx)
	req.NoError(err)
	txs := bp.AllTxs()
	blk, err := suite.bi.NewBlock(1)
	req.NoError(err)
	err = blk.AddTxs(suite.bi, txs)
	req.NoError(err)
}

func (suite *BlockTestSuite) TearDownTest() {
	CloseTestBlock(suite.bi)

}

func TestBlockTestSuite(t *testing.T) {
	suite.Run(t, new(BlockTestSuite))
}
