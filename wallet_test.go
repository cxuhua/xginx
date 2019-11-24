package xginx

import (
	"log"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
)

func TestCache(t *testing.T) {
	c := cache.New(time.Second*3, time.Second*3)
	c.Set("aa", 111, time.Second)
	time.Sleep(time.Second * 2)
	log.Println(c.Get("aa"))
}
