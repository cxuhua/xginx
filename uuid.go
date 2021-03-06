package xginx

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

//第一个字节为分类字节,可分0-254类
type DocumentID [12]byte

var NilDocumentID DocumentID

var DocumentIDLen = len(NilDocumentID)

//是否是空的id
func (id DocumentID) IsNil() bool {
	return id.Equal(NilDocumentID)
}

func (id DocumentID) Encode(w IWriter) error {
	return w.WriteFull(id[:])
}

func (id *DocumentID) Decode(r IReader) error {
	return r.ReadFull(id[:])
}

func (id DocumentID) Equal(v DocumentID) bool {
	return bytes.Equal(id[:], v[:])
}

func NewDocumentIDFrom(b []byte) DocumentID {
	id := DocumentID{}
	copy(id[:], b)
	return id
}

func DocumentIDFromHex(s string) DocumentID {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	if len(b) != 12 {
		panic("length error")
	}
	var oid [12]byte
	copy(oid[:], b[:])
	return oid
}

//复制id并转换到指定类型
func (id DocumentID) To(typ byte) DocumentID {
	oid := id
	oid.SetType(typ)
	return oid
}

//获取类型字节
func (id DocumentID) Type() byte {
	return id[0]
}

//修改类型字节
func (id *DocumentID) SetType(typ byte) {
	id[0] = typ
}

func (id DocumentID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

func (id *DocumentID) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var err error
	switch len(b) {
	case 12:
		copy(id[:], b)
	default:
		var res interface{}
		err := json.Unmarshal(b, &res)
		if err != nil {
			return err
		}
		str, ok := res.(string)
		if !ok {
			m, ok := res.(map[string]interface{})
			if !ok {
				return errors.New("not an extended JSON ObjectID")
			}
			oid, ok := m["$oid"]
			if !ok {
				return errors.New("not an extended JSON ObjectID")
			}
			str, ok = oid.(string)
			if !ok {
				return errors.New("not an extended JSON ObjectID")
			}
		}
		if len(str) == 0 {
			copy(id[:], NilDocumentID[:])
			return nil
		}
		if len(str) != 24 {
			return fmt.Errorf("cannot unmarshal into an ObjectID, the length must be 24 but it is %d", len(str))
		}
		_, err = hex.Decode(id[:], []byte(str))
		if err != nil {
			return err
		}
	}
	return err
}

func (id DocumentID) String() string {
	return hex.EncodeToString(id[:])
}

func readRandomUint32() uint32 {
	var b [4]byte
	_, err := io.ReadFull(rand.Reader, b[:])
	if err != nil {
		panic(fmt.Errorf("cannot initialize objectid package with crypto.rand.Reader: %v", err))
	}

	return (uint32(b[0]) << 0) | (uint32(b[1]) << 8) | (uint32(b[2]) << 16) | (uint32(b[3]) << 24)
}

func processUniqueBytes() [4]byte {
	var b [4]byte
	_, err := io.ReadFull(rand.Reader, b[:])
	if err != nil {
		panic(fmt.Errorf("cannot initialize objectid package with crypto.rand.Reader: %v", err))
	}
	return b
}

func putUint24(b []byte, v uint32) {
	b[0] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[2] = byte(v)
}

var objectIDCounter = readRandomUint32()

var processUnique = processUniqueBytes()

func NewDocumentIDFromTimestamp(typ byte, timestamp time.Time) DocumentID {
	var b [12]byte
	b[0] = typ
	binary.BigEndian.PutUint32(b[1:5], uint32(timestamp.Unix()))
	copy(b[5:9], processUnique[:])
	putUint24(b[9:12], atomic.AddUint32(&objectIDCounter, 1))
	return b
}

func NewDocumentID(typ ...byte) DocumentID {
	t := byte(0)
	if len(typ) > 0 {
		t = typ[0]
	}
	return NewDocumentIDFromTimestamp(t, time.Now())
}
