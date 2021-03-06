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
func (lis *TestLis) OnUnlinkBlock(blk *BlockInfo) {

}

//更新区块数据成功时
func (lis *TestLis) OnLinkBlock(blk *BlockInfo) {

}

//当块创建时，可以添加，修改块内信息
func (lis *TestLis) OnNewBlock(blk *BlockInfo) error {
	return DefaultNewBlock(lis, blk)
}

//完成区块，当检测完成调用,设置merkle之前
func (lis *TestLis) OnFinished(blk *BlockInfo) error {
	return DefaultkFinished(blk)
}

//当收到网络数据时,数据包根据类型转换成需要的包
func (lis *TestLis) OnClientMsg(c *Client, msg MsgIO) {

}

//当加载交易列表到区块时可用此方法过滤不加入区块的
//调用 AddTxs 时会触发
func (lis *TestLis) OnLoadTxs(txs []*TX) []*TX {
	return txs
}

//链关闭时
func (lis *TestLis) OnClose() {

}

//当服务启动后会调用一次
func (lis *TestLis) OnStart() {

}

//系统结束时
func (lis *TestLis) OnStop() {

}

//当交易进入交易池之前，返回错误不会进入交易池
func (lis *TestLis) OnTxPool(tx *TX) error {
	return nil
}

//当交易池的交易被替换时
func (lis *TestLis) OnTxPoolRep(old *TX, new *TX) {

}

//GetAccount 获取测试账户0-4
func (lis *TestLis) MinerAddr() Address {
	return lis.addrs[0]
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

//NewTempDir 创建一个随机的临时目录
func NewTempDir() string {
	conf := &Config{}
	rid := conf.GenUInt64()
	return os.TempDir() + Separator + fmt.Sprintf("%d", rid)
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
func NewTestBlockIndex(num int) *BlockIndex {
	//测试配置文件
	lis := newTestLis(5)
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
