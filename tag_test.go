package xginx

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
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
	tkey = "JvQcZnKs2bI3RDO5"
	//标签uid
	tuid = TagUID{0x04, 0x7A, 0x17, 0x32, 0xAA, 0x61, 0x80}
)

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
	err := UseSession(context.Background(), func(db DBImp) error {
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
	err := UseSession(context.Background(), func(db DBImp) error {
		loc := Location{}
		loc.Set(180.14343, -85.2343434)
		tk := &TTagInfo{}
		tk.UID = tuid.Bytes()
		tk.Ver = 1
		tk.Loc = loc.Get()
		tk.CTR = 1
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
	tag := NewTagInfo()
	tag.TLoc.Set(21.44545, -19.1122)
	s, err := tag.EncodeHex()
	log.Println(string(s), err, tag.pos)
}

//最终数据
func TestVerfyData(t *testing.T) {
	//验证数据块
	b, err := NewBlockInfo(HexDecode("434301000000e450c80cb0b59bf4047a1732aa618000005db58047478e511696a013ea4880fb044301020305060708090001020304050600000000000000000000000000000000001c4ff23fe22fd115037bf0b06cdc10b371886ea3454677cd6bb61ad8a3152418301a107ecd74f941ac3045022100a961f4c18ae6fa5790a7afb6c94093ecc81f7af4974795b0de791f49ff0e7dbf022079ac110b5e413430f74f84a71323a975b23695a66b175b50c27612cbd8bf3b9600000000971947a6f831086ac0a12241e22fd11502d63d8d3ce68653c1505295ae7907b084216e91e2ac8afbd241d5a5959c5544223045022100b9e3dd66b768727f1f58a4942640ab52c69a408851bde0d375f3032a8dd97baa022055de6b0e3617438161d1f5ccb62b14d7652dd3cd1b5cbf3c7300fdc81d350f7000000000"))
	if err != nil {
		panic(err)
	}
	pool := conf.ClonePool()
	if err := b.Verify(pool); err != nil {
		t.Error(err)
	}
}

//https://api.xginx.com/sign/CC01000000E450C80CB0B59BF4047A1732AA618000005DB58047478E511696
//CC01000000E450C80CB0B59BF4047A1732AA618000005DB58047478E511696
func TestTagData(t *testing.T) {
	err := UseSession(context.Background(), func(db DBImp) error {
		surl := "https://api.xginx.com/sign/CC01000000E450C80CB0B59BF4047A1732AA618000005DB58047478E511696"
		otag := NewTagInfo(surl)
		//客户端服务器端都要解码
		if err := otag.DecodeURL(); err != nil {
			panic(err)
		}
		sigb, err := otag.ToSigBinary()
		if err != nil {
			panic(err)
		}
		//模拟客户端签名
		pk, err := LoadPrivateKey(cpkey)
		if err != nil {
			panic(err)
		}
		client := &ClientBlock{}
		client.CLoc.Set(122.33, 112.44)
		client.Prev = HashID{1, 2, 3, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6}
		client.CTime = time.Now().UnixNano()
		if err := client.Sign(pk, sigb); err != nil {
			panic(err)
		}
		//校验客户端数据
		err = otag.Valid(db, client)
		if err != nil {
			return err
		}
		tb := &ServerBlock{}
		//从证书池获取可用证书签名
		pool := conf.ClonePool()
		bb, err := tb.Sign(pool, otag, client)
		if err != nil {
			return err
		}
		//
		vv, err := bb.Decode()
		if err != nil {
			return err
		}
		if vv.ClientBlock.CTime != client.CTime {
			return errors.New("time cmp error")
		}
		if vv.ServerBlock.STime != tb.STime {
			return errors.New("stime cmp error")
		}
		log.Printf(hex.EncodeToString(bb))
		log.Println("LEN=", len(bb), bb.Hash())
		return err
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
