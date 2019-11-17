package xginx

import (
	"encoding/binary"
	"errors"
)

const (
	COIN      = Amount(100000000)
	MAX_MONEY = 21000000 * COIN
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
