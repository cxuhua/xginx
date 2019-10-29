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
