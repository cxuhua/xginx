package xginx

import (
	"encoding/binary"
	"log"
	"testing"
	"time"
)

func TestBaseBits(t *testing.T) {
	conf = LoadConfig("test.json")
	limit := NewUINT256(conf.PowLimit)
	if limit.Compact(false) != 0x1d00ffff {
		t.Errorf("base bits error")
	}

	x := int64(-10000)
	ux := uint64(x) << 1
	if x < 0 {
		ux = ^ux
	}
	log.Println(ux)
	buf := make([]byte, 10)
	log.Println(binary.PutVarint(buf, -10000))
	log.Println(buf)
}

//201600 bits compute
//lastBlock = 2016599
//(lastBlock-2016 + 1) -> 2012-09-20 06:14:11
//lastBlock -> 2012-10-03 09:17:01
//lastBlock Bits = 0x1a05db8b
//result:1a057e08
func TestCalculateWorkRequired(t *testing.T) {
	t1, _ := time.Parse("2006-01-02 15:04:05", "2012-09-20 06:14:11")
	t2, _ := time.Parse("2006-01-02 15:04:05", "2012-10-03 09:17:01")
	x := CalculateWorkRequired(uint32(t2.Unix()), uint32(t1.Unix()), 0x1a05db8b)
	if x != 0x1a057e08 {
		t.Errorf("failed")
	}
}

//Check whether a block hash satisfies the proof-of-work requirement specified by nBits
func TestCheckProofOfWork(t *testing.T) {
	conf = LoadConfig("test.json")
	h := NewHASH256("0000000000000000000e20e727e0f9e4d88c44d68e572fbc9a2bd8c61e50010b")
	b := CheckProofOfWork(h, 0x1715b23e)
	if !b {
		t.Errorf("test failed")
	}
	b = CheckProofOfWork(NewHASH256("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"), 0x1d00ffff)
	if !b {
		t.Errorf("test 0 height block failed")
	}
}
