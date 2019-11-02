package xginx

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"
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

func TestLocation_Distance(t *testing.T) {
	loc1 := Location{}
	loc1.Set(116.368904, 39.923423)
	loc2 := Location{}
	loc2.Set(116.387271, 39.922501)
	dis := loc1.Distance(loc2)
	log.Println(uint64(dis))
}

func TestVarStr(t *testing.T) {
	buf := &bytes.Buffer{}
	s := VarStr("1245677")
	if err := s.EncodeWriter(buf); err != nil {
		panic(err)
	}
	v := VarStr("")
	if err := v.DecodeReader(buf); err != nil {
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
	err := store.UseSession(context.Background(), func(db DBImp) error {
		kv, err := LoadTagInfo(tuid, db)
		if err != nil {
			panic(err)
		}
		log.Println(kv)
		return err
	})
	if err != nil {
		panic(err)
	}
}

func TestSaveTestKey(t *testing.T) {
	pk := "uxYrjiMMZ2fuXuRih6ty7UVb5ggwYApqM8qTq2BT5sxQ"
	pk1, _ := B58Decode(pk, BitcoinAlphabet)
	pub, _ := NewPublicKey(pk1)
	hash := pub.Hash()
	err := store.UseSession(context.Background(), func(db DBImp) error {
		loc := Location{}
		loc.Set(116.368904, 39.923423)
		tk := &TTagInfo{}
		tk.UID = tuid.Bytes()
		tk.Ver = 1
		tk.Loc = loc.Get()
		tk.CTR = 1
		tk.ASV = S622
		tk.PKH = hash[:]
		tk.SetMacKey(0)
		copy(tk.Keys[0][:], tkey)
		err := tk.Save(db)
		if err != nil {
			panic(err)
		}
		return err
	})
	if err != nil {
		panic(err)
	}
}

func TestMakeTagURL(t *testing.T) {
	pk := "uxYrjiMMZ2fuXuRih6ty7UVb5ggwYApqM8qTq2BT5sxQ"
	pk1, _ := B58Decode(pk, BitcoinAlphabet)
	pub, _ := NewPublicKey(pk1)
	tag := NewTagInfo()
	tag.TLoc.Set(116.368904, 39.923423)
	tag.TASV = S622
	tag.TPKH.SetPK(pub)
	s, err := tag.EncodeHex()
	log.Println(string(s), err, tag.pos)
}

//http://api.xginx.com/sign/II01000000507C5C45B6D4CB17CADEC3AE1EDE36775302EF0330C5295F714ACF8089000000000000000000000000000000000000
//II01000000507C5C45B6D4CB17CADEC3AE1EDE36775302EF0330C5295F714ACF8089000000000000000000000000000000000000
func TestTagData(t *testing.T) {
	err := store.UseSession(context.Background(), func(db DBImp) error {
		surl := "http://192.168.31.177:9334/sign/OO01000000507C5C45B6D4CB17CADEC3AE1EDE36775302EF0330C5295F714ACF8089047D1432AA6180000010C930ABEAFC1D8553"
		otag := NewTagInfo(surl)
		//客户端服务器端都要解码
		if err := otag.DecodeURL(); err != nil {
			panic(err)
		}
		sigb, err := otag.ToSigBytes()
		if err != nil {
			panic(err)
		}
		//模拟客户端签名
		pk, err := LoadPrivateKey(cpkey)
		if err != nil {
			panic(err)
		}
		client := &CliPart{}
		client.CLoc.Set(122.33, 112.44)
		client.Prev = HASH256{}
		client.CTime = time.Now().UnixNano()
		cb, err := client.Sign(pk, sigb)
		if err != nil {
			panic(err)
		}
		log.Println(hex.EncodeToString(cb))
		//校验客户端数据
		err = otag.Valid(db, client)
		if err != nil {
			return err
		}
		tb := &SerPart{}
		//获取一个可签名的私钥
		spri := conf.GetPrivateKey()
		if spri == nil {
			return errors.New("private key mss")
		}
		//用私钥0签名数据
		bb, err := tb.Sign(spri, otag, client)
		if err != nil {
			return err
		}
		bl, err := VerifyUnit(conf, bb)
		if err != nil {
			return fmt.Errorf("verify sign data error %w", err)
		}
		return bl.Save(db)
	})
	if err != nil {
		panic(err)
	}
}

func TestMaxBits(t *testing.T) {
	for i := uint(0); i < 64; i++ {
		xx := uint64(1 << i)
		if MaxBits(xx) != i {
			t.Errorf("error %x", xx)
		}
	}
}
