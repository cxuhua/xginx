package xginx

const (
	SCRIPT_BASE_TYPE   = uint8(0) //coinbase input 0
	SCRIPT_UNLOCK_TYPE = uint8(1) //pubkey sigvalue
	SCRIPT_LOCKED_TYPE = uint8(2) //hash160(pubkey)
)

type Script []byte

func (s Script) Ver() uint8 {
	return s[0]
}

func (s Script) Encode(w IWriter) error {
	return VarBytes(s).Encode(w)
}

func (s *Script) Decode(r IReader) error {
	return (*VarBytes)(s).Decode(r)
}

func NewBaseScript(b []byte) Script {
	s := Script{SCRIPT_BASE_TYPE}
	s = append(s, b...)
	return s
}

func NewUnlockScript(sig *SigValue) Script {
	s := Script{SCRIPT_UNLOCK_TYPE}
	s = append(s, sig.Encode()...)
	return s
}

func NewLockScript(pub *PublicKey) Script {
	s := Script{SCRIPT_LOCKED_TYPE}
	s = append(s, pub.Encode()...)
	return s
}
