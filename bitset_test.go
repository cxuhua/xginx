package xginx

import (
	"log"
	"testing"
)

func TestNewBitFrom(t *testing.T) {
	bs := BitSetFrom(make([]byte, 10))
	for i := 0; i < 10*8; i++ {
		bs.SetTo(i, true)
	}
	log.Println(bs.b)
	for i := 0; i < 10*8; i++ {
		bs.SetTo(i, false)
	}
	log.Println(bs.b)
	for i := 0; i < 2; i++ {
		bs.SetTo(i, true)
	}
	log.Println(bs.b)
}
