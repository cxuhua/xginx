package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

const (
	SCRIPT_BASE_TYPE      = uint8(0) //coinbase input 0
	SCRIPT_STDUNLOCK_TYPE = uint8(1) //标准解锁脚本 pubkey sigvalue
	SCRIPT_STDLOCKED_TYPE = uint8(2) //标准锁定脚本 HASH160(pubkey)
	SCRIPT_WITNESS_TYPE   = uint8(3) //witness type
)

type Script []byte

func (s Script) Len() int {
	return len(s)
}

func (s Script) Type() uint8 {
	return s[0]
}

func (s Script) IsStdUnlockScript() bool {
	return s.Len() > 1 && s.Len() < MAX_SCRIPT_SIZE && s[0] == SCRIPT_STDUNLOCK_TYPE
}

func (s Script) StdLockedHash() HASH160 {
	hash := HASH160{}
	copy(hash[:], s[1:])
	return hash
}

//out
func (s Script) IsStdLockedcript() bool {
	return s.Len() > 1 && s.Len() < 64 && s[0] == SCRIPT_STDLOCKED_TYPE
}

func (s Script) StdPKH() HASH160 {
	hash := HASH160{}
	copy(hash[:], s[1:])
	return hash
}

//in
func (s Script) IsBaseScript() bool {
	return s.Len() > 1 && s.Len() < 128 && s[0] == SCRIPT_BASE_TYPE
}

//获取coinbase中的区块高度
func (s Script) Height() uint32 {
	return Endian.Uint32(s[1:5])
}

func (s Script) Encode(w IWriter) error {
	return VarBytes(s).Encode(w)
}

func (s *Script) Decode(r IReader) error {
	return (*VarBytes)(s).Decode(r)
}

//加入区块高度
func GetCoinbaseScript(h uint32, bs ...[]byte) Script {
	s := Script{SCRIPT_BASE_TYPE}
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
type StdLockedScript struct {
	Type uint8
	Pkh  HASH160
}

func (ss StdLockedScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *StdLockedScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Decode(r); err != nil {
		return err
	}
	return nil
}

func StdLockedScriptFrom(s Script) (*StdLockedScript, error) {
	if !s.IsStdLockedcript() {
		return nil, errors.New("type error")
	}
	buf := bytes.NewReader(s)
	std := &StdLockedScript{}
	if err := std.Decode(buf); err != nil {
		return nil, err
	}
	if std.Type != SCRIPT_STDLOCKED_TYPE {
		return nil, errors.New("type error")
	}
	return std, nil
}

func NewStdLockedScript(v interface{}) (Script, error) {
	std := &StdLockedScript{}
	std.Type = SCRIPT_STDLOCKED_TYPE
	std.Pkh = NewHASH160(v)
	buf := &bytes.Buffer{}
	if err := std.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type WitnessScript struct {
	Type uint8    //SCRIPT_WITNESS_TYPE
	Pks  PKBytes  //物品公钥 hash160=objId
	Sig  SigBytes //签名
}

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

func (ss *WitnessScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.Pks.Decode(r); err != nil {
		return err
	}
	if err := ss.Sig.Decode(r); err != nil {
		return err
	}
	return nil
}

func NewWitnessScript(pub *PublicKey, sig *SigValue) WitnessScript {
	wit := WitnessScript{}
	wit.Type = SCRIPT_WITNESS_TYPE
	wit.Pks.Set(pub)
	wit.Sig.Set(sig)
	return wit
}

//标准解锁脚本
type StdUnlockScript struct {
	Type uint8
	Pkh  HASH160
}

func (ss StdUnlockScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *StdUnlockScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Decode(r); err != nil {
		return err
	}
	return nil
}

func StdUnlockScriptFrom(s Script) (*StdUnlockScript, error) {
	if !s.IsStdUnlockScript() {
		return nil, errors.New("type error")
	}
	buf := bytes.NewReader(s)
	std := &StdUnlockScript{}
	if err := std.Decode(buf); err != nil {
		return nil, err
	}
	if std.Type != SCRIPT_STDUNLOCK_TYPE {
		return nil, errors.New("type error")
	}
	return std, nil
}

func NewStdUnlockScript(pkh HASH160) Script {
	std := &StdUnlockScript{}
	std.Type = SCRIPT_STDUNLOCK_TYPE
	std.Pkh = pkh
	buf := &bytes.Buffer{}
	err := std.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}
