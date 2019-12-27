package xginx

import (
	"fmt"
	"sort"
)

//金额状态
type CoinsState struct {
	Locks Coins  //锁定的
	Coins Coins  //当前可用
	All   Coins  //所有
	Sum   Amount //总和
}

func (s *CoinsState) Merge(v *CoinsState) {
	s.Locks = append(s.Locks, v.Locks...)
	s.Coins = append(s.Coins, v.Coins...)
	s.All = append(s.All, v.All...)
	s.Sum += v.Sum
}

func (s CoinsState) String() string {
	return fmt.Sprintf("Locks = %d, coins = %d sum = %d", s.Locks.Balance(), s.Coins.Balance(), s.Sum)
}

//金额记录
type Coins []*CoinKeyValue

//假设当前消费高度为 spent 获取金额状态
func (c Coins) State(spent uint32) *CoinsState {
	s := &CoinsState{All: c}
	for _, v := range c {
		if !v.IsMatured(spent) {
			s.Locks = append(s.Locks, v)
		} else if v.pool {
			s.Coins = append(s.Coins, v)
		} else if spent-v.Height.ToUInt32() >= conf.Confirms {
			s.Coins = append(s.Coins, v)
		} else {
			s.Locks = append(s.Locks, v)
		}
		s.Sum += v.Value
	}
	return s
}

//按高度排序
func (c *Coins) Sort() {
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].Height < (*c)[j].Height
	})
}

//获取总金额
func (c Coins) Balance() Amount {
	a := Amount(0)
	for _, v := range c {
		a += v.Value
	}
	return a
}

//金额存储结构
type CoinKeyValue struct {
	CPkh   HASH160 //公钥hash
	TxId   HASH256 //交易id
	Index  VarUInt //输出索引
	Value  Amount  //输出金额
	Base   uint8   //是否属于coinbase o or 1
	Height VarUInt //所在区块高度
	pool   bool    //是否来自内存池
	spent  bool    //是否在内存池被消费了
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
	if err := buf.TRead(&tk.Base); err != nil {
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

func (tk CoinKeyValue) MustValue() []byte {
	buf := NewWriter()
	if err := tk.Value.Encode(buf); err != nil {
		panic(err)
	}
	if err := buf.TWrite(tk.Base); err != nil {
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
	return tk.pool || tk.Base == 0 || spent-tk.Height.ToUInt32() >= COINBASE_MATURITY
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

func (tk CoinKeyValue) GetAddress() Address {
	addr, err := EncodeAddress(tk.CPkh)
	if err != nil {
		panic(err)
	}
	return addr
}

//用来存储pkh拥有的可消费的金额
func (tk CoinKeyValue) MustKey() []byte {
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
