package xginx

import (
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

func TestLoadTagInfo(t *testing.T) {
	defer DB().Close()
	ti, err := LoadTagInfo(tuid)
	if err != nil {
		panic(err)
	}
	log.Println(ti.CCTR.ToUInt())
}

func TestSaveTagInfo(t *testing.T) {
	defer DB().Close()
	pri, err := LoadPrivateKey(cpkey)
	if err != nil {
		panic(err)
	}
	ti := &TTagInfo{}
	ti.CPKS.Set(pri.PublicKey())
	ti.Time = time.Now().UnixNano()
	ti.CCTR.Set(10)
	err = ti.Save(tuid)
	if err != nil {
		panic(err)
	}
}

func TestLoadTestKey(t *testing.T) {
	defer DB().Close()
	kv, err := LoadTagKeys(tuid)
	if err != nil {
		panic(err)
	}
	log.Println(kv)
}

func TestSaveTestKey(t *testing.T) {
	defer DB().Close()
	tk := &TTagKeys{}
	tk.TVer = 1
	tk.TPKS = spkey
	tk.TLoc.Set(180.14343, -85.2343434)
	copy(tk.Keys[0][:], tkey)
	err := tk.Save(tuid)
	if err != nil {
		panic(err)
	}
}

//https:// api.xginx.com/sign/CC01000000BCB45F6B764532CD047A1732AA618002F9A4DCD3D1DD0E531F76C32A4AA8B123F299909E8FC7D94A4F22E270DA80906300005B 45D28CEB0096724D
//CC01000000BCB45F6B764532CD047A1732AA618002F9A4DCD3D1DD0E531F76C32A4AA8B123F299909E8FC7D94A4F22E270DA80906300005B45D28CEB0096724D
func TestTagData(t *testing.T) {
	surl := "https://api.xginx.com/sign/CC01000000BCB45F6B764532CD047A1732AA618002F9A4DCD3D1DD0E531F76C32A4AA8B123F299909E8FC7D94A4F22E270DA80906300005B45D28CEB0096724D"
	otag := NewTagInfo(surl)
	err := otag.Valid()
	if err != nil {
		t.Error(err)
	}
	pos := &TagPos{}
	log.Println(otag.Encode(pos))
}

func TestMaxBits(t *testing.T) {
	for i := uint(0); i < 64; i++ {
		xx := uint64(1 << i)
		if MaxBits(xx) != i {
			t.Errorf("error %x", xx)
		}
	}
}
