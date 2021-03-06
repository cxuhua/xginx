package xginx

import (
	"errors"
)

// CheckProofOfWork whether a block hash satisfies the proof-of-work requirement specified by nBits
func CheckProofOfWork(hash HASH256, bits uint32) bool {
	h := UINT256{}
	n, o := h.SetCompact(bits)
	if n {
		return false
	}
	if h.IsZero() {
		return false
	}
	if o {
		return false
	}
	if h.Cmp(conf.LimitHash) > 0 {
		return false
	}
	ch := hash.ToU256()
	return ch.Cmp(h) <= 0
}

//GetMinPowBits Minimum difficulty
func GetMinPowBits() uint32 {
	return NewUINT256(conf.PowLimit).Compact(false)
}

//CheckProofOfWorkBits 检测难度值是否正确
func CheckProofOfWorkBits(bits uint32) bool {
	h := UINT256{}
	n, o := h.SetCompact(bits)
	if n {
		return false
	}
	if h.IsZero() {
		return false
	}
	if o {
		return false
	}
	return h.Cmp(conf.LimitHash) <= 0
}

//CalculateWorkRequired 计算工作难度
//ct = lastBlock blockTime
//pt = lastBlock - 2016 + 1 blockTime
//pw = lastBlock's bits
func CalculateWorkRequired(ct uint32, pt uint32, pw uint32) uint32 {
	span := uint32(conf.PowTime)
	limit := conf.LimitHash
	sub := ct - pt
	if sub <= 0 {
		panic(errors.New("ct pt error"))
	}
	if sv := span / 4; sub < sv {
		sub = sv
	}
	if sv := span * 4; sub > sv {
		sub = sv
	}
	n := UINT256{}
	n.SetCompact(pw)
	n = n.MulUInt32(sub)
	n = n.Div(NewUINT256(span))
	if n.Cmp(limit) > 0 {
		n = limit
	}
	return n.Compact(false)
}
