package xginx

import (
	"testing"
)

func TestAuctionScript(t *testing.T) {
	//出价人
	bpri, _ := NewPrivateKey()
	//物品
	opri, _ := NewPrivateKey()
	objId := ObjectId{}
	objId.BHB = 0 //区块开始高度
	objId.BHE = 5 //区块结束高度
	objId.OID = opri.PublicKey().Hash()

	obj2 := ObjectId{}
	if err := obj2.From(objId.String()); err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !obj2.Equal(objId) {
		t.Error("equal object error")
		t.FailNow()
	}

	auclock := AucLockScript{}
	auclock.Type = SCRIPT_AUCLOCKED_TYPE
	auclock.BidId = bpri.PublicKey().Hash()
	auclock.ObjId = objId

	out := &TxOut{}
	//出价200
	out.Value = 200
	out.Script = auclock.ToScript()

	//解锁脚本
	aucunl := AucUnlockScript{}
	aucunl.Type = SCRIPT_AUCUNLOCK_TYPE
	aucunl.BidPks.Set(bpri.PublicKey())
	aucunl.ObjPks.Set(opri.PublicKey())
}
