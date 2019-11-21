package xginx

import (
	"log"
	"testing"

	"github.com/syndtr/goleveldb/leveldb/filter"
)

func TestBloom(t *testing.T) {
	b := filter.NewBloomFilter(10)
	log.Println(b.Name())
	b.NewGenerator()
}
