package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"

	"github.com/willf/bloom"
)

var (
	//系统路径分隔符
	Separator = string(os.PathSeparator)
	//主链存储器
	store IStore = nil
)

type IDataStore interface {
	Read(st FileState) ([]byte, error)
	Write(b []byte) (FileState, error)
	Close()
	Init()
	Sync(id ...uint32)
}

type IStore interface {
	//关闭数据库
	Close()
	//初始化
	Init(arg ...interface{})
	//索引数据库
	Index() DBImp
	//区块状态数据库
	State() DBImp
	//标签数据库
	Tags() DBImp
	//区块数据文件
	Blk() IDataStore
	//事物回退文件
	Rev() IDataStore
	//获取存储的最高块信息
	GetBestValue() BestValue
	//设置标签计数器
	SetTagCtr(id TagUID, nv uint32) error
	//加载所有标签，并设置到过滤器
	LoadAllTags(bf *bloom.BloomFilter)
	//验证是否有验证成功的单元hash
	HasUnitash(id HASH256) (HASH160, error)
	//打包确认后可移除单元hash
	DelUnitHash(id HASH256) error
	//添加一个验证成功的单元hash
	PutUnitHash(id HASH256, cli PKBytes) error
	//保存标签信息
	SaveTag(tag *TTagInfo) error
	//获取标签信息
	LoadTagInfo(id TagUID) (*TTagInfo, error)
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

type DBImp interface {
	Has(ks ...[]byte) bool
	Put(ks ...[]byte) error
	Get(ks ...[]byte) ([]byte, error)
	Del(ks ...[]byte) error
	Write(b *Batch) error
	Close()
	Iterator(slice ...*Range) *Iterator
}

var (
	CTR_PREFIX   = []byte{'C'} //标签计数器前缀 TagDB()
	TAG_PREFIX   = []byte{'T'} //标签信息前缀 TagDB()
	BLOCK_PREFIX = []byte{'B'} //块头信息前缀 IndexDB()
	HUNIT_PREFIX = []byte{'H'} //单元hash，签名hash存在说明数据验证通过签名 TagDB()
	UXS_PREFIX   = []byte{'U'} //uts 所在区块前缀 数据为区块id+（uts索引+uv索引) StateDB()存储
	TXS_PREFIX   = []byte{'T'} //tx 所在区块前缀 数据为区块id+（txs索引 StateDB()存储
	CBI_PREFIX   = []byte{'C'} //用户最后单元块id StateDB()存储
)

var (
	BestBlockKey = []byte("BestBlockKey") //StateDB 保存
	InvalidBest  = NewInvalidBest()
)

type BestValue struct {
	Id     HASH256
	Height uint32
}

func NewInvalidBest() BestValue {
	return BestValue{
		Height: InvalidHeight,
	}
}

func (v BestValue) IsValid() bool {
	return v.Height != InvalidHeight
}

func (v BestValue) Bytes() []byte {
	buf := &bytes.Buffer{}
	buf.Write(v.Id[:])
	_ = binary.Write(buf, Endian, v.Height)
	return buf.Bytes()
}

func (v *BestValue) From(b []byte) error {
	buf := bytes.NewReader(b)
	if _, err := buf.Read(v.Id[:]); err != nil {
		return err
	}
	if err := binary.Read(buf, Endian, &v.Height); err != nil {
		return err
	}
	return nil
}

func GetDBKey(p []byte, id ...[]byte) []byte {
	tk := []byte{}
	tk = append(tk, p...)
	for _, v := range id {
		tk = append(tk, v...)
	}
	return tk
}

//标签数据
type TTagKey [16]byte

//保存数据库中的结构
type TTagInfo struct {
	UID  TagUID     //uid
	Ver  uint32     //版本 from tag
	Loc  Location   //uint32-uint32 位置 from tag
	ASV  uint8      //分配比例
	PKH  HASH160    //所属公钥HASH160
	Keys [5]TTagKey //ntag424 5keys
}

func (t *TTagInfo) Decode(r IReader) error {
	if err := binary.Read(r, Endian, t.UID[:]); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &t.Ver); err != nil {
		return err
	}
	if err := t.Loc.Decode(r); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &t.ASV); err != nil {
		return err
	}
	if err := t.PKH.Decode(r); err != nil {
		return err
	}
	for i, _ := range t.Keys {
		err := binary.Read(r, Endian, &t.Keys[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (t TTagInfo) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, t.UID[:]); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, t.Ver); err != nil {
		return err
	}
	if err := t.Loc.Encode(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, t.ASV); err != nil {
		return err
	}
	if err := t.PKH.Encode(w); err != nil {
		return err
	}
	for _, v := range t.Keys {
		err := binary.Write(w, Endian, v[:])
		if err != nil {
			return err
		}
	}
	return nil
}

func (tag TTagInfo) Mackey() []byte {
	idx := (tag.Ver >> 28) & 0xF
	return tag.Keys[idx][:]
}

func (tag *TTagInfo) SetMacKey(idx int) {
	if idx < 0 || idx >= len(tag.Keys) {
		panic(errors.New("idx out bound"))
	}
	tag.Ver |= uint32((idx & 0xf) << 28)
}
