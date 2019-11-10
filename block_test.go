package xginx

import (
	"errors"
	"log"
	"testing"
	"time"
)

var (
	//测试用矿工私钥
	TestMinerPrivateKey = "L4eSSzfWoTB9Y3eZo4Wp9TBPBsTJCcwmbioRcda3cM86MnUMrXhN"
	TestMinePri, _      = LoadPrivateKey(TestMinerPrivateKey)
	//测试用客户端key
	TestCliPrivateKey = "KzVa4aqLziZWuiKFkPRkM46ZTrdzJhfuUxbe8pmxgosjoEYYnZuM"
	TestCliPri, _     = LoadPrivateKey(TestCliPrivateKey)
	//测试用标签私钥
	TestTagPrivateKey = "Kyxcf5FAp2tneFB1ZXzNfZTYpu28fQ7DF99aqUZ3NgtD6hJEJ8zJ"
	TestTagPri, _     = LoadPrivateKey(TestTagPrivateKey)

	tuid1 = TagUID{0x04, 0x7D, 0x14, 0x32, 0xAA, 0x61, 0x80}
	tuid2 = TagUID{0x04, 0x7D, 0x14, 0x32, 0xAA, 0x61, 0x81}
)

//创建测试用单元数据
func newTestUnit(uid TagUID, lng float64, lat float64, time int64, prev HASH256) *Unit {
	u := &Unit{}
	u.TUID = uid
	u.TLoc.Set(lng, lat)
	u.TPKH = TestTagPri.PublicKey().Hash()
	u.TASV = S631
	u.verifyok = true //假设已经验证
	u.CLoc.Set(lng, lat)
	u.Prev = prev
	u.CPks.Set(TestCliPri.PublicKey())
	u.CTime = time
	SetRandInt(&u.Nonce)
	u.STime = time
	return u
}

var (
	testNow = time.Now()
)

func NewTestBlock(bi *BlockIndex) *BlockInfo {
	b := bi.NewBlock()
	cli := TestCliPri.PublicKey()

	cliBestId, _ := bi.GetCliBestId(cli.Hash())

	testNow = testNow.Add(time.Hour * 3)
	u1 := newTestUnit(tuid1, 116.29331, 39.985513, testNow.UnixNano(), cliBestId)

	testNow = testNow.Add(time.Hour * 3)
	u2 := newTestUnit(tuid2, 116.545698, 39.944812, testNow.UnixNano(), u1.Hash())

	us := &Units{u1, u2}

	err := b.AddUnits(bi, us)
	if err != nil {
		panic(err)
	}
	calcer := NewTokenCalcer(TestMinePri.PublicKey().Hash())
	err = b.Finish(bi, calcer)
	if err != nil {
		panic(err)
	}
	return b
}

func TestBlockChain(t *testing.T) {
	bi := NewBlockIndex()
	testnum := uint32(10)
	fb := NewTestBlock(bi)
	conf.genesisId = fb.ID()
	log.Println("genesis_block=", fb.ID())
	_, err := bi.LinkTo(fb)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	for i := uint32(1); i < testnum; i++ {
		cb := NewTestBlock(bi)
		_, err = bi.LinkTo(cb)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if i%10000 == 0 {
			log.Println(i, "block")
		}
		fb = cb
	}
	if bi.Len() != int(testnum) {
		t.Errorf("add main chain error")
		t.FailNow()
	}
	bi.db.Sync()
}

func TestLRUCache(t *testing.T) {
	c := NewCache(4)
	id := HASH256{1}
	v1 := c.Get(id, func() (size int, value Value) {
		log.Println("a1")
		return 5, 100
	})

	log.Println(v1.Value())

	v1 = c.Get(HASH256{3}, func() (size int, value Value) {
		log.Println("a2")
		return 5, 101
	})
	v1.Release()

	v2 := c.Get(HASH256{3}, func() (size int, value Value) {
		log.Println("a3")
		return 5, 102
	})
	log.Println(v2.Value())
}

func TestBlockSign(t *testing.T) {

	bi := NewBlockIndex()
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}

	//获取矿工的所有输出
	ds, err := bi.ListTokens(TestMinePri.PublicKey().Hash())
	if err != nil {
		panic(err)
	}
	//获取标签的输出
	//ds, err = bi.ListTokens(TestTagPri.PublicKey().Hash())
	//if err != nil {
	//	panic(err)
	//}
	////获取用户的输出
	//ds, err = bi.ListTokens(TestCliPri.PublicKey().Hash())
	//if err != nil {
	//	panic(err)
	//}

	b := bi.NewBlock()
	//组装交易
	tx := &TX{Ver: 1}
	ins := []*TxIn{}
	txout := &TxOut{}
	//转到miner
	txout.Script = StdLockedScript(TestMinePri.PublicKey())
	for _, v := range ds {
		ins = append(ins, v.GetTxIn())
		txout.Value += v.Value
	}
	outs := []*TxOut{txout}
	tx.Ins = ins
	tx.Outs = outs
	//添加签名
	err = tx.Sign(bi)
	if err != nil {
		panic(err)
	}
	err = b.AddTx(bi, tx)
	if err != nil {
		panic(err)
	}

	cli := TestCliPri.PublicKey()

	cliBestId, _ := bi.GetCliBestId(cli.Hash())

	u1 := &Unit{}
	u1.CPks.Set(cli)
	u1.Prev = cliBestId
	SetRandInt(&u1.Nonce)
	u1.STime = time.Now().UnixNano()

	u2 := &Unit{}
	u2.CPks.Set(cli)
	u2.Prev = u1.Hash()
	SetRandInt(&u2.Nonce)
	u2.STime = time.Now().UnixNano()

	us := &Units{u1, u2}

	err = b.AddUnits(bi, us)
	if err != nil {
		panic(err)
	}

	calcer := NewTokenCalcer(TestMinePri.PublicKey().Hash())
	err = b.Finish(bi, calcer)
	if err != nil {
		panic(err)
	}
}

func TestUnlinkBlock(t *testing.T) {
	bi := NewBlockIndex()
	err := bi.LoadAll()
	if err != nil {
		panic(err)
	}
	for {
		bv := bi.GetBestValue()
		if !bv.IsValid() {
			log.Println("not has best block")
			break
		}
		last := bi.Last()
		if !bv.Id.Equal(last.ID()) {
			panic(errors.New("best id error"))
		}
		if bv.Height != last.Height {
			panic(errors.New("best height error"))
		}
		b, err := bi.LoadBlock(last.ID())
		if err != nil {
			panic(err)
		}
		err = bi.Unlink(b)
		if err != nil {
			panic(err)
		}
	}
	bi.db.Sync()
}

func TestValueScale(t *testing.T) {
	log.Println(S721.Scale())
	log.Println(S622.Scale())
	log.Println(S631.Scale())
	log.Println(S640.Scale())
	log.Println(S550.Scale())
}

func TestCalcDistance(t *testing.T) {
	bits := NewUINT256(conf.PowLimit).Compact(false)
	calcer := NewTokenCalcer(TestMinePri.PublicKey().Hash())
	now := time.Now().UnixNano()
	//i1 first
	i1 := &Unit{}
	i1.TLoc.Set(104.0658044815, 30.5517656113)
	i1.CTime = now
	i1.CLoc.Set(104.0671670437, 30.5573090657)
	i1.STime = now
	i1.TPKH = HASH160{1}
	i1.TASV = S631

	i2 := &Unit{}
	i2.Prev = i1.Hash()
	i2.TLoc.Set(104.0615880489, 30.5536596605)
	i2.CTime = now + int64(time.Hour)
	i2.CLoc.Set(104.0615880489, 30.5536596605)
	i2.STime = now + int64(time.Hour)
	i2.TPKH = HASH160{2}
	i2.TASV = S622

	i3 := &Unit{}
	i3.Prev = i2.Hash()
	i3.Prev = i2.Hash()
	i3.TLoc.Set(104.0671670437, 30.5573090657)
	i3.CTime = now + int64(time.Hour*2)
	i3.CLoc.Set(104.0671670437, 30.5573090657)
	i3.STime = now + int64(time.Hour*2)
	i3.TPKH = HASH160{3}
	i3.TASV = S721

	is := &Units{i1, i2, i3}
	err := calcer.Calc(bits, is)
	log.Println(calcer, err)
}
