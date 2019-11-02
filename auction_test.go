package xginx

import (
	"log"
	"testing"
	"time"
)

func TestAuctionScript(t *testing.T) {

	b := &BlockInfo{Txs: []*TX{}}

	objId := HASH160{100}

	//第一个竞价者
	pri1, err := NewPrivateKey()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	out1 := &TxOut{}
	out1.Value = 200

	ss1 := &AuctionScript{}
	ss1.Type = SCRIPT_AUCTION_TYPE
	ss1.Time = time.Now().UnixNano()
	ss1.Owner = HASH160{1}
	ss1.ObjId = objId
	if err := ss1.Sign(out1.Value, pri1); err != nil {
		t.Error(err)
		t.FailNow()
	}
	s1, err := ss1.ToScript()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	out1.Script = s1

	tx1 := &TX{Outs: []*TxOut{out1}}
	b.Txs = append(b.Txs, tx1)

	//第二个竞价者
	pri2, err := NewPrivateKey()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	out2 := &TxOut{}
	out2.Value = 200

	ss2 := &AuctionScript{}
	ss2.Type = SCRIPT_AUCTION_TYPE
	ss2.Time = time.Now().UnixNano()
	ss2.Owner = HASH160{1}
	ss2.ObjId = objId
	if err := ss2.Sign(out2.Value, pri2); err != nil {
		t.Error(err)
		t.FailNow()
	}
	s2, err := ss2.ToScript()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	out2.Script = s2

	tx2 := &TX{Outs: []*TxOut{out2}}
	b.Txs = append(b.Txs, tx2)

	asv, err := out2.IsBidHighest(objId, b)
	log.Println(asv, err)
}
