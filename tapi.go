package xginx

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

//测试用接口

//TestLis 测试用监听器
type TestLis struct {
	Listener
	addrs []Address
	acc   map[HASH160]*Account
	ams   []*Account
	t     uint32
}

func newTestLis(accnum int) *TestLis {
	lis := &TestLis{
		t:   uint32(time.Now().Unix()),
		acc: map[HASH160]*Account{},
	}
	for i := 0; i < accnum; i++ {
		//创建5-1账号，启用仲裁
		acc, err := NewAccount(5, 1, true)
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
	LogInfof("create %d test account", accnum)
	return lis
}

//GetAccount 获取测试账户0-4
func (lis *TestLis) GetAccount(i int) *Account {
	return lis.ams[i]
}

//TimeNow 测试用时间返回
func (lis *TestLis) TimeNow() uint32 {
	atomic.AddUint32(&lis.t, 1)
	return atomic.LoadUint32(&lis.t)
}

//OnInit 测试用
func (lis *TestLis) OnInit(bi *BlockIndex) error {
	//测试每次清楚数据
	return bi.RemoveBestValue()
}

//OnSignTx 签名器
func (lis *TestLis) OnSignTx(signer ISigner) error {
	_, in, out, _ := signer.GetObjs()
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

//NewTestConfig 创建一个测试用的配置
func NewTestConfig(dir ...string) *Config {
	conf = &Config{}
	conf.nodeid = conf.GenUInt64()
	if len(dir) > 0 && dir[0] != "" {
		conf.DataDir = dir[0]
	} else {
		conf.DataDir = os.TempDir() + Separator + fmt.Sprintf("%d", conf.nodeid)
	}
	conf.MinerNum = 1
	conf.Ver = 10000
	conf.TCPPort = 9333
	conf.TCPIp = "127.0.0.1"
	conf.MaxConn = 50
	conf.Halving = 210000
	conf.PowLimit = "00ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	conf.PowSpan = 2016
	conf.PowTime = 2016 * 60 * 10
	conf.Seeds = []string{"seed.xginx.com"}
	conf.flags = [4]byte{'T', 'E', 'S', 'T'}
	conf.LimitHash = NewUINT256(conf.PowLimit)
	return conf
}

//CloseTestBlock 关闭测试用区块链
func CloseTestBlock(bi *BlockIndex) {
	LogInfof("remove temp dir = %s", conf.DataDir)
	bi.Close()
	_ = os.RemoveAll(conf.DataDir)
}

//GetTestAccount 获取测试用账号
func GetTestAccount(bi *BlockIndex) []*Account {
	lis := bi.lptr.(*TestLis)
	return lis.ams
}

//NewTestOneBlock 从交易池获取交易打包区块测试
func NewTestOneBlock() error {
	bi := GetBlockIndex()
	blk, err := bi.NewBlock(1)
	if err != nil {
		return err
	}
	err = blk.LoadTxs(bi)
	if err != nil {
		return err
	}
	err = blk.Finish(bi)
	if err != nil {
		return err
	}
	calcbits(bi, blk)
	err = bi.LinkBlk(blk)
	if err != nil {
		return err
	}
	return nil
}

//GetTestListener 获取测试监听
func GetTestListener(bi *BlockIndex) *TestLis {
	return bi.lptr.(*TestLis)
}

//NewTestBlockIndex 创建一个测试用区块索引
//num创建num个区块
func NewTestBlockIndex(num int, miner ...Address) *BlockIndex {
	//测试配置文件
	lis := newTestLis(5)
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
