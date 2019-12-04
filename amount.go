package xginx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

// MAX_MONEY < MAX_COMPRESS_UINT
const (
	MAX_COMPRESS_UINT = uint64(0b1111 << 57)
	COIN              = Amount(100_000_000)
	MAX_MONEY         = 21_000_000 * COIN
)

//结算当前奖励
func GetCoinbaseReward(h uint32) Amount {
	halvings := int(h) / conf.Halving
	if halvings >= 64 {
		return 0
	}
	n := 50 * COIN
	n >>= halvings
	return n
}

type Amount int64

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

//解析金额
func ParseMoney(str string) (Amount, error) {
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, err
	}
	a := Amount(f * float64(COIN))
	return a, nil
}

func (a Amount) String() string {
	abs := int64(a)
	if abs < 0 {
		abs = -abs
	}
	n := a / COIN
	x := a % COIN
	str := fmt.Sprintf("%d.%08d", n, x)
	trim := 0
	for i := len(str) - 1; str[i] == '0' && str[i-1] != '.'; i-- {
		trim++
	}
	if trim > 0 {
		str = str[:len(str)-trim]
	}
	if a < 0 {
		return "-" + str
	}
	return str
}

func (a Amount) Bytes() []byte {
	if !a.IsRange() {
		panic(errors.New("amount range error"))
	}
	cv := CompressUInt(uint64(a))
	lb := make([]byte, binary.MaxVarintLen64)
	l := binary.PutUvarint(lb, cv)
	return lb[:l]
}

func (v *Amount) From(b []byte) Amount {
	cv, _ := binary.Uvarint(b)
	vv := DecompressUInt(cv)
	*v = Amount(vv)
	return *v
}

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

func (a Amount) IsRange() bool {
	return a >= 0 && a <= MAX_MONEY
}

//max : 60 bits
func CompressUInt(n uint64) uint64 {
	if n > MAX_COMPRESS_UINT {
		panic(VarSizeErr)
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

//max : 60 bits
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
	if n > MAX_COMPRESS_UINT {
		panic(VarSizeErr)
	}
	return n
}
