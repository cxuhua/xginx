package xginx

import (
	"log"
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
	txs, err := suite.bi.ListTxs(conf.MinerAddr)
	suite.Require().NoError(err)
	for _, v := range txs {
		log.Println(v.TxId, v.Height)
	}
}

func (suite *BlockTestSuite) TestRePushTx() {

}

func (suite *BlockTestSuite) TearDownTest() {
	CloseTestBlock(suite.bi)
}

func TestBlockTestSuite(t *testing.T) {
	suite.Run(t, new(BlockTestSuite))
}
