package xginx

import (
	"container/list"
	"errors"
	"sync"
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

type TagCacher interface {
	Del(id TagUID)
	Get(id TagUID) (*TTagInfo, error)
	Set(tx *TTagInfo) (*TTagInfo, error)
	Push(cv ...ICacher)
	Pop(n ...int)
}

var (
	CacheNotFoundErr = errors.New("cache not found")
	//block cache
	Txs = newTxs()
)

type tagcacher struct {
	mu sync.Mutex
	xs ICacher
	lv *list.List
}

func newTxs() TagCacher {
	v := &tagcacher{
		lv: list.New(),
	}
	v.xs = newMemCacher()
	v.lv.PushBack(v.xs)
	return v
}

func (db *tagcacher) Push(cv ...ICacher) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if len(cv) == 0 {
		db.xs = newMemCacher()
		db.lv.PushBack(db.xs)
		return
	}
	for _, v := range cv {
		db.xs = v
		db.lv.PushBack(db.xs)
	}
}

func (db *tagcacher) Pop(n ...int) {
	db.mu.Lock()
	defer db.mu.Unlock()
	num := 1
	if len(n) > 0 && n[0] > 0 {
		num = n[0]
	}
	for ; num > 0 && db.lv.Len() > 1; num-- {
		db.lv.Remove(db.lv.Back())
		db.xs = db.lv.Back().Value.(ICacher)
	}
}

func (db *tagcacher) Set(tx *TTagInfo) (*TTagInfo, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	v, err := db.xs.Set(tx.UID[:], tx)
	if err != nil {
		return nil, err
	}
	return v.(*TTagInfo), nil
}

func (db *tagcacher) Del(id TagUID) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.xs.Del(id[:])
}

func (db *tagcacher) Get(id TagUID) (*TTagInfo, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	v, err := db.xs.Get(id[:])
	if err != nil {
		return nil, err
	}
	return v.(*TTagInfo), nil
}
