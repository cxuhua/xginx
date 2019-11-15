package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

const (
	SCRIPT_COINBASE_TYPE = uint8(0) //coinbase input 0
	SCRIPT_LOCKED_TYPE   = uint8(1) //标准锁定脚本 HASH160(pubkey)
	SCRIPT_WITNESS_TYPE  = uint8(2) //witness type
)

type Script []byte

func (s Script) Len() int {
	return len(s)
}

func (s Script) Type() uint8 {
	return s[0]
}

func getwitnesssize() int {
	x := WitnessScript{}
	x.Type = SCRIPT_WITNESS_TYPE
	buf := &bytes.Buffer{}
	_ = x.Encode(buf)
	return buf.Len()
}

func getcoinbaseminsize() int {
	return GetCoinbaseScript(0, []byte{}).Len()
}

func getlockedsize() int {
	x := LockedScript{}
	x.Type = SCRIPT_LOCKED_TYPE
	buf := &bytes.Buffer{}
	_ = x.Encode(buf)
	return buf.Len()
}

var (
	witnesssize    = getwitnesssize()
	lockedsize     = getlockedsize()
	conbaseminsize = getcoinbaseminsize()
)

//in script
func (s Script) IsCoinBase() bool {
	return s.Len() >= conbaseminsize && s.Len() <= 128 && s[0] == SCRIPT_COINBASE_TYPE
}

//in script
func (s Script) IsWitness() bool {
	return s.Len() == witnesssize && s[0] == SCRIPT_WITNESS_TYPE
}

//out script
func (s Script) IsLocked() bool {
	return s.Len() == lockedsize && s[0] == SCRIPT_LOCKED_TYPE
}

//coinbase交易没有pkh
func (s Script) GetPkh() HASH160 {
	pkh := HASH160{}
	copy(pkh[:], s[1:1+len(pkh)])
	return pkh
}

//获取coinbase中的区块高度
func (s Script) Height() uint32 {
	return Endian.Uint32(s[1:5])
}

func (s Script) Encode(w IWriter) error {
	return VarBytes(s).Encode(w)
}

func (s Script) ForID(w IWriter) error {
	if s.IsCoinBase() {
		return VarBytes(s).Encode(w)
	} else {
		return s.ToWitness().ForID(w)
	}
}

//签名，验证写入
func (s Script) ForVerify(w IWriter) error {
	return s.ToWitness().ForVerify(w)
}

func (s *Script) Decode(r IReader) error {
	return (*VarBytes)(s).Decode(r)
}

func (s Script) ToWitness() WitnessScript {
	if !s.IsWitness() {
		panic(errors.New("witness error"))
	}
	buf := bytes.NewReader(s)
	wit := WitnessScript{}
	err := wit.Decode(buf)
	if err != nil {
		panic(err)
	}
	return wit
}

func GetCoinbaseScript(h uint32, bs ...[]byte) Script {
	s := Script{SCRIPT_COINBASE_TYPE}
	hb := []byte{0, 0, 0, 0}
	//当前块高度
	Endian.PutUint32(hb, h)
	s = append(s, hb...)
	//加当前时间戳
	Endian.PutUint32(hb, uint32(time.Now().Unix()))
	s = append(s, hb...)
	//加点随机值
	rv := uint32(0)
	SetRandInt(&rv)
	Endian.PutUint32(hb, rv)
	s = append(s, hb...)
	//自定义数据
	for _, v := range bs {
		s = append(s, v...)
	}
	return s
}

//标准锁定脚本
type LockedScript struct {
	Type uint8
	Pkh  HASH160
}

func (ss LockedScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *LockedScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Decode(r); err != nil {
		return err
	}
	return nil
}

func NewLockedScript(v interface{}) (Script, error) {
	std := &LockedScript{}
	std.Type = SCRIPT_LOCKED_TYPE
	std.Pkh = NewHASH160(v)
	buf := &bytes.Buffer{}
	if err := std.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//隔离见证
type WitnessScript struct {
	Type uint8    //SCRIPT_WITNESS_TYPE
	Pks  PKBytes  //公钥
	Sig  SigBytes //签名
}

//id计算
func (ss WitnessScript) ForID(w IWriter) error {
	return binary.Write(w, Endian, ss.Type)
}

//编码需要签名的数据
func (ss WitnessScript) ForVerify(w IWriter) error {
	return binary.Write(w, Endian, ss.Type)
}

//编码
func (ss WitnessScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.Pks.Encode(w); err != nil {
		return err
	}
	if err := ss.Sig.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss WitnessScript) ToScript() Script {
	buf := &bytes.Buffer{}
	err := ss.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (ss *WitnessScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if ss.Type != SCRIPT_WITNESS_TYPE {
		return nil
	}
	if err := ss.Pks.Decode(r); err != nil {
		return err
	}
	if err := ss.Sig.Decode(r); err != nil {
		return err
	}
	return nil
}

func EmptyWitnessScript() Script {
	wit := WitnessScript{}
	wit.Type = SCRIPT_WITNESS_TYPE
	return wit.ToScript()
}

func NewWitnessScript(pub *PublicKey, sig *SigValue) WitnessScript {
	wit := WitnessScript{}
	wit.Type = SCRIPT_WITNESS_TYPE
	wit.Pks.Set(pub)
	wit.Sig.Set(sig)
	return wit
}
