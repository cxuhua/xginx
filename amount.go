package xginx

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

//金额定义
const (
	MaxCompressUInt = uint64(0b1111 << 57)
	Coin            = Amount(1000)
	// MaxMoney < MaxCompressUInt
	MaxMoney = 21000000 * Coin
)

var (
	//1个coin可分割为1000份
	CoinSplit = decimal.NewFromInt(int64(Coin))
)

//GetCoinbaseReward 结算当前奖励
func GetCoinbaseReward(h uint32) Amount {
	hlv := int(h) / conf.Halving
	if hlv < 0 || hlv >= 64 {
		return 0
	}
	n := 50 * Coin
	n >>= hlv
	return n
}

//Amount 金额类型
type Amount int64

//Decode 解码金额，并解压缩
func (a *Amount) Decode(r IReader) error {
	cv, err := binary.ReadUvarint(r)
	if err != nil {
		return err
	}
	vv := DecompressUInt(cv)
	*a = Amount(vv)
	if !(*a).IsRange() {
		return errors.New("amount range error")
	}
	return nil
}

//ParseMoney 解析金额
func ParseMoney(str string) (Amount, error) {
	num, err := decimal.NewFromString(str)
	if err != nil {
		return 0, err
	}
	num = num.Mul(CoinSplit)
	amt := Amount(num.IntPart())
	if !amt.IsRange() {
		return 0, fmt.Errorf("%d out max amount", amt)
	}
	return amt, nil
}

func (a Amount) String() string {
	num := decimal.NewFromInt(int64(a))
	num = num.Div(CoinSplit)
	return num.String()
}

//Bytes 压缩金额并生成二进制数据
func (a Amount) Bytes() []byte {
	if !a.IsRange() {
		panic(errors.New("amount range error"))
	}
	cv := CompressUInt(uint64(a))
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutUvarint(lb, cv)
	return lb[:l]
}

//From 从二进制生成金额
func (a *Amount) From(b []byte) Amount {
	cv, _ := binary.Uvarint(b)
	vv := DecompressUInt(cv)
	*a = Amount(vv)
	return *a
}

//Encode 编码并压缩金额
func (a Amount) Encode(w IWriter) error {
	if !a.IsRange() {
		return errors.New("amount range error")
	}
	cv := CompressUInt(uint64(a))
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutUvarint(lb, cv)
	wb := lb[:l]
	return w.TWrite(wb)
}

//IsRange 检测金额是否在有效范围内
func (a Amount) IsRange() bool {
	return a >= 0 && a <= MaxMoney
}

//CompressUInt 压缩一个整形 max : 60 bits
func CompressUInt(n uint64) uint64 {
	if n > MaxCompressUInt {
		panic(ErrVarSize)
	}
	if n == 0 {
		return 0
	}
	e := uint64(0)
	for ((n % 10) == 0) && e < 9 {
		n /= 10
		e++
	}
	if e < 9 {
		d := (n % 10)
		n /= 10
		n = 1 + (n*9+d-1)*10 + e
	} else {
		n = 1 + (n-1)*10 + 9
	}
	return n
}

//DecompressUInt 解压整形max : 60 bits
func DecompressUInt(x uint64) uint64 {
	if x == 0 {
		return 0
	}
	x--
	e := x % 10
	x /= 10
	n := uint64(0)
	if e < 9 {
		d := (x % 9) + 1
		x /= 9
		n = x*10 + d
	} else {
		n = x + 1
	}
	for e != 0 {
		n *= 10
		e--
	}
	if n > MaxCompressUInt {
		panic(ErrVarSize)
	}
	return n
}
