package xginx

import "bytes"

const (
	SCRIPT_BASE_TYPE   = uint8(0) //coinbase input 0
	SCRIPT_UNLOCK_TYPE = uint8(1) //pubkey sigvalue
	//标准交易
	SCRIPT_LOCKED_TYPE = uint8(2) //HASH160(pubkey)
	//竞价成功不可消费类型
	SCRIPT_AUCNOREV_TYPE = uint8(3) //HASH160(pubkey)
	//竞价成功可以消费类型
	SCRIPT_AUCTION_TYPE = uint8(4) //HASH160(pubkey)
)

type Script []byte

func (s Script) Len() int {
	return len(s)
}

func (s Script) Type() uint8 {
	return s[0]
}

func (s Script) IsUnlockScript() bool {
	return s.Len() > 1 && s[0] == SCRIPT_UNLOCK_TYPE
}

func (s Script) IsLockedcript() bool {
	return s.Len() > 1 && s[0] == SCRIPT_LOCKED_TYPE
}

func (s Script) IsBaseScript() bool {
	return s.Len() > 1 && s[0] == SCRIPT_BASE_TYPE
}

func (s Script) Encode(w IWriter) error {
	return VarBytes(s).Encode(w)
}

func (s *Script) Decode(r IReader) error {
	return (*VarBytes)(s).Decode(r)
}

func BaseScript(b []byte) Script {
	s := Script{SCRIPT_BASE_TYPE}
	s = append(s, b...)
	return s
}

func UnlockScript(pub *PublicKey, sig *SigValue) Script {
	pb := pub.Encode()
	sb := sig.Encode()
	buf := &bytes.Buffer{}
	//type
	buf.WriteByte(SCRIPT_UNLOCK_TYPE)
	//sig value
	buf.WriteByte(byte(len(sb)))
	buf.Write(sb)
	//public key
	buf.WriteByte(byte(len(pb)))
	buf.Write(pb)
	return buf.Bytes()
}

func LockedScript(pub *PublicKey) Script {
	s := Script{SCRIPT_LOCKED_TYPE}
	hash := Hash160(pub.Encode())
	s = append(s, hash...)
	return s
}
