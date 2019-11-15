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
	Sign(acc *Account) error
	//获取签名数据
	GetSigBytes() ([]byte, error)
}

//标准签名器
type stdsigner struct {
	bi  *BlockIndex //链索引
	tx  *TX         //当前交易
	out *TxOut      //输入引用的输出
	in  *TxIn       //当前签名验证的输入
	idx int         //输入索引号
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
	wits, err := sr.in.Script.ToWitness()
	if err != nil {
		return err
	}
	if err := wits.Check(); err != nil {
		return err
	}
	if hash, err := wits.Hash(); err != nil {
		return err
	} else if !hash.Equal(sr.out.Script.GetPkh()) {
		return errors.New("hash error txout")
	}
	//至少需要签名正确的数量
	less := int(wits.Less)
	//总的数量
	num := int(wits.Num)
	if len(wits.Pks) != num {
		return errors.New("pub num error")
	}
	if num < less {
		return errors.New("pub num error,num must >= less")
	}
	sigb, err := sr.GetSigBytes()
	if err != nil {
		return err
	}
	sigh := Hash256(sigb)
	for i, k := 0, 0; i < len(wits.Sig) && k < len(wits.Pks); {
		sig, err := NewSigValue(wits.Sig[i][:])
		if err != nil {
			return err
		}
		pub, err := NewPublicKey(wits.Pks[k][:])
		if err != nil {
			return err
		}
		vok := pub.Verify(sigh, sig)
		if vok {
			less--
			i++
		}
		//如果启用仲裁，并且当前仲裁验证成功立即返回
		if vok && wits.IsEnableArb() && wits.Arb == uint8(k) {
			less = 0
			break
		}
		k++
	}
	if less > 0 {
		return errors.New("sig verify error")
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
	if err := sr.in.Script.ForVerify(buf); err != nil {
		return nil, err
	}
	if err := sr.out.Script.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.out.Value.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.OutputsHash().Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.tx.Ext.ForVerify(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//签名生成解锁脚本
func (sr *stdsigner) Sign(acc *Account) error {
	if err := acc.Check(); err != nil {
		return err
	}
	wits := acc.NewWitnessScript()
	if hash, err := wits.Hash(); err != nil {
		return err
	} else if !hash.Equal(sr.out.Script.GetPkh()) {
		return errors.New("out pubs hash error")
	}
	sigb, err := sr.GetSigBytes()
	if err != nil {
		return err
	}
	sigh := Hash256(sigb)
	//向acc请求签名
	for i := 0; i < len(acc.pubs); i++ {
		sigs, err := acc.Sign(i, sigh)
		if err != nil {
			continue
		}
		wits.Sig = append(wits.Sig, sigs)
	}
	if err := wits.Check(); err != nil {
		return err
	}
	if script, err := wits.ToScript(); err != nil {
		return err
	} else {
		sr.in.Script = script
	}
	return nil
}
