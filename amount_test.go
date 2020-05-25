package xginx

import (
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type Time int64

//MarshalJSON json编码重定义
func (t Time) MarshalJSON() ([]byte, error) {
	s := time.Unix(int64(t), 0).Format("2006-01-02 15:04:05")
	return []byte(`"` + s + `"`), nil
}

//UnmarshalJSON json解码重定义
func (t *Time) UnmarshalJSON([]byte) error {
	*t = 12
	return nil
}

//Now 赋值当前时间
func (t *Time) Now() {
	*t = Time(time.Now().Unix())
}

type MyTime struct {
	Time
}

func (t MyTime) MarshalJSON() ([]byte, error) {
	s := time.Unix(int64(t.Time), 0).Format("2006-01-02 15:04:05")
	return []byte(`"` + s + `"`), nil
}

//UnmarshalJSON json解码重定义
func (t *MyTime) UnmarshalJSON(b []byte) error {
	log.Println(string(b[1 : len(b)-1]))
	v, err := time.Parse("2006-01-02 15:04:05", string(b[1:len(b)-1]))
	if err != nil {
		return err
	}
	t.Time = Time(v.Unix())
	return nil
}

func TestFinalSeq(t *testing.T) {
	v := VarUInt(FinalSequence)
	assert.Equal(t, len(v.Bytes()), 4)
}

func TestUseMaxMoney(t *testing.T) {
	if uint64(MaxMoney) > MaxCompressUInt {
		t.Errorf("can't use amount compress")
	}
	type A struct {
		A MyTime `bson:"time" json:"time"`
	}
	a := A{}
	a.A.Now()
	d, err := json.Marshal(a)
	log.Println(string(d), err)
	b := &A{}
	json.Unmarshal(d, b)
	log.Println(b.A)
}

func TestAmountPut(t *testing.T) {
	a := Amount(100000)
	b := a.Bytes()
	c := Amount(0)
	c.From(b)
	if a != c {
		t.Error("test bytes from error")
	}
}

func TestAmountDecodeEncode(t *testing.T) {
	buf := NewReadWriter()
	a := MaxMoney
	err := a.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	b := Amount(0)
	err = b.Decode(buf)
	if err != nil {
		t.Error(err)
	}
	if a != b {
		t.Errorf("MAX_MONEY equal test error")
	}
}
