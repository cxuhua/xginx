package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const (
	SCRIPT_BASE_TYPE      = uint8(0) //coinbase input 0
	SCRIPT_STDUNLOCK_TYPE = uint8(1) //标准解锁脚本 pubkey sigvalue
	SCRIPT_STDLOCKED_TYPE = uint8(2) //标准锁定脚本 HASH160(pubkey)
	SCRIPT_AUCLOCKED_TYPE = uint8(3) //竞价锁定脚本AucLockScript
	SCRIPT_AUCUNLOCK_TYPE = uint8(4) //竞价解锁脚本AucUnlockScript
	SCRIPT_ARBLOCKED_TYPE = uint8(5) //三方仲裁脚本（买方，卖方，第三方) 解锁需要仲裁+买方 或者 仲裁+卖方
	SCRIPT_ARBUNLOCK_TYPE = uint8(5)
)

type Script []byte

func (s Script) Len() int {
	return len(s)
}

func (s Script) Type() uint8 {
	return s[0]
}

func (s Script) IsStdUnlockScript() bool {
	return s.Len() > 1 && s.Len() < 128 && s[0] == SCRIPT_STDUNLOCK_TYPE
}

func (s Script) StdLockedHash() HASH160 {
	if !s.IsStdLockedcript() {
		panic(errors.New("type error"))
	}
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

//out
func (s Script) IsArbLockScript() bool {
	return s.Len() > 1 && s.Len() < 128 && s[0] == SCRIPT_ARBLOCKED_TYPE
}

//out
func (s Script) IsAucLockScript() bool {
	return s.Len() > 1 && s.Len() < 128 && s[0] == SCRIPT_AUCLOCKED_TYPE
}

//in
func (s Script) IsAucUnlockScript() bool {
	return s.Len() > 1 && s.Len() < 256 && s[0] == SCRIPT_AUCUNLOCK_TYPE
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
func BaseScript(h uint32, bs ...[]byte) Script {
	hb := []byte{0, 0, 0, 0}
	Endian.PutUint32(hb, h)
	s := Script{SCRIPT_BASE_TYPE}
	s = append(s, hb...)
	for _, v := range bs {
		s = append(s, v...)
	}
	return s
}

type StdUnlockScript struct {
	Type uint8
	Pks  PKBytes  //物品公钥 hash160=objId
	Sig  SigBytes //签名
}

func (ss StdUnlockScript) Encode(w IWriter) error {
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

func (ss *StdUnlockScript) Decode(r IReader) error {
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

func NewStdUnlockScript(pub *PublicKey, sig *SigValue) (Script, error) {
	std := &StdUnlockScript{}
	std.Type = SCRIPT_STDUNLOCK_TYPE
	std.Pks.Set(pub)
	std.Sig.Set(sig)
	buf := &bytes.Buffer{}
	if err := std.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func StdLockedScript(v interface{}) Script {
	var hash HASH160
	s := Script{SCRIPT_STDLOCKED_TYPE}
	switch v.(type) {
	case *PublicKey:
		pub := v.(*PublicKey)
		hash = pub.Hash()
	case HASH160:
		hash = v.(HASH160)
	case PKBytes:
		pks := v.(PKBytes)
		hash = Hash160From(pks[:])
	case string:
		pub, err := LoadPublicKey(v.(string))
		if err != nil {
			panic(err)
		}
		hash = pub.Hash()
	default:
		panic(errors.New("v args type error"))
	}
	s = append(s, hash[:]...)
	return s
}
