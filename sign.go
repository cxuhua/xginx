package xginx

import (
	"errors"
	"fmt"
)

//IGetSigBytes 获取签名数据接口
type IGetSigBytes interface {
	GetSigBytes() ([]byte, error)
}

//ISignTx 签名交易
type ISignTx interface {
	SignTx(singer ISigner, pass ...string) error
}

var (
	//如果签名时返回这个忽略
	ErrIgnoreSignError = errors.New("ignore sign error")
)

//ISigner 签名验证接口
type ISigner interface {
	//签名校验
	Verify(bi *BlockIndex) error
	//签名生成解锁脚本
	Sign(bi *BlockIndex, lis ISignTx, pass ...string) error
	//获取签名hash
	GetSigHash() ([]byte, error)
	//获取签名对象 当前交易，当前输入，输入引用的输出,输入在交易中的索引
	GetObjs() (*TX, *TxIn, *TxOut, int)
	//检测签名 脚本调用
	VerifySign() error
	//验证地址 脚本调用
	VerifyAddr() error
}

//多重签名器
type mulsigner struct {
	tx  *TX    //当前交易
	out *TxOut //输入引用的输出
	in  *TxIn  //当前签名验证的输入
	idx int    //输入索引
}

//NewSigner 新建标准签名
func NewSigner(tx *TX, out *TxOut, in *TxIn, idx int) ISigner {
	return &mulsigner{
		tx:  tx,
		out: out,
		in:  in,
		idx: idx,
	}
}

//GetObjs 获取签名对象
func (sr *mulsigner) GetObjs() (*TX, *TxIn, *TxOut, int) {
	return sr.tx, sr.in, sr.out, sr.idx
}

//验证签名是否正确
func (sr *mulsigner) VerifySign() error {
	//获取输入脚本
	wits, err := sr.in.Script.ToWitness()
	if err != nil {
		return fmt.Errorf("witness script miss %w", err)
	}
	if err := wits.Check(); err != nil {
		return fmt.Errorf("witness check error %w", err)
	}
	//获取签名hash
	sigh, err := sr.GetSigHash()
	if err != nil {
		return err
	}
	//转换统一校验签名
	acc, err := wits.ToAccount()
	if err != nil {
		return err
	}
	//多重签名校验
	return acc.VerifyAll(sigh, wits.Sig)
}

//检测hash是否一致
func (sr *mulsigner) VerifyAddr() error {
	//获取输入脚本
	wits, err := sr.in.Script.ToWitness()
	if err != nil {
		return fmt.Errorf("witness script miss %w", err)
	}
	if err := wits.Check(); err != nil {
		return fmt.Errorf("witness check error %w", err)
	}
	//获取锁定脚本
	locked, err := sr.out.Script.ToLocked()
	if err != nil {
		return fmt.Errorf("locked script miss %w", err)
	}
	//pkh一致才能通过
	if hash, err := wits.Hash(); err != nil || !hash.Equal(locked.Pkh) {
		return fmt.Errorf("hash equal error %w", err)
	}
	return nil
}

//Verify 多重签名脚本验证
func (sr *mulsigner) Verify(bi *BlockIndex) error {
	//获取输入脚本
	wits, err := sr.in.Script.ToWitness()
	if err != nil {
		return fmt.Errorf("witness script miss %w", err)
	}
	if err := wits.Check(); err != nil {
		return fmt.Errorf("witness check error %w", err)
	}
	//获取锁定脚本
	locked, err := sr.out.Script.ToLocked()
	if err != nil {
		return fmt.Errorf("locked script miss %w", err)
	}
	//执行脚本
	return sr.ExecScript(bi, wits, locked)
}

//OutputsHash outhash
func (sr *mulsigner) OutputsHash() HASH256 {
	if hash, set := sr.tx.outs.IsSet(); set {
		return hash
	}
	buf := NewWriter()
	for _, v := range sr.tx.Outs {
		err := v.Encode(buf)
		if err != nil {
			panic(err)
		}
	}
	return sr.tx.outs.Hash(buf.Bytes())
}

//PrevoutHash hash
func (sr *mulsigner) PrevoutHash() HASH256 {
	if hash, set := sr.tx.pres.IsSet(); set {
		return hash
	}
	buf := NewWriter()
	for _, v := range sr.tx.Ins {
		err := v.OutHash.Encode(buf)
		if err != nil {
			panic(err)
		}
		err = v.OutIndex.Encode(buf)
		if err != nil {
			panic(err)
		}
	}
	return sr.tx.pres.Hash(buf.Bytes())
}

//GetSigHash 获取输入签名数据
//out 当前输入对应的上一个输出,idx 当前输入的索引位置
func (sr *mulsigner) GetSigHash() ([]byte, error) {
	buf := NewWriter()
	if err := sr.tx.Ver.Encode(buf); err != nil {
		return nil, err
	}
	//所有引用的交易hash
	if err := sr.PrevoutHash().Encode(buf); err != nil {
		return nil, err
	}
	//当前输入引用hash
	if err := sr.in.OutHash.Encode(buf); err != nil {
		return nil, err
	}
	if err := sr.in.OutIndex.Encode(buf); err != nil {
		return nil, err
	}
	//输入脚本
	if err := sr.in.Script.ForVerify(buf); err != nil {
		return nil, err
	}
	if err := sr.in.Sequence.Encode(buf); err != nil {
		return nil, err
	}
	//输出脚本
	if err := sr.out.Script.Encode(buf); err != nil {
		return nil, err
	}
	//输出值
	if err := sr.out.Value.Encode(buf); err != nil {
		return nil, err
	}
	//所有输出hash
	if err := sr.OutputsHash().Encode(buf); err != nil {
		return nil, err
	}
	//交易脚本
	if err := sr.tx.Script.Encode(buf); err != nil {
		return nil, err
	}
	return Hash256(buf.Bytes()), nil
}

//Sign 开始签名
func (sr *mulsigner) Sign(bi *BlockIndex, lis ISignTx, pass ...string) error {
	return lis.SignTx(sr, pass...)
}
