package xginx

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	dbptr *leveldb.DB = nil
	once  sync.Once
)

const (
	DB_TAG_PREFIX = 't' //tag sign info
	DB_KEY_PREFIX = 'k' //保存标签5个密钥
)

type TagKey [16]byte

func (k *TagKey) Rand() {
	_, _ = rand.Read(k[:])
}

type TTagKeys struct {
	ID   TagUID    //key index
	TVer uint32    //版本
	TLoc Location  //位置
	TPKS string    //标签私钥编码保存
	Keys [5]TagKey //5个AES密钥
}

func NewTagKey(id TagUID) []byte {
	key := make([]byte, len(id)+1)
	key[0] = DB_KEY_PREFIX
	copy(key[1:], id[:])
	return key
}

func ReadString(s io.Reader) (string, error) {
	l := uint8(0)
	if err := binary.Read(s, Endian, &l); err != nil {
		return "", err
	}
	b := make([]byte, l)
	if err := binary.Read(s, Endian, b); err != nil {
		return "", err
	}
	return string(b), nil
}

func LoadTagKeys(id TagUID) (*TTagKeys, error) {
	kv := &TTagKeys{}
	key := NewTagKey(id)
	b, err := DB().Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("load tag keys error %w", err)
	}
	buf := bytes.NewBuffer(b)
	if err := binary.Read(buf, Endian, &kv.TVer); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, Endian, &kv.TLoc); err != nil {
		return nil, err
	}
	if s, err := ReadString(buf); err != nil {
		return nil, err
	} else {
		kv.TPKS = s
	}
	if err := binary.Read(buf, Endian, &kv.Keys); err != nil {
		return nil, err
	}
	kv.ID = id
	return kv, nil
}

func (k TTagKeys) Save(id TagUID) error {
	key := NewTagKey(id)
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, Endian, k.TVer); err != nil {
		return err
	}
	if err := binary.Write(buf, Endian, k.TLoc); err != nil {
		return err
	}
	if err := binary.Write(buf, Endian, uint8(len(k.TPKS))); err != nil {
		return err
	}
	if err := binary.Write(buf, Endian, []byte(k.TPKS)); err != nil {
		return err
	}
	if err := binary.Write(buf, Endian, k.Keys); err != nil {
		return err
	}
	return DB().Put(key, buf.Bytes(), nil)
}

type TTagInfo struct {
	ID   TagUID  //key index
	CCTR TagCTR  //last ctr count
	CPKS PKBytes //last sign cpks
	Time int64   //last sign time
}

func NewInfoKey(id TagUID) []byte {
	key := make([]byte, len(id)+1)
	key[0] = DB_TAG_PREFIX
	copy(key[1:], id[:])
	return key
}

func LoadTagInfo(id TagUID) (*TTagInfo, error) {
	kv := &TTagInfo{}
	key := NewInfoKey(id)
	b, err := DB().Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("load tag keys error %w", err)
	}
	buf := bytes.NewBuffer(b)
	if err := binary.Read(buf, Endian, &kv.CCTR); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, Endian, &kv.CPKS); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, Endian, &kv.Time); err != nil {
		return nil, err
	}
	kv.ID = id
	return kv, nil
}

func (k TTagInfo) Save(id TagUID) error {
	key := NewInfoKey(id)
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, Endian, k.CCTR); err != nil {
		return err
	}
	if err := binary.Write(buf, Endian, k.CPKS); err != nil {
		return err
	}
	if err := binary.Write(buf, Endian, k.Time); err != nil {
		return err
	}
	return DB().Put(key, buf.Bytes(), nil)
}

func DB() *leveldb.DB {
	once.Do(func() {
		bf := filter.NewBloomFilter(5)
		opts := &opt.Options{
			Filter: bf,
		}
		sdb, err := leveldb.OpenFile("datadir", opts)
		if err != nil {
			panic(err)
		}
		dbptr = sdb
	})
	return dbptr
}
