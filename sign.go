package xginx

import (
	"errors"
	"fmt"
)

//获取签名数据接口
type IGetSigBytes interface {
	GetSigBytes() ([]byte, error)
}

//签名交易
type ISignerListener interface {
	SignTx(singer ISigner) error
}

//签名验证接口
type ISigner interface {
	//签名校验
	Verify() error
	//签名生成解锁脚本
	Sign(lis ISignerListener) error
	//获取签名hash
	GetSigHash() ([]byte, error)
	//获取签名对象 当前交易，当前输入，输入引用的输出,输入在交易中的位置
	GetObjs() (*TX, *TxIn, *TxOut, int)
	//获取消费地址
	GetAddress() Address
	//获取交易id
	GetTxId() HASH256
}

//多重签名器
type mulsigner struct {
	tx  *TX    //当前交易
	out *TxOut //输入引用的输出
	in  *TxIn  //当前签名验证的输入
	idx int    //输入索引
}

//新建标准签名
func NewSigner(tx *TX, out *TxOut, in *TxIn, idx int) ISigner {
	return &mulsigner{
		tx:  tx,
		out: out,
		in:  in,
		idx: idx,
	}
}

//
func (sr *mulsigner) GetTxId() HASH256 {
	return sr.tx.MustID()
}

//获取输出对应的地址
func (sr *mulsigner) GetAddress() Address {
	addr, err := sr.out.Script.GetAddress()
	if err != nil {
		panic(err)
	}
	return addr
}

//获取签名对象
func (sr *mulsigner) GetObjs() (*TX, *TxIn, *TxOut, int) {
	return sr.tx, sr.in, sr.out, sr.idx
}

//多重签名验证
func (sr *mulsigner) Verify() error {
	wits, err := sr.in.Script.ToWitness()
	if err != nil {
		return err
	}
	if err := wits.Check(); err != nil {
		return err
	}
	pkh, err := sr.out.Script.GetPkh()
	if err != nil {
		return err
	}
	if hash, err := wits.Hash(); err != nil || !hash.Equal(pkh) {
		return fmt.Errorf("hash equal error %w", err)
	}
	sigh, err := sr.GetSigHash()
	if err != nil {
		return err
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
		}
		if less == 0 {
			break
		}
		k++
	}
	if less > 0 {
		return errors.New("sig verify error")
	}
	return nil
}

func (sp *mulsigner) OutputsHash() HASH256 {
	if hash, set := sp.tx.outs.IsSet(); set {
		return hash
	}
	buf := NewWriter()
	for _, v := range sp.tx.Outs {
		err := v.Encode(buf)
		if err != nil {
			panic(err)
		}
	}
	return sp.tx.outs.Hash(buf.Bytes())
}

func (sp *mulsigner) PrevoutHash() HASH256 {
	if hash, set := sp.tx.pres.IsSet(); set {
		return hash
	}
	buf := NewWriter()
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
func (sr *mulsigner) GetSigHash() ([]byte, error) {
	buf := NewWriter()
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
	if err := buf.TWrite(sr.in.Sequence); err != nil {
		return nil, err
	}
	if err := sr.OutputsHash().Encode(buf); err != nil {
		return nil, err
	}
	if err := buf.TWrite(sr.tx.LockTime); err != nil {
		return nil, err
	}
	return Hash256(buf.Bytes()), nil
}

func (sr *mulsigner) Sign(lis ISignerListener) error {
	return lis.SignTx(sr)
}
