package xginx

import (
	"bytes"
	"encoding/hex"
	"log"
	"testing"

	"github.com/willf/bloom"
)

var (
	//客户测试私钥
	cpkey = "5KMyai4SdgKnoyGX49qEiAXKP8TeDyUyL8PgAHtikmqZJ9WVGtc"
	//服务器测试私钥
	spkey = "L4h8htSsTiLh9axT9i5mzdMtpZdqQiKofdRf9bfJqurh8gbTVtS4"
	//tag aes测试密钥
	tkey = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} //"JvQcZnKs2bI3RDO5"
	//标签uid
	tuid = TagUID{0x04, 0x7D, 0x14, 0x32, 0xAA, 0x61, 0x80}
)

//latitude=30.558675,longitude=104.033261
func TestLocation_Distance(t *testing.T) {
	loc1 := Location{}
	loc1.Set(104.033261, 30.558675)
	loc2 := Location{}
	loc2.Set(116.387271, 39.922501)
	dis := loc1.Distance(loc2)
	log.Println(uint64(dis))
}

func TestVarStr(t *testing.T) {
	buf := &bytes.Buffer{}
	s := VarStr("1245677")
	if err := s.Encode(buf); err != nil {
		panic(err)
	}
	v := VarStr("")
	if err := v.Decode(buf); err != nil {
		panic(err)
	}
	if v != s {
		t.Errorf("encode decode error")
	}
}

func TestSig(t *testing.T) {
	pk := "uxYrjiMMZ2fuXuRih6ty7UVb5ggwYApqM8qTq2BT5sxQ"
	pk1, _ := B58Decode(pk, BitcoinAlphabet)

	sg := "AN1rKvt6EFabeqmHkKsUZtWsA4YHHucwc6eUbaAnkHiRF8x9PvZNn87UQnjSekk3eXThUhmPjrFAkRR5263XiCpucWjSTQBA4"
	sg1, _ := B58Decode(sg, BitcoinAlphabet)

	hv := make([]byte, 32)
	for i, _ := range hv {
		hv[i] = byte(i)
	}
	pub, err := NewPublicKey(pk1)
	if err != nil {
		panic(err)
	}
	sig, err := NewSigValue(sg1)
	if err != nil {
		panic(err)
	}

	ok := pub.Verify(hv, sig)
	log.Println(ok)

	//pri, err := LoadPrivateKey(spkey)
	//if err != nil {
	//	panic(err)
	//}
	//pb := pri.PublicKey().Encode()
	//ps := B58Encode(pb, BitcoinAlphabet)
	//log.Println(ps)
	//sig, err := pri.Sign(hv)
	//if err != nil {
	//	panic(err)
	//}
	//sigdata := sig.Encode()
	//log.Println(len(sigdata), B58Encode(sigdata, BitcoinAlphabet))
}

func TestLoadTestKey(t *testing.T) {

	TagStore.LoadAllTags(bloom.New(1000, 10))

	id, err := hex.DecodeString("9B2FCF46B3352B964E86E0CA47678FDC306A918A386111FFC60208B2795111B4")
	if err != nil {
		panic(err)
	}
	hid := HASH256{}
	copy(hid[:], id)
	TagStore.HasUnitash(hid)
}

func TestSaveTestKey(t *testing.T) {
	pk := "uxYrjiMMZ2fuXuRih6ty7UVb5ggwYApqM8qTq2BT5sxQ"
	pk1, _ := B58Decode(pk, BitcoinAlphabet)
	pub, _ := NewPublicKey(pk1)
	hash := pub.Hash()
	loc := Location{}
	loc.Set(116.368904, 39.923423)
	tk := &TTagInfo{}
	tk.UID = tuid
	tk.Ver = 1
	tk.Loc = loc
	tk.ASV = S622.ToUInt8()
	tk.PKH = hash
	tk.Keys[4] = TTagKey{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	tk.SetMacKey(0)
	copy(tk.Keys[0][:], tkey)
	err := TagStore.SaveTag(tk)
	if err != nil {
		panic(err)
	}
}

func TestMakeTagURL(t *testing.T) {
	pk := "uxYrjiMMZ2fuXuRih6ty7UVb5ggwYApqM8qTq2BT5sxQ"
	pk1, _ := B58Decode(pk, BitcoinAlphabet)
	pub, _ := NewPublicKey(pk1)
	tag := NewTagInfo()
	tag.TLoc.Set(104.062810, 30.552873)
	tag.TASV = S622
	tag.TPKH.SetPK(pub)
	s, err := tag.EncodeHex()
	log.Println(string(s), err, tag.pos)
}

func TestMaxBits(t *testing.T) {
	for i := uint(0); i < 64; i++ {
		xx := uint64(1 << i)
		if MaxBits(xx) != i {
			t.Errorf("error %x", xx)
		}
	}
}
