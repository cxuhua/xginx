package xginx

import (
	"errors"
	"os"
)

var (
	//系统路径分隔符
	Separator = string(os.PathSeparator)
)

//数据块存储
type IChunkStore interface {
	Read(st BlkChunk) ([]byte, error)
	Write(b []byte) (BlkChunk, error)
	Close()
	Init() error
	Sync(id ...uint32)
}

//区块存储
type IBlkStore interface {
	//同步数据
	Sync()
	//关闭数据库
	Close()
	//初始化
	Init(arg ...interface{})
	//索引数据库
	Index() DBImp
	//区块数据文件
	Blk() IChunkStore
	//事物回退文件
	Rev() IChunkStore
}

func getDBKeyValue(ks ...[]byte) ([]byte, []byte) {
	var k []byte
	var v []byte
	l := len(ks)
	if l < 2 {
		panic(errors.New("args ks num error"))
	} else if l == 2 {
		k = ks[0]
		v = ks[1]
	} else if l > 2 {
		k = GetDBKey(ks[0], ks[1:l-1]...)
		v = ks[l-1]
	}
	return k, v
}

func getDBKey(ks ...[]byte) []byte {
	var k []byte
	l := len(ks)
	if l < 1 {
		panic(errors.New("args ks num error"))
	} else if l == 1 {
		k = ks[0]
	} else if l > 1 {
		k = GetDBKey(ks[0], ks[1:]...)
	}
	return k
}

//事务接口
type TRImp interface {
	Has(ks ...[]byte) (bool, error)
	Put(ks ...[]byte) error
	Get(ks ...[]byte) ([]byte, error)
	Del(ks ...[]byte) error
	Write(b *Batch, sync ...bool) error
	Iterator(slice ...*Range) *Iterator
	Commit() error
	Discard()
}

//数据基本操作接口
type DBImp interface {
	Has(ks ...[]byte) bool
	Put(ks ...[]byte) error
	Get(ks ...[]byte) ([]byte, error)
	Del(ks ...[]byte) error
	Write(b *Batch, sync ...bool) error
	Compact(r *Range) error
	Close()
	Iterator(slice ...*Range) *Iterator
	Sync()
	Transaction() (TRImp, error)
	NewBatch() *Batch
	LoadBatch(d []byte) (*Batch, error)
}

var (
	BLOCK_PREFIX = []byte{1} //块头信息前缀 ->blkmeta
	TXS_PREFIX   = []byte{2} //tx 所在区块前缀 ->blkid+txidx
	COINS_PREFIX = []byte{3} //账户可用金额存储 pkh_txid_idx -> amount
	TXP_PREFIX   = []byte{4} //账户相关交易索引  pkh_txid -> blkid+txidx
	REFTX_PREFIX = []byte{5} //存放交易池中的交易引用的其他交易，只在交易池使用
)

func GetDBKey(p []byte, ids ...[]byte) []byte {
	tk := []byte{}
	tk = append(tk, p...)
	for _, v := range ids {
		tk = append(tk, v...)
	}
	return tk
}
