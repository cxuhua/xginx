package xginx

import (
	"bytes"
	"context"
	"encoding/hex"
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
//434301000000e450c80cb0b59bf4047a1732aa618000005db58047478e511696a013ea4880fb04430000000000000000000000000000000000000000000000000000000000000000442a8b76f670d115037bf0b06cdc10b371886ea3454677cd6bb61ad8a3152418301a107ecd74f941ac304402203f130f323d2750460ac09b4b9d02ff6beb5f2883a1afa374572c860c72b4ac1102207f1e88274fbf5fce3476d1bb0e9cdc0257246b2271e4bc57d9ef5c1be267c9bc0000000000d2749ebf12dfa32564c0677af670d11502d63d8d3ce68653c1505295ae7907b084216e91e2ac8afbd241d5a5959c5544223046022100ce70a86bdc134125c326add7a081cf5f7400dac9296c0f6ac2a79e07c8736827022100a0052a5e1cd398d8251ff4c42d77d396dc02be02ecdbfdddb48da78b00adba88000000
//hash = dab3c131a5072b16faa9149033fadd0cfdf00eba7d7264981d8ac0556f98446a
func TestVerfyData(t *testing.T) {
	//验证数据块
	pool := conf.NewCertPool()
	b, err := VerifyBlockInfo(pool, HexDecode("434301000000e450c80cb0b59bf4047a1732aa618000005db58047478e511696a013ea4880fb04430000000000000000000000000000000000000000000000000000000000000000442a8b76f670d115037bf0b06cdc10b371886ea3454677cd6bb61ad8a3152418301a107ecd74f941ac304402203f130f323d2750460ac09b4b9d02ff6beb5f2883a1afa374572c860c72b4ac1102207f1e88274fbf5fce3476d1bb0e9cdc0257246b2271e4bc57d9ef5c1be267c9bc0000000000d2749ebf12dfa32564c0677af670d11502d63d8d3ce68653c1505295ae7907b084216e91e2ac8afbd241d5a5959c5544223046022100ce70a86bdc134125c326add7a081cf5f7400dac9296c0f6ac2a79e07c8736827022100a0052a5e1cd398d8251ff4c42d77d396dc02be02ecdbfdddb48da78b00adba88000000"))
	if err != nil {
		panic(err)
	}

	err = UseSession(context.Background(), func(db DBImp) error {
		return b.Save(db)
	})
	if err != nil {
		panic(err)
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
		client.Prev = HashID{}
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
		pool := conf.NewCertPool()
		bb, err := tb.Sign(pool, otag, client)
		if err != nil {
			return err
		}
		log.Printf(hex.EncodeToString(bb))
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
