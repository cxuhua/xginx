package xginx

import (
	"errors"
	"log"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	//1-1
	//3-2
	a32 = "XrxPtJaoXqJWg79S931ceyMsYcgMCUXousiVm2UB7PnVfWCD9RB5gpQ5mWGYGgemTUTXfcpGv4G4x8v2ErgW5TstcLPRW927Jxzj1tyqQmcCDdB5xFgKv4c6xXEHfnUzThURZh26L5tzScWSJ62NFkT4gPoosrkTFMdUvxZT1mxiX6WBa6PL4YVxnhFtinYn9FSqqeF7VKEbDWQC4jPJt9Df86TndKcB2sfPzX5LfAVcBPieDVKRXPm93SBp2hTJXZXyDRV1U8c2j6n2V5pyYhpRGDbH3iFpWJFuQGDCKto3dP8aM9hxTbXWHf95Zywg5ZZnTXwobhxwz7khcn1JWhdwni9cXqzhddzhGZVmYS7mFqJY5hGR814uDrsB18yCFCyBRMELRq6GMTaafoo7MaBgjzxKYkfeneXrCx4XrX2G3QjiThhkSrX1HQGeCWdp8iNSsjE1zjrZFivgamUJQ46JVWNxd8LChKjuLfuexcG4uDairUptYqUm7hUT"
	//3-3签名
	a33 = "XrxPtJaoXqJWg79S931ceyMsYcgMCUXousiVm2UB7PnVfWCD9RB5gpQ5mWGYGgemTUTXfcpGv4G4x8v2ErgW5TstcLPRW927Jxzj1tyqQmcCDdB5xFgKv4c6xXEHfnUzThURZh26L5tzScWSJ62NFkT4gPoosrkTFMdUvxZT1mxiX6WBa6PL4YVxnhFtinYn9FSqqeF7VKEbDWQC4jPJt9Df86TndKcB2sfPzX5LfAVcBPieDVKRXPm93SBp2hTJXZXyDRV1U8c2j6n2V5pyYhpRGDbH3iFpWJFuQGDCKto3dP8aM9hxTbXWHf95Zywg5ZZnTXwobhxwz7khcn1JWhdwni9cXqzhddzhGZVmYS7mFqJY5hGR814uDrsB18yCFCyBRMELRq6GMTaafoo7MaBgjzxKYkfeneXrCx4XrX2G3QjiThhkSrX1HQGeCWdp8iNSsjE1zjrZFivgamUJQ46JVWNxd8LChKjuLfuexcG4uDairUptYqUm7hUT"

	TestAccount, _ = LoadAccount(a32)
	DstAccount, _  = LoadAccount(a33)
)

func TestBlockHeader(t *testing.T) {
	h := BlockHeader{}
	h.Time = 1
	h.Nonce = 2

	b := h.Bytes()
	b.SetTime(time.Now())
	b.SetNonce(4)

	h2 := b.Header()

	if h2.Time != 3 {
		t.Errorf("time set error")
	}

	if h2.Nonce != 4 {
		t.Errorf("nonce set error")
	}
}

//测试用监听器
type tlis struct {
}

func (lis *tlis) OnClose(bi *BlockIndex) {

}
func (lis *tlis) OnInitHttp(m *gin.Engine) {

}

func (lis *tlis) OnClientMsg(c *Client, msg MsgIO) {

}

func (lis *tlis) OnUpdateBlock(bi *BlockIndex, blk *BlockInfo) {

}

func (lis *tlis) OnUpdateHeader(bi *BlockIndex, ele *TBEle) {

}

func (lis *tlis) OnStartup() {

}

//当块创建完毕
func (lis *tlis) OnNewBlock(bi *BlockIndex, blk *BlockInfo) error {

	//设置base out script
	//创建coinbase tx
	tx := &TX{}
	tx.Ver = 1

	//base tx
	in := &TxIn{}
	in.Script = blk.CoinbaseScript([]byte("Test Block"))
	tx.Ins = []*TxIn{in}

	out := &TxOut{}
	out.Value = blk.CoinbaseReward()
	if script, err := TestAccount.NewLockedScript(); err != nil {
		return err
	} else {
		out.Script = script
	}
	tx.Outs = []*TxOut{out}

	blk.Txs = []*TX{tx}

	return nil
}

func (lis *tlis) GetWallet() IWallet {
	return nil
}

//完成区块
func (lis *tlis) OnFinished(bi *BlockIndex, blk *BlockInfo) error {
	if len(blk.Txs) == 0 {
		return errors.New("txs miss")
	}
	btx := blk.Txs[0]
	if !btx.IsCoinBase() {
		return errors.New("coinbase tx miss")
	}
	fee, err := blk.GetFee(bi)
	if err != nil {
		return err
	}
	if fee == 0 {
		return nil
	}
	btx.Outs[0].Value += fee
	return blk.CheckTxs(bi)
}

func NewTestBlock(bi *BlockIndex) *BlockInfo {
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	if err := blk.CalcPowHash(1000000, bi); err != nil {
		panic(err)
	}
	return blk
}

func TestBlockChain(t *testing.T) {
	conf = LoadConfig("v10000.json")
	InitBlockIndex(&tlis{})
	bi := GetBlockIndex()
	defer bi.Close()
	testnum := uint32(10)
	for i := uint32(0); i < testnum; i++ {
		cb := NewTestBlock(bi)
		_, err := bi.LinkHeader(cb.Header)
		if err != nil {
			panic(err)
		}
		err = bi.UpdateBlk(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i > 0 && i%10000 == 0 {
			log.Println(i, "block")
		}
	}
}

func TestTransfire(t *testing.T) {
	conf = LoadConfig("v10000.json")
	InitBlockIndex(&tlis{})
	addr, err := DstAccount.GetAddress()
	if err != nil {
		panic(err)
	}
	src, err := TestAccount.GetAddress()
	if err != nil {
		panic(err)
	}
	bi := GetBlockIndex()
	blk, err := bi.NewBlock(1)
	if err != nil {
		panic(err)
	}
	tx, err := bi.Transfer(src, addr, 3*COIN, 1*COIN)
	if err != nil {
		panic(err)
	}
	//tx, err = bi.Transfer(TestAccount, addr, 3*COIN, 1*COIN)
	//if err != nil {
	//	panic(err)
	//}
	err = blk.AddTx(bi, tx)
	if err != nil {
		panic(err)
	}
	if err := blk.Finish(bi); err != nil {
		panic(err)
	}
	if err := blk.CalcPowHash(1000000, bi); err != nil {
		panic(err)
	}
	_, err = bi.LinkHeader(blk.Header)
	if err != nil {
		panic(err)
	}
	err = bi.UpdateBlk(blk)
	if err != nil {
		panic(err)
	}
	ds, err := bi.ListCoins(addr)
	if err != nil {
		panic(err)
	}
	log.Println(ds)
}

func TestUnlinkBlock(t *testing.T) {
	conf = LoadConfig("v10000.json")
	InitBlockIndex(&tlis{})
	bi := GetBlockIndex()
	err := bi.UnlinkLast()
	if err != nil {
		panic(err)
	}
}
