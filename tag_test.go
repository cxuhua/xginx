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
//434301000000e450c80cb0b59bf4047a1732aa618000005db58047478e511696a013ea4880fb04430102030506070809000102030405060000000000000000000000000000000000bc82f8369d21d115037bf0b06cdc10b371886ea3454677cd6bb61ad8a3152418301a107ecd74f941ac30460221009870021662955c792f2e4a7a5e12c445310f4d7adc6c42794283694d1e2062e0022100b46ff32254d2bb10b813390b761046d1bd2a754c6d0ff3507d22db5d2364fbf20000000b73a6fdd7b4b82eb4c6d28c9f21d11502f9a4dcd3d1dd0e531f76c32a4aa8b123f299909e8fc7d94a4f22e270da809063304502201028f56f9a9711177e5367c0f3e2a499695c699a620a2c2690dbfc861aeeae43022100ee3e1eab3c2f84e7af5acf216652f361322de50998cd80bace6fd063641a7b6700000000
func TestVerfyData(t *testing.T) {
	//验证数据块
	b, err := NewBlockInfo(HexDecode("434301000000e450c80cb0b59bf4047a1732aa618000005db58047478e511696a013ea4880fb0443010203050607080900010203040506000000000000000000000000000000000018ce4182662bd115037bf0b06cdc10b371886ea3454677cd6bb61ad8a3152418301a107ecd74f941ac3045022024106ea12acfb1df3ce62704f8615b2ee53a28466ff8aae15dbd7a2bad6a42af022100f1c27b5fd478aa4ed9a94a18b90c4340ec3f8c688e7dbedf7ee72228ba9f249700000000b6ed826eed8a27e1ec957283662bd115020e82995d0fa68e3eabe93b2d839cc95f72b41e632789c244f2a088865e7971e93046022100fb654dad9f4d38654ad6fe62b6f108d497427a712b7ddb72b9dd541422590164022100e76e46878c6c4cc1a6292b174b288be943f1fe4e41a3ae94ce55ea3999034cf7000000"))
	if err != nil {
		panic(err)
	}
	if err := b.Verify(); err != nil {
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
		//使用证书1签名数据
		cert, err := Conf.GetNodeCert(1)
		if err != nil {
			return err
		}
		bb, err := tb.Sign(cert, otag, client)
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
