package xginx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	//地址key前缀
	AddrPrefix = []byte{1}
	//加密的私钥
	EncPKPrefix = []byte{2}
	//未加密
	StdPKPrefix = []byte{3}
)

// AES加密
func AesEncrypt(block cipher.Block, data []byte) ([]byte, error) {
	if block == nil {
		return nil, errors.New("block nil")
	}
	//随机生成iv
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	dl := len(data)
	l := (dl/aes.BlockSize)*aes.BlockSize + aes.BlockSize
	if dl%aes.BlockSize == 0 {
		l = dl
	}
	//add iv length
	dd := make([]byte, l+aes.BlockSize)
	n := l - dl
	//copy iv to dd
	copy(dd[0:], iv)
	//copy data to dd
	copy(dd[aes.BlockSize:], data)
	//fill end bytes
	for i := 0; i < n; i++ {
		dd[dl+i+aes.BlockSize] = byte(n)
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(dd[aes.BlockSize:], dd[aes.BlockSize:])
	return dd, nil
}

//检测最后几个字节是否是加密
func bytesEquInt(data []byte, n byte) bool {
	l := len(data)
	if l == 0 {
		return false
	}
	for i := 0; i < l; i++ {
		if data[i] != n {
			return false
		}
	}
	return true
}

// AES解密
func AesDecrypt(block cipher.Block, data []byte) ([]byte, error) {
	if block == nil {
		return nil, errors.New("block nil")
	}
	bytes := len(data)
	if bytes < 32 || bytes%aes.BlockSize != 0 {
		return nil, errors.New("decrypt data length error")
	}
	//16 bytes iv
	iv := data[:aes.BlockSize]
	dd := data[aes.BlockSize:]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(dd, dd)
	l := len(dd)
	if n := dd[l-1]; n <= aes.BlockSize {
		x := l - int(n)
		if bytesEquInt(dd[x:], n) {
			dd = dd[:x]
		}
	}
	return dd, nil
}

//整理key为 16 24 or 32
func TrimAESKey(key []byte) ([]byte, error) {
	size := len(key) / 8
	if size <= 2 {
		size = 2
	}
	if size > 4 {
		size = 4
	}
	iLen := size * 8
	ikey := make([]byte, iLen)
	if len(key) > iLen {
		copy(ikey[0:], key[:iLen])
	} else {
		copy(ikey[0:], key)
	}
	return ikey, nil
}

//创建加密算法
func NewAESCipher(key []byte) (cipher.Block, error) {
	ikey, err := TrimAESKey(key)
	if err != nil {
		return nil, err
	}
	return aes.NewCipher(ikey)
}

//钱包处理
type IWallet interface {
	//解密一段时间，时间到达后私钥失效
	Decryption(addr string, pw string, time time.Duration) error
	//加密钱包
	Encryption(addr string, pw string) error
	//根据钱包地址获取私钥
	GetAccount(addr string) (*Account, error)
	//关闭钱包
	Close()
	//新建地址
	NewAccount(num uint8, less uint8) (string, error)
	//导入私钥 pw != ""添加密码
	ImportAccount(pri string, pw string) error
	//获取所有地址
	ListAccount() []string
	//移除地址
	RemoveAccount(addr string) error
}

type LevelDBWallet struct {
	done chan bool
	//钱包地址
	mu   sync.RWMutex
	dir  string
	dptr DBImp
	//过期时间
	times map[string]time.Time
	//私钥缓存
	cache map[string]*Account
}

//列出地址
func (db *LevelDBWallet) ListAccount() []string {
	ds := []string{}
	iter := db.dptr.Iterator(NewPrefix(AddrPrefix))
	defer iter.Close()
	for iter.Next() {
		key := iter.Key()
		ds = append(ds, string(key[1:]))
	}
	return ds
}

func (db *LevelDBWallet) RemoveAccount(addr string) error {
	err := db.dptr.Del(AddrPrefix, []byte(addr))
	if err != nil {
		return err
	}
	db.mu.Lock()
	delete(db.cache, addr)
	db.mu.Unlock()
	return nil
}

func (db *LevelDBWallet) NewAccount(num uint8, less uint8) (string, error) {
	acc, err := NewAccount(num, less)
	if err != nil {
		return "", err
	}
	addr, err := acc.GetAddress()
	if err != nil {
		return "", err
	}
	dump, err := acc.Dump()
	if err != nil {
		return "", err
	}
	vbs := append([]byte{}, StdPKPrefix...)
	vbs = append(vbs, []byte(dump)...)
	err = db.dptr.Put(AddrPrefix, []byte(addr), vbs)
	if err != nil {
		return "", err
	}
	db.mu.Lock()
	db.cache[addr] = acc
	db.mu.Unlock()
	return addr, nil
}

//导入私钥 pw != ""添加密码
func (db *LevelDBWallet) ImportAccount(ss string, pw string) error {
	acc, err := LoadAccount(ss)
	if err != nil {
		return err
	}
	addr, err := acc.GetAddress()
	if err != nil {
		return err
	}
	dump, err := acc.Dump()
	if err != nil {
		return err
	}
	if pw == "" {
		vbs := append([]byte{}, StdPKPrefix...)
		vbs = append(vbs, []byte(dump)...)
		return db.dptr.Put(AddrPrefix, []byte(addr), vbs)
	} else {
		block, err := NewAESCipher([]byte(pw))
		if err != nil {
			return err
		}
		data, err := AesEncrypt(block, []byte(dump))
		if err != nil {
			return fmt.Errorf("password error %w", err)
		}
		vbs := append([]byte{}, EncPKPrefix...)
		vbs = append(vbs, data...)
		return db.dptr.Put(AddrPrefix, []byte(addr), vbs)
	}
}

//解密一段时间，时间到达后私钥失效
func (db *LevelDBWallet) Decryption(addr string, pw string, st time.Duration) error {
	vbs, err := db.dptr.Get(AddrPrefix, []byte(addr))
	if err != nil {
		return err
	}
	//未被加密
	if vbs[0] == StdPKPrefix[0] {
		return errors.New("address not crypt")
	}
	block, err := NewAESCipher([]byte(pw))
	if err != nil {
		return err
	}
	data, err := AesDecrypt(block, vbs[1:])
	if err != nil {
		return fmt.Errorf("password error %w", err)
	}
	acc, err := LoadAccount(string(data))
	if err != nil {
		return err
	}
	db.mu.Lock()
	db.cache[addr] = acc
	db.times[addr] = time.Now().Add(st)
	db.mu.Unlock()
	return nil
}

//加密地址私钥
func (db *LevelDBWallet) Encryption(addr string, pw string) error {
	vbs, err := db.dptr.Get(AddrPrefix, []byte(addr))
	if err != nil {
		return err
	}
	//存在已经被加密
	if vbs[0] == EncPKPrefix[0] {
		return nil
	}
	block, err := NewAESCipher([]byte(pw))
	if err != nil {
		return err
	}
	data, err := AesEncrypt(block, vbs[1:])
	if err != nil {
		return err
	}
	vbs = append([]byte{}, EncPKPrefix...)
	vbs = append(vbs, data...)
	err = db.dptr.Put(AddrPrefix, []byte(addr), vbs)
	if err != nil {
		return err
	}
	db.mu.Lock()
	delete(db.cache, addr)
	delete(db.times, addr)
	db.mu.Unlock()
	return nil
}

//根据钱包地址获取私钥
func (db *LevelDBWallet) GetAccount(addr string) (*Account, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	//过期删除
	if pt, has := db.times[addr]; has {
		if time.Now().Sub(pt) > 0 {
			delete(db.cache, addr)
			delete(db.times, addr)
		}
	}
	//从缓存获取
	if pri, has := db.cache[addr]; has {
		return pri, nil
	}
	vbs, err := db.dptr.Get(AddrPrefix, []byte(addr))
	if err != nil {
		return nil, err
	}
	if vbs[0] != StdPKPrefix[0] {
		return nil, errors.New("std address error,addr encryption")
	}
	acc, err := LoadAccount(string(vbs[1:]))
	if err != nil {
		return nil, err
	}
	db.cache[addr] = acc
	return acc, nil
}

func (db *LevelDBWallet) checkTimer() {
	timer := time.NewTimer(time.Second * 5)
	for {
		select {
		case <-timer.C:
			//定时删除过期的私钥
			db.mu.Lock()
			dkeys := []string{}
			for k, v := range db.times {
				if time.Now().Sub(v) < 0 {
					continue
				}
				dkeys = append(dkeys, k)
			}
			if len(dkeys) > 0 {
				log.Println("delete expire keys", len(dkeys))
			}
			for _, k := range dkeys {
				delete(db.cache, k)
				delete(db.times, k)
			}
			db.mu.Unlock()
			timer.Reset(time.Second * 5)
		case <-db.done:
			return
		}
	}
}

//关闭钱包
func (db *LevelDBWallet) Close() {
	db.done <- true
	db.dptr.Close()
}

func NewLevelDBWallet(dir string) (IWallet, error) {
	ss := &LevelDBWallet{dir: dir}
	opts := &opt.Options{
		Filter: filter.NewBloomFilter(4),
	}
	sdb, err := leveldb.OpenFile(ss.dir, opts)
	if err != nil {
		return nil, err
	}
	ss.cache = map[string]*Account{}
	ss.times = map[string]time.Time{}
	ss.dptr = NewDB(sdb)
	ss.done = make(chan bool, 1)
	go ss.checkTimer()
	return ss, nil
}
