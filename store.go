package xginx

import (
	"errors"
	"fmt"
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
	COINS_PREFIX = []byte{3} //账户可用金额存储 pkh_txid_idx -> amount
	TXP_PREFIX   = []byte{4} //账户相关交易索引  pkh_txid -> blkid+txidx
	REFTX_PREFIX = []byte{5} //存放交易池中的交易引用的其他交易，只在交易池使用
)

//金额状态
type CoinsState struct {
	Pool    Amount //内存中可用的
	Index   Amount //db记录的
	Matured Amount //未成熟的
	Sum     Amount //总和
}

func (s CoinsState) String() string {
	return fmt.Sprintf("pool = %d,index = %d, matured = %d, sum = %d", s.Pool, s.Index, s.Matured, s.Sum)
}

//金额记录
type Coins []*CoinKeyValue

//假设当前消费高度为 spent 获取金额状态
func (c Coins) State(spent uint32) CoinsState {
	s := CoinsState{}
	for _, v := range c {
		if v.IsMatured(spent) {
			s.Matured += v.Value
		} else if v.pool {
			s.Pool += v.Value
		} else {
			s.Index += v.Value
		}
		s.Sum += v.Value
	}
	return s
}

//获取总金额
func (c Coins) Balance() Amount {
	a := Amount(0)
	for _, v := range c {
		a += v.Value
	}
	return a
}

//积分key
type CoinKeyValue struct {
	CPkh     HASH160 //cli hash
	TxId     HASH256 //tx id
	Index    VarUInt //txout idx
	Value    Amount  //list时设置不包含在key中
	Coinbase uint8   //是否属于coinbase o or 1
	Height   VarUInt //所在区块高度
	pool     bool    //是否来自内存池
	spent    bool    //是否在内存池被消费了
}

func (tk *CoinKeyValue) From(k []byte, v []byte) error {
	buf := NewReader(k)
	//解析key
	cp, err := buf.ReadByte()
	if err != nil || cp != COINS_PREFIX[0] {
		return fmt.Errorf("conins prefix error %w", err)
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
	//解析value
	buf = NewReader(v)
	if err := tk.Value.Decode(buf); err != nil {
		return err
	}
	if err := buf.TRead(&tk.Coinbase); err != nil {
		return err
	}
	if err := tk.Height.Decode(buf); err != nil {
		return err
	}
	return nil
}

//创建一个消费输入
func (tk CoinKeyValue) NewTxIn(acc *Account) (*TxIn, error) {
	in := NewTxIn()
	in.OutHash = tk.TxId
	in.OutIndex = tk.Index
	if script, err := acc.NewWitnessScript().ToScript(); err != nil {
		return nil, err
	} else {
		in.Script = script
	}
	return in, nil
}

func (tk CoinKeyValue) GetValue() []byte {
	buf := NewWriter()
	if err := tk.Value.Encode(buf); err != nil {
		panic(err)
	}
	if err := buf.TWrite(tk.Coinbase); err != nil {
		panic(err)
	}
	if err := tk.Height.Encode(buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

//是否成熟可用
//内存中的，非coinbase直接可用
func (tk CoinKeyValue) IsMatured(spent uint32) bool {
	return tk.pool || tk.Coinbase == 0 || spent-tk.Height.ToUInt32() >= COINBASE_MATURITY
}

//消费key,用来记录输入对应的输出是否已经别消费
func (tk CoinKeyValue) SpentKey() []byte {
	buf := NewWriter()
	err := buf.WriteFull(COINS_PREFIX)
	if err != nil {
		panic(err)
	}
	err = tk.TxId.Encode(buf)
	if err != nil {
		panic(err)
	}
	err = tk.Index.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

//用来存储pkh拥有的可消费的金额
func (tk CoinKeyValue) GetKey() []byte {
	buf := NewWriter()
	err := buf.WriteFull(COINS_PREFIX)
	if err != nil {
		panic(err)
	}
	err = tk.CPkh.Encode(buf)
	if err != nil {
		panic(err)
	}
	err = tk.TxId.Encode(buf)
	if err != nil {
		panic(err)
	}
	err = tk.Index.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

var (
	BestBlockKey = []byte("BestBlockKey") //最高区块数据保存key
	InvalidBest  = NewInvalidBest()       //无效的状态
)

type BestValue struct {
	Id     HASH256
	Height uint32
}

//获取当前高度
func (bv BestValue) Curr() uint32 {
	if !bv.IsValid() {
		return 0
	} else {
		return bv.Height
	}
}

func (bv BestValue) LastID() HASH256 {
	if bv.IsValid() {
		return bv.Id
	} else {
		return conf.genesis
	}
}

//获取下一个高度
func (bv BestValue) Next() uint32 {
	return NextHeight(bv.Height)
}

func NextHeight(h uint32) uint32 {
	if h == InvalidHeight {
		return 0
	} else {
		return h + 1
	}
}

func NewInvalidBest() BestValue {
	return BestValue{
		Id:     ZERO256,
		Height: InvalidHeight,
	}
}

func BestValueBytes(id HASH256, h uint32) []byte {
	v := &BestValue{
		Id:     id,
		Height: h,
	}
	return v.Bytes()
}

func (v BestValue) IsValid() bool {
	return v.Height != InvalidHeight
}

func (v BestValue) Bytes() []byte {
	w := NewWriter()
	err := v.Id.Encode(w)
	if err != nil {
		panic(err)
	}
	err = w.TWrite(v.Height)
	if err != nil {
		panic(err)
	}
	return w.Bytes()
}

func (v *BestValue) From(b []byte) error {
	r := NewReader(b)
	if err := v.Id.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&v.Height); err != nil {
		return err
	}
	return nil
}

func GetDBKey(p []byte, ids ...[]byte) []byte {
	tk := []byte{}
	tk = append(tk, p...)
	for _, v := range ids {
		tk = append(tk, v...)
	}
	return tk
}
