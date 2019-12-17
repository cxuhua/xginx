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

func (suite *BlockTestSuite) TestRePushTx() {

}

func (suite *BlockTestSuite) TearDownTest() {
	CloseTestBlock(suite.bi)
}

func TestBlockTestSuite(t *testing.T) {
	suite.Run(t, new(BlockTestSuite))
}
