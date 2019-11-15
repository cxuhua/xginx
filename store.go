package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
)

var (
	//系统路径分隔符
	Separator = string(os.PathSeparator)
)

//数据块存储
type IChunkStore interface {
	Read(st FileState) ([]byte, error)
	Write(b []byte) (FileState, error)
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
	COIN_PREFIX  = []byte{3} //积分相关存储 pkh_txid_idx -> amount
	EXT_PREFIX   = []byte{4} //扩展数据索引 extid -> blkid+txidx+extlen
)

type ExtKeyValue struct {
	BlkId  HASH256 //区块id
	TxIdx  VarUInt //交易索引
	ExtLen VarUInt //数据长度
}

func (ekv *ExtKeyValue) From(bb []byte) error {
	buf := bytes.NewReader(bb)
	err := ekv.BlkId.Decode(buf)
	if err != nil {
		return err
	}
	err = ekv.TxIdx.Decode(buf)
	if err != nil {
		return err
	}
	err = ekv.ExtLen.Decode(buf)
	if err != nil {
		return err
	}
	return nil
}

func (ekv ExtKeyValue) GetValue() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := ekv.BlkId.Encode(buf)
	if err != nil {
		return nil, err
	}
	err = ekv.TxIdx.Encode(buf)
	if err != nil {
		return nil, err
	}
	err = ekv.ExtLen.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//积分key
type CoinKeyValue struct {
	CPkh  HASH160 //cli hash
	TxId  HASH256 //tx id
	Index VarUInt //txout idx
	Value Amount  //list时设置不包含在key中
}

func (tk *CoinKeyValue) From(k []byte, v []byte) error {
	buf := bytes.NewReader(k)
	pf := []byte{0}
	if _, err := buf.Read(pf); err != nil {
		return err
	}
	if !bytes.Equal(pf, COIN_PREFIX) {
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

func (tk CoinKeyValue) GetTxIn() *TxIn {
	in := &TxIn{}
	in.OutHash = tk.TxId
	in.OutIndex = tk.Index
	in.Script = EmptyWitnessScript()
	return in
}

func (tk CoinKeyValue) GetValue() []byte {
	return tk.Value.Bytes()
}

func (tk CoinKeyValue) GetKey() []byte {
	buf := &bytes.Buffer{}
	_, _ = buf.Write(COIN_PREFIX)
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
