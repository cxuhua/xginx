package xginx

import (
	"bytes"
	"context"
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
	//
	tuid = TagUID{0x04, 0x7A, 0x17, 0x32, 0xAA, 0x61, 0x80}
)

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
		tk.PKS = spkey
		tk.Loc = loc.Get()
		tk.CTR = 1
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

//https://api.xginx.com/sign/CC01000000BCB45F6B764532CD047A1732AA618002F9A4DCD3D1DD0E531F76C32A4AA8B123F299909E8FC7D94A4F22E270DA80906300005B 45D28CEB0096724D
//CC01000000BCB45F6B764532CD047A1732AA618002F9A4DCD3D1DD0E531F76C32A4AA8B123F299909E8FC7D94A4F22E270DA80906300005B45D28CEB0096724D
func TestTagData(t *testing.T) {
	spk, err := LoadPrivateKey(spkey)
	if err != nil {
		panic(err)
	}
	err = UseTransaction(context.Background(), func(db DBImp) error {
		pk, err := LoadPrivateKey(cpkey)
		if err != nil {
			panic(err)
		}
		surl := "https://api.xginx.com/sign/CC01000000BCB45F6B764532CD047A1732AA618002F9A4DCD3D1DD0E531F76C32A4AA8B123F299909E8FC7D94A4F22E270DA80906300005B45D28CEB0096724D"
		otag := NewTagInfo(surl)
		//客户端服务器端都要解码
		if err := otag.DecodeURL(); err != nil {
			panic(err)
		}
		sigb, err := otag.ToSigBinary()
		if err != nil {
			panic(err)
		}

		//客户端签名
		client := &ClientBlock{}
		client.CLoc.Set(122.33, 112.44)
		client.Prev = HashID{1, 2, 3, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6}
		client.CTime = time.Now().UnixNano()
		if err := client.Sign(pk, sigb); err != nil {
			panic(err)
		}
		err = otag.Valid(db, client)
		if err != nil {
			return err
		}
		tb := &TagBlock{}
		bb, err := tb.Sign(spk, otag, client)
		if err != nil {
			return err
		}
		vv, err := bb.Decode()
		if err != nil {
			return err
		}
		if !bytes.Equal(vv.TagInfo.TPKS[:], otag.TPKS[:]) {
			return errors.New("cmp tag tpks error")
		}
		if vv.ClientBlock.CTime != client.CTime {
			return errors.New("time cmp error")
		}
		if vv.TagBlock.STime != tb.STime {
			return errors.New("stime cmp error")
		}
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
