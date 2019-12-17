package xginx

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

//测试用接口

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
	for i := 0; i < int(acc.Num); i++ {
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

func NewTestConfig() {
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
}

func CloseTestBlock(bi *BlockIndex) {
	bi.Close()
	os.RemoveAll(conf.DataDir )
}

func GetTestAccount(bi *BlockIndex)[]*Account {
	lis := bi.lptr.(*TestLis)
	return lis.ams
}

//创建一个测试用区块索引
//num创建num个区块
func NewTestBlockIndex(num int, miner ...Address) *BlockIndex {
	//测试配置文件
	lis := newTestLis()
	if len(miner) > 0 {
		conf.MinerAddr = miner[0]
	} else {
		conf.MinerAddr = lis.addrs[0] //0作为矿工账号
	}
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
