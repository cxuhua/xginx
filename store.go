package xginx

import (
	"errors"
	"os"
)

//系统路径分隔符
var (
	Separator = string(os.PathSeparator)
)

//IChunkStore 数据块存储
type IChunkStore interface {
	Read(st BlkChunk) ([]byte, error)
	Write(b []byte) (BlkChunk, error)
	Close()
	Init() error
	Sync(id ...uint32)
}

//IBlkStore 区块存储
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

//TRImp 事务接口
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

//DBImp 数据基本操作接口
type DBImp interface {
	Has(ks ...[]byte) bool              //key是否存在
	Put(ks ...[]byte) error             //添加键值
	Get(ks ...[]byte) ([]byte, error)   //根据key获取值
	Del(ks ...[]byte) error             //删除key
	Write(b *Batch, sync ...bool) error //批量写
	Compact(r *Range) error             //合并
	Close()                             //关闭数据库
	Iterator(slice ...*Range) *Iterator //搜索
	Sync()                              //同步到磁盘
	Transaction() (TRImp, error)        //创建事务
	NewBatch() *Batch                   //创建批量
	LoadBatch(d []byte) (*Batch, error) //加载批量数据
}

//数据前缀定义
var (
	BlockPrefix = []byte{1} //块头信息前缀 ->blkmeta
	TxsPrefix   = []byte{2} //tx 所在区块前缀 ->blkid+txidx
	CoinsPrefix = []byte{3} //账户可用金额存储 pkh_txid_idx -> amount
	TxpPrefix   = []byte{4} //账户相关交易索引 按高度排序  pkh_height(big endian)_txid -> blkid+txidx
)

//GetDBKey 获取存储key
func GetDBKey(p []byte, ids ...[]byte) []byte {
	tk := append([]byte{}, p...)
	for _, id := range ids {
		tk = append(tk, id...)
	}
	return tk
}
