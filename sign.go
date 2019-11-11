package xginx

import (
	"bytes"
	"errors"
)

//获取签名数据接口
type IGetSigBytes interface {
	GetSigBytes() ([]byte, error)
}

//签名验证接口
type ISigner interface {
	//签名校验
	Verify() error
	//签名生成解锁脚本
	Sign(pri *PrivateKey) error
}

//标准签名器
type stdsigner struct {
	bi  *BlockIndex
	tx  *TX
	out *TxOut
	in  *TxIn
	idx int
}

//新建标准签名
func newStdSigner(bi *BlockIndex, tx *TX, out *TxOut, in *TxIn, idx int) ISigner {
	return &stdsigner{
		bi:  bi,
		tx:  tx,
		out: out,
		in:  in,
		idx: idx,
	}
}

//签名校验
func (sr *stdsigner) Verify() error {
	std, err := StdUnlockScriptFrom(sr.in.Script)
	if err != nil {
		return err
	}
	pub, err := NewPublicKey(std.Pks[:])
	if err != nil {
		return err
	}
	pkh, err := sr.out.GetPKH()
	if err != nil {
		return err
	}
	if !pub.Hash().Equal(pkh) {
		return errors.New("not mine txout")
	}
	sig, err := NewSigValue(std.Sig[:])
	if err != nil {
		return err
	}
	sigb, err := sr.GetSigBytes()
	if err != nil {
		return err
	}
	if !pub.Verify(Hash256(sigb), sig) {
		return errors.New("verify failed")
	}
	return nil
}

//获取输入签名数据
//out 当前输入对应的上一个输出,idx 当前输入的索引位置
func (sr *stdsigner) GetSigBytes() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := sr.tx.Ver.Encode(buf); err != nil {
		return nil, err
	}
	err := VarUInt(len(sr.tx.Ins)).Encode(buf)
	if err != nil {
		return nil, err
	}
	for i, v := range sr.tx.Ins {
		err := v.OutHash.Encode(buf)
		if err != nil {
			return nil, err
		}
		err = v.OutIndex.Encode(buf)
		if err != nil {
			return nil, err
		}
		if i == sr.idx {
			err = sr.out.Script.Encode(buf)
		} else {
			err = Script{}.Encode(buf)
		}
		if err != nil {
			return nil, err
		}
	}
	err = VarUInt(len(sr.tx.Outs)).Encode(buf)
	if err != nil {
		return nil, err
	}
	for _, v := range sr.tx.Outs {
		if err := v.Encode(buf); err != nil {
			return nil, err
		}
	}
	//
	//最后放签名类型，默认为1
	if err := VarUInt(1).Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//签名生成解锁脚本
func (sr *stdsigner) Sign(pri *PrivateKey) error {
	pkh, err := sr.out.GetPKH()
	if err != nil {
		return err
	}
	if !pri.PublicKey().Hash().Equal(pkh) {
		return errors.New("not mine txout")
	}
	sigb, err := sr.GetSigBytes()
	if err != nil {
		return err
	}
	sig, err := pri.Sign(Hash256(sigb))
	if err != nil {
		return err
	}
	script, err := NewStdUnlockScript(pri.PublicKey(), sig)
	if err != nil {
		return err
	}
	sr.in.Script = script
	return nil
}
