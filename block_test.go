package xginx

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

func (suite *BlockTestSuite) TestRePushTx() {
	assert := assert.New(suite.T())
	lis := suite.bi.lptr.(*TestLis)
	mi := suite.bi.NewMulTrans()
	var first *TX
	dst, _ := lis.ams[1].GetAddress()
	//创建10个交易
	for i := 0; i < 10; i++ {
		mi.Acts = []*Account{lis.ams[0]}
		mi.Dst = []Address{dst}
		mi.Amts = []Amount{1 * COIN}
		mi.Fee = 0
		//创建交易
		tx, err := mi.NewTx(true)
		assert.Equal(err, nil, "new tx error")
		err = tx.Check(suite.bi, true)
		if i == 0 {
			first = tx
		}
	}
	assert.Equal(suite.bi.txp.Len(), 10, "tx pool count error")
	//创建区块打包
	blk, err := suite.bi.NewBlock(1)
	assert.Equal(err, nil, "new block error")
	//只打包第一个交易
	err = blk.AddTx(suite.bi, first)
	assert.Equal(err, nil, "add tx error")
	err = blk.Finish(suite.bi)
	assert.Equal(err, nil, "finish block error")
	calcbits(suite.bi, blk)
	err = suite.bi.LinkBlk(blk)
	assert.Equal(err, nil, "link block error")
	//剩下的9个交易应该是恢复进去的
	assert.Equal(suite.bi.txp.Len(), 9, "tx pool count error")
	ds, err := suite.bi.ListCoins(dst)
	assert.Equal(err, nil, "list conis error")
	assert.Equal(ds.All.Balance(), 10*COIN, "dst coin error")
	assert.Equal(ds.Indexs.Balance(), 1*COIN, "dst coin error")
	//打包剩下的交易
	//创建区块打包
	blk, err = suite.bi.NewBlock(1)
	assert.Equal(err, nil, "new block error")
	//只打包第一个交易
	err = blk.LoadTxs(suite.bi)
	assert.Equal(err, nil, "load txs error")
	err = blk.Finish(suite.bi)
	assert.Equal(err, nil, "finish block error")
	calcbits(suite.bi, blk)
	err = suite.bi.LinkBlk(blk)
	assert.Equal(err, nil, "link block error")
	//剩下交易应该全部被打包了
	assert.Equal(suite.bi.txp.Len(), 0, "tx pool count error")
	//目标应该有10个
	ds, err = suite.bi.ListCoins(dst)
	assert.Equal(err, nil, "list coins error")
	//总的101个区块减去转出的,多了一个区块，多奖励50，所以应该是101
	assert.Equal(ds.All.Balance(), 10*COIN, "dst coin error")
}

func (suite *BlockTestSuite) TearDownTest() {
	CloseTestBlock(suite.bi)
}

func TestBlockTestSuite(t *testing.T) {
	suite.Run(t, new(BlockTestSuite))
}
