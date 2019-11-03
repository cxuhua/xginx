package xginx

import "bytes"

const (
	SCRIPT_BASE_TYPE      = uint8(0) //coinbase input 0
	SCRIPT_STDUNLOCK_TYPE = uint8(1) //标准解锁脚本 pubkey sigvalue
	SCRIPT_STDLOCKED_TYPE = uint8(2) //标准锁定脚本 HASH160(pubkey)
	SCRIPT_AUCLOCK_TYPE   = uint8(3) //竞价锁定脚本AucLockScript
	SCRIPT_AUCUNLOCK_TYPE = uint8(4) //竞价解锁脚本AucUnlockScript
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

func (s Script) IsStdLockedcript() bool {
	return s.Len() > 1 && s.Len() < 64 && s[0] == SCRIPT_STDLOCKED_TYPE
}

func (s Script) IsBaseScript() bool {
	return s.Len() > 1 && s.Len() < 128 && s[0] == SCRIPT_BASE_TYPE
}

func (s Script) IsAucLockScript() bool {
	return s.Len() > 1 && s.Len() < 128 && s[0] == SCRIPT_AUCLOCK_TYPE
}

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
func BaseScript(h uint32, b []byte) Script {
	hb := []byte{0, 0, 0, 0}
	Endian.PutUint32(hb, h)
	s := Script{SCRIPT_BASE_TYPE}
	s = append(s, hb...)
	s = append(s, b...)
	return s
}

func UnlockScript(pub *PublicKey, sig *SigValue) Script {
	pb := pub.Encode()
	sb := sig.Encode()
	buf := &bytes.Buffer{}
	//type
	buf.WriteByte(SCRIPT_STDUNLOCK_TYPE)
	//sig value
	buf.WriteByte(byte(len(sb)))
	buf.Write(sb)
	//public key
	buf.WriteByte(byte(len(pb)))
	buf.Write(pb)
	return buf.Bytes()
}

func LockedScript(pub *PublicKey) Script {
	s := Script{SCRIPT_STDLOCKED_TYPE}
	hash := Hash160(pub.Encode())
	s = append(s, hash...)
	return s
}
