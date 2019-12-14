package xginx

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

//测试用监听器
type TestLis struct {
	Listener
	addrs []Address
	acc   map[HASH160]*Account
	ams   []*Account
	t     uint32
}

func newTestLis() *TestLis {
	lis := &TestLis{
		t:   uint32(time.Now().Unix()),
		acc: map[HASH160]*Account{},
	}
	for i := 0; i < 5; i++ {
		//创建5个账号
		acc, err := NewAccount(1, 1, false)
		if err != nil {
			panic(err)
		}
		addr, err := acc.GetAddress()
		if err != nil {
			panic(err)
		}
		pkh, err := acc.GetPkh()
		if err != nil {
			panic(err)
		}
		lis.acc[pkh] = acc
		lis.addrs = append(lis.addrs, addr)
		lis.ams = append(lis.ams, acc)
	}
	if len(lis.addrs) != 5 {
		panic(errors.New("create test account error"))
	}
	LogInfo("create 5 test account")
	return lis
}

func (lis *TestLis) TimeNow() uint32 {
	atomic.AddUint32(&lis.t, 1)
	return atomic.LoadUint32(&lis.t)
}

func (lis *TestLis) OnInit(bi *BlockIndex) error {
	//测试每次清楚数据
	return bi.RemoveBestValue()
}

//当账户没有私钥时调用此方法签名
//singer 签名器
func (lis *TestLis) OnSignTx(signer ISigner) error {
	_, in, out := signer.GetObjs()
	pkh, err := out.Script.GetPkh()
	if err != nil {
		return err
	}
	acc := lis.acc[pkh]
	if acc == nil {
		return errors.New("get signer acc error")
	}
	sigh, err := signer.GetSigHash()
	if err != nil {
		return err
	}
	wits := acc.NewWitnessScript()
	for i := 0; i < int(acc.num); i++ {
		sig, err := acc.Sign(i, sigh)
		if err != nil {
			return err
		}
		wits.Sig = append(wits.Sig, sig)
	}
	script, err := wits.ToScript()
	if err != nil {
		return err
	}
	in.Script = script
	return nil
}

//计算难度hash，测试环境下难度很低
func calcbits(bi *BlockIndex, blk *BlockInfo) {
	hb := blk.Header.Bytes()
	for {
		hb.SetNonce(RandUInt32())
		id := hb.Hash()
		if CheckProofOfWork(id, blk.Header.Bits) {
			blk.Header = hb.Header()
			break
		}
	}
	if bi.Len() == 0 {
		conf.genesis = blk.Header.MustID()
	}
}

//创建一个测试用区块索引
func NewTestBlockIndex(num int) *BlockIndex {
	//测试配置文件
	conf = &Config{}
	conf.nodeid = conf.GenUInt64()
	conf.DataDir = os.TempDir() + Separator + fmt.Sprintf("%d", conf.nodeid)
	conf.MinerNum = 1
	conf.Ver = 10000
	conf.TcpPort = 9333
	conf.TcpIp = "127.0.0.1"
	conf.MaxConn = 50
	conf.Halving = 210000
	conf.PowLimit = "00ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	conf.PowSpan = 2016
	conf.PowTime = 2016 * 60 * 10
	conf.AddrPrefix = "st"
	conf.Seeds = []string{"seed.xginx.com"}
	conf.Flags = [4]byte{'T', 'E', 'S', 'T'}
	conf.LimitHash = NewUINT256(conf.PowLimit)
	lis := newTestLis()
	conf.MinerAddr = lis.addrs[0] //0作为矿工账号
	//测试区块索引
	bi := InitBlockIndex(lis)
	for i := 0; i < num; i++ {
		blk, err := bi.NewBlock(1)
		if err != nil {
			panic(err)
		}
		err = blk.Finish(bi)
		if err != nil {
			panic(err)
		}
		calcbits(bi, blk)
		err = bi.LinkBlk(blk)
		if err != nil {
			panic(err)
		}
	}
	LogInfof("test create %d block in %s", num, conf.DataDir)
	return bi
}

func TestRePushTx(t *testing.T) {
	bi := NewTestBlockIndex(100)
	defer bi.Close()
	lis := bi.lptr.(*TestLis)
	mi := bi.NewMulTrans()
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
		if err != nil {
			t.Fatal(err)
		}
		err = tx.Check(bi, true)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = tx
		}
	}
	if bi.txp.Len() != 10 {
		t.Fatal("tx pool count error")
	}
	//创建区块打包
	blk, err := bi.NewBlock(1)
	if err != nil {
		t.Fatal(err)
	}
	//只打包第一个交易
	err = blk.AddTx(bi, first)
	if err != nil {
		t.Fatal(err)
	}
	err = blk.Finish(bi)
	if err != nil {
		t.Fatal(err)
	}
	calcbits(bi, blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		t.Fatal(err)
	}
	//剩下的9个交易应该是恢复进去的
	if bi.txp.Len() != 9 {
		t.Fatal("tx pool count error")
	}
	//目标应该有10个 但打包的只有一个
	ds, err := bi.ListCoins(dst)
	if err != nil {
		t.Fatal(err)
	}
	//总的101个区块减去转出的,多了一个区块，多奖励50，所以应该是101
	if ds.All.Balance() != 10*COIN {
		t.Fatal("dst coin error")
	}
	if ds.Indexs.Balance() != 1*COIN {
		t.Fatal("dst coin error")
	}
	//打包剩下的交易
	//创建区块打包
	blk, err = bi.NewBlock(1)
	if err != nil {
		t.Fatal(err)
	}
	//只打包第一个交易
	err = blk.LoadTxs(bi)
	if err != nil {
		t.Fatal(err)
	}
	err = blk.Finish(bi)
	if err != nil {
		t.Fatal(err)
	}
	calcbits(bi, blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		t.Fatal(err)
	}
	//剩下交易应该全部被打包了
	if bi.txp.Len() != 0 {
		t.Fatal("tx pool count error")
	}
	//目标应该有10个
	ds, err = bi.ListCoins(dst)
	if err != nil {
		t.Fatal(err)
	}
	//总的101个区块减去转出的,多了一个区块，多奖励50，所以应该是101
	if ds.All.Balance() != 10*COIN {
		t.Fatal("dst coin error")
	}
}

//测试转账
func TestTransfer(t *testing.T) {
	bi := NewTestBlockIndex(100)
	defer bi.Close()
	lis := bi.lptr.(*TestLis)
	mi := bi.NewMulTrans()
	//0 -> 1
	mi.Acts = []*Account{lis.ams[0]}
	dst, _ := lis.ams[1].GetAddress()
	mi.Dst = []Address{dst}
	mi.Amts = []Amount{10 * COIN}
	mi.Fee = 1 * COIN
	//创建交易
	tx, err := mi.NewTx(true)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.Check(bi, true)
	if err != nil {
		t.Fatal(err)
	}
	if bi.txp.Len() != 1 {
		t.Fatal("tx pool count error")
	}
	//创建区块打包
	blk, err := bi.NewBlock(1)
	if err != nil {
		t.Fatal(err)
	}
	err = blk.LoadTxs(bi)
	if err != nil {
		t.Fatal(err)
	}
	err = blk.Finish(bi)
	if err != nil {
		t.Fatal(err)
	}
	calcbits(bi, blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		t.Fatal(err)
	}
	//打包成功后交易池应该为空
	if bi.txp.Len() != 0 {
		t.Fatal("tx pool count error")
	}
	//目标应该获得10个
	ds, err := bi.ListCoins(dst)
	if err != nil {
		t.Fatal(err)
	}
	if ds.All.Balance() != 10*COIN {
		t.Fatal("dst coin error")
	}
	//目标应该有一个交易
	txs, err := bi.ListTxs(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 {
		t.Fatal("txs count error")
	}
	//目标交易id应该=创建的交易id
	if !txs[0].TxId.Equal(tx.MustID()) {
		t.Fatalf("tx id error %v != %v", txs[0], tx)
	}
	//转账的应该少了10个coin
	ds, err = bi.ListCoins(conf.MinerAddr)
	if err != nil {
		t.Fatal(err)
	}
	//总的101个区块减去转出的,多了一个区块，多奖励50，所以应该是101
	if ds.All.Balance() != 101*50*COIN-10*COIN {
		t.Fatal("src coin error")
	}
}

//测试成熟的coinbase
func TestMtsBlock(t *testing.T) {
	bi := NewTestBlockIndex(100)
	defer bi.Close()
	if bi.Len() != 100 {
		t.Fatal("create 100 block error")
	}
	ds, err := bi.ListCoins(conf.MinerAddr)
	if err != nil {
		t.Fatal(err)
	}
	//100个区块应该得到500个
	if ds.All.Balance() != 100*50*COIN {
		t.Fatal("all coin count error")
	}
	//只有一个成熟
	if ds.Indexs.Balance() != 50*COIN {
		t.Fatal("Mts coin count error")
	}
	//成熟的那个应该是区块0
	if ds.Indexs[0].Height != 0 {
		t.Fatal("mts index error")
	}
}

func TestCreateBlock(t *testing.T) {
	bi := NewTestBlockIndex(10)
	defer bi.Close()
	if bi.Len() != 10 {
		t.Fatal("create 10 block error")
	}
	ds, err := bi.ListCoins(conf.MinerAddr)
	if err != nil {
		t.Fatal(err)
	}
	//10个区块应该得到500个
	if ds.All.Balance() != 500*COIN {
		t.Fatal("coin count error")
	}
	//并且都未成熟
	if ds.NotMts.Balance() != 500*COIN {
		t.Fatal("coin count error")
	}
	//和矿工相关的交易应该有10个，都是coinbase
	ts, err := bi.ListTxs(conf.MinerAddr)
	if len(ts) != 10 {
		t.Fatal("tx count error")
	}
	//所有交易应该都是coinbase
	for _, v := range ts {
		tx, err := bi.LoadTX(v.TxId)
		if err != nil {
			t.Fatal(err)
		}
		if !tx.IsCoinBase() {
			t.Fatal("coinbase error")
		}
	}
}
