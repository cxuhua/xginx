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
	wits := sr.in.Script.ToWitness()
	if wits.Type != SCRIPT_WITNESS_TYPE {
		return errors.New("witness script type error")
	}
	pub, err := NewPublicKey(wits.Pks[:])
	if err != nil {
		return err
	}
	if !pub.Hash().Equal(sr.out.Script.GetPkh()) {
		return errors.New("not mine txout")
	}
	sig, err := NewSigValue(wits.Sig[:])
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

func (sp *stdsigner) OutputsHash() HASH256 {
	if hash, set := sp.tx.outs.IsSet(); set {
		return hash
	}
	buf := &bytes.Buffer{}
	for _, v := range sp.tx.Outs {
		err := v.Encode(buf)
		if err != nil {
			panic(err)
		}
	}
	return sp.tx.outs.Hash(buf.Bytes())
}

func (sp *stdsigner) PrevoutHash() HASH256 {
	if hash, set := sp.tx.pres.IsSet(); set {
		return hash
	}
	buf := &bytes.Buffer{}
	for _, v := range sp.tx.Ins {
		err := v.OutHash.Encode(buf)
		if err != nil {
			panic(err)
		}
		err = v.OutIndex.Encode(buf)
		if err != nil {
			panic(err)
		}
	}
	return sp.tx.pres.Hash(buf.Bytes())
}

//获取输入签名数据
//out 当前输入对应的上一个输出,idx 当前输入的索引位置
func (sr *stdsigner) GetSigBytes() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := sr.tx.Ver.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.PrevoutHash().Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.in.OutHash.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.in.OutIndex.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.in.ExtBytes.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.in.Script.ForVerify(buf); err != nil {
		return nil, err
	}
	if err := sr.out.Value.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.OutputsHash().Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//签名生成解锁脚本
func (sr *stdsigner) Sign(pri *PrivateKey) error {
	pub := pri.PublicKey()
	if !pub.Hash().Equal(sr.out.Script.GetPkh()) {
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
	sr.in.Script = NewWitnessScript(pri.PublicKey(), sig).ToScript()
	return nil
}
