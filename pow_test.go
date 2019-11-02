package xginx

import (
	"log"
	"testing"
	"time"
)

func TestPow(t *testing.T) {
	hash := NewHASH256("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f")
	b := CheckProofOfWork(hash, 0x1d00ffff)
	log.Println(b)
}

func TestBaseBits(t *testing.T) {
	limit := NewUINT256(conf.PowLimit)
	if limit.Compact(false) != 0x1d00ffff {
		t.Errorf("base bits error")
	}
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
	h := NewHASH256("00000000000003010530e33a849b27ded874202911e9e63263cb49245744fb9e")
	b := CheckProofOfWork(h, 0x1a0575ef)
	if !b {
		t.Errorf("test failed")
	}
	b = CheckProofOfWork(NewHASH256("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"), 0x1d00ffff)
	if !b {
		t.Errorf("test 0 height block failed")
	}
}
