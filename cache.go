package xginx

import (
	"errors"
	"time"

	"github.com/patrickmn/go-cache"
)

type ICacher interface {
	Del(id []byte)
	Get(id []byte) (interface{}, error)
	Set(id []byte, v interface{}) (interface{}, error)
}

type memcacher struct {
	c *cache.Cache
}

func newMemCacher() ICacher {
	return &memcacher{
		c: cache.New(time.Minute*10, time.Minute*30),
	}
}

func (c *memcacher) Del(id []byte) {
	c.c.Delete(string(id[:]))
}

func (c *memcacher) Get(id []byte) (interface{}, error) {
	v, ok := c.c.Get(string(id[:]))
	if ok && v == nil {
		c.c.Delete(string(id[:]))
		return nil, CacheNotFoundErr
	}
	if !ok || v == nil {
		return nil, CacheNotFoundErr
	}
	return v, nil
}

func (c *memcacher) Set(id []byte, v interface{}) (interface{}, error) {
	c.c.Set(string(id[:]), v, cache.DefaultExpiration)
	return v, nil
}

//用于缓存数据库数据
var (
	CacheNotFoundErr = errors.New("cache not found")
	//unit cache
	CUnit = newMemCacher()
	//tag cache
	CTag = newMemCacher()
	//block cache
	CBlock = newMemCacher()
	//tx cache
	CTx = newMemCacher()
)
