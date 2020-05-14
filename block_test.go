package xginx

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type BlockTestSuite struct {
	suite.Suite
	bi *BlockIndex
}

func (suite *BlockTestSuite) SetupSuite() {
	NewTestConfig()
	suite.bi = NewTestBlockIndex(100)
}

func (suite *BlockTestSuite) SetupTest() {

}

//测试用转账listener
type transListner struct {
	ITransListener
	bi  *BlockIndex
	src *Account
	dst *Account
}

func newTransListner(bi *BlockIndex, src *Account, dst *Account) *transListner {
	return &transListner{bi: bi, src: src, dst: dst}
}

//获取金额对应的账户方法
func (lis *transListner) GetAcc(ckv *CoinKeyValue) (*Account, error) {
	return lis.src, nil
}

func (lis *transListner) GetTxOutExec(addr Address) []byte {
	return DefaultLockedScript
}

//获取输入执行脚本 ckv消费的金额对象
func (lis *transListner) GetTxInExec(ckv *CoinKeyValue) []byte {
	return DefaultInputScript
}

//当输入创建好
func (lis *transListner) OnNewTxIn(tx *TX, in *TxIn) error {
	return nil
}

//当输出创建好
func (lis *transListner) OnNewTxOut(tx *TX, out *TxOut) error {
	return nil
}

//当交易创建完毕
func (lis *transListner) OnNewTx(tx *TX) error {
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
	//100区块会产生100个金额记录，其中99个未成熟不可用，1个可用
	req.Equal(100, len(coins.All))
	req.Equal(99, len(coins.Locks))
	req.Equal(1, len(coins.Coins))
	//矿工创建100个区块，奖励总数100*50，但只有50 *Coin可用
	//因为矿工奖励的必须满100高度才可用
	req.Equal(50*Coin, coins.Coins.Balance())
	//转账目标
	dst := lis.GetAccount(1)
	daddr, err := dst.GetAddress()
	req.NoError(err)

	tlis := newTransListner(suite.bi, src, dst)
	//生成交易
	mi := suite.bi.NewTrans(tlis)
	//向dst转账1COIN
	mi.Add(daddr, 1*Coin)
	//1000作为交易费
	mi.Fee = 1 * Coin
	tx, err := mi.NewTx(0, DefaultTxScript)
	req.NoError(err)
	bp := suite.bi.GetTxPool()
	req.NotNil(bp)
	err = bp.PushTx(suite.bi, tx)
	req.NoError(err)
	txs := bp.AllTxs()
	//应该有一个放入了交易池
	req.Equal(1, len(txs))

	// cp := tx.Clone(1)
	// err = cp.Sign(suite.bi, tlis)
	// req.NoError(err)
	// err = bp.PushTx(suite.bi, cp)
	// req.NoError(err)

	//创建一个新区块
	blk, err := suite.bi.NewBlock(1)
	req.NoError(err)
	err = blk.AddTxs(suite.bi, txs)
	req.NoError(err)
	//应该有两个交易
	req.Equal(2, len(blk.Txs))
	//完成区块设置准备链接到链
	err = blk.Finish(suite.bi)
	req.NoError(err)
	//模拟计算工作量证明，测试环境下很容易
	calcbits(suite.bi, blk)
	//链接到测试链
	err = suite.bi.LinkBlk(blk)
	req.NoError(err)
	coins, err = suite.bi.ListCoins(saddr)
	req.NoError(err)
	//101区块会产生101个金额记录，其中99个未成熟不可用，2个可用,2个是因为刚才又链接了一个新的区块
	req.Equal(101, len(coins.All))
	req.Equal(99, len(coins.Locks))
	req.Equal(2, len(coins.Coins))
	//可用的金额记录应该是2*50-2,因为被转走了1个，交易费1个
	req.Equal((2*50-2)*Coin, coins.Coins.Balance())
	coins, err = suite.bi.ListCoins(daddr)
	req.NoError(err)
	req.Equal(1*Coin, coins.Coins.Balance())
}

func (suite *BlockTestSuite) TestSequence() {
}

func (suite *BlockTestSuite) TearDownTest() {

}

func (suite *BlockTestSuite) TearDownSuite() {
	CloseTestBlock(suite.bi)
}

func TestBlockTestSuite(t *testing.T) {
	suite.Run(t, new(BlockTestSuite))
}
