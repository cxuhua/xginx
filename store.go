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
	TagStore ITagStore = nil
)

//数据块存储
type IChunkStore interface {
	Read(st FileState) ([]byte, error)
	Write(b []byte) (FileState, error)
	Close()
	Init() error
	Sync(id ...uint32)
}

//标签库存储
type ITagStore interface {
	//同步数据
	Sync()
	//关闭数据库
	Close()
	//初始化
	Init(arg ...interface{})
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

//事物接口
type TRImp interface {
	Has(ks ...[]byte) (bool, error)
	Put(ks ...[]byte) error
	Get(ks ...[]byte) ([]byte, error)
	Del(ks ...[]byte) error
	Write(b *Batch) error
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
	Write(b *Batch) error
	Close()
	Iterator(slice ...*Range) *Iterator
	Sync()
	Transaction() (TRImp, error)
}

var (
	CTR_PREFIX   = []byte{1} //标签计数器前缀 TagDB()
	TAG_PREFIX   = []byte{2} //标签信息前缀 TagDB()
	BLOCK_PREFIX = []byte{3} //块头信息前缀 IndexDB()
	HUNIT_PREFIX = []byte{4} //单元hash，签名hash存在说明数据验证通过签名 TagDB()
	UXS_PREFIX   = []byte{5} //uts 所在区块前缀 数据为区块id+（uts索引+uv索引) StateDB()存储
	TXS_PREFIX   = []byte{6} //tx 所在区块前缀 数据为区块id+（txs索引 StateDB()存储
	CBI_PREFIX   = []byte{7} //用户最后单元块id StateDB()存储
	TOKEN_PREFIX = []byte{8} //积分相关存储 StateDB pkh_txid_idx
)

//积分key
type TokenKeyValue struct {
	CPkh  HASH160 //cli hash
	TxId  HASH256 //tx id
	Index VarUInt //txout idx
	Value VarUInt //list时设置不包含在key中
}

func (tk *TokenKeyValue) From(k []byte, v []byte) error {
	buf := bytes.NewReader(k)
	pf := []byte{0}
	if _, err := buf.Read(pf); err != nil {
		return err
	}
	if !bytes.Equal(pf, TOKEN_PREFIX) {
		return errors.New("key prefix error")
	}
	if err := tk.CPkh.Decode(buf); err != nil {
		return err
	}
	if err := tk.TxId.Decode(buf); err != nil {
		return err
	}
	if err := tk.Index.Decode(buf); err != nil {
		return err
	}
	tk.Value.From(v)
	return nil
}

func (tk TokenKeyValue) GetTxIn() *TxIn {
	in := &TxIn{}
	in.OutHash = tk.TxId
	in.OutIndex = tk.Index
	return in
}

func (tk TokenKeyValue) GetValue() []byte {
	return tk.Value.Bytes()
}

func (tk TokenKeyValue) GetKey() []byte {
	buf := &bytes.Buffer{}
	_, _ = buf.Write(TOKEN_PREFIX)
	_ = tk.CPkh.Encode(buf)
	_ = tk.TxId.Encode(buf)
	_ = tk.Index.Encode(buf)
	return buf.Bytes()
}

var (
	BestBlockKey = []byte("BestBlockKey") //StateDB 保存
	InvalidBest  = NewInvalidBest()       //无效的状态
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
