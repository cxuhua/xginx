package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
)

//块服务脚本
type BlkLockScript struct {
}

//竞价时脚本只会打包在bbh和beh高度之间的区块上
//如果区块有回退，回退的区块内容作废
//物品id组成
type ObjectId struct {
	BHB VarUInt //区块开始高度
	BHE VarUInt //区块截至高度
	OID HASH160 //物品公钥hash160
}

func (v ObjectId) Equal(b ObjectId) bool {
	return v.BHE == b.BHE && v.BHB == b.BHB && v.OID.Equal(b.OID)
}

func (v *ObjectId) From(id string) error {
	pre, data, err := BECH32Decode(id)
	if err != nil {
		return err
	}
	if pre != conf.ObjIdPrefix {
		return errors.New("prefix error")
	}
	buf := bytes.NewReader(data)
	return v.Decode(buf)
}

func (v ObjectId) String() string {
	buf := &bytes.Buffer{}
	err := v.Encode(buf)
	if err != nil {
		panic(err)
	}
	return BECH32Encode(conf.ObjIdPrefix, buf.Bytes())
}

func (v ObjectId) Encode(w IWriter) error {
	if err := v.BHB.Encode(w); err != nil {
		return err
	}
	if err := v.BHE.Encode(w); err != nil {
		return err
	}
	if err := v.OID.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *ObjectId) Decode(r IReader) error {
	if err := v.BHB.Decode(r); err != nil {
		return err
	}
	if err := v.BHE.Decode(r); err != nil {
		return err
	}
	if err := v.OID.Decode(r); err != nil {
		return err
	}
	return nil
}

//如果需要销毁所得积分也需要出价人和物品提供私钥形成value输出为0的交易进行销毁
//竞价解锁脚本，txin使用
type AucUnlockScript struct {
	Type   uint8    //类型 SCRIPT_AUCUNLOCK_TYPE
	BidPks PKBytes  //出价人公钥 hash160=BidId
	ObjPks PKBytes  //物品公钥 hash160=objId
	BidSig SigBytes //签名
	ObjSig SigBytes //签名
}

func (v AucUnlockScript) EncodeWriter(w IWriter) error {
	if err := binary.Write(w, Endian, v.Type); err != nil {
		return err
	}
	if err := v.BidPks.Encode(w); err != nil {
		return err
	}
	if err := v.ObjPks.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v AucUnlockScript) Encode(w IWriter) error {
	if err := v.EncodeWriter(w); err != nil {
		return err
	}
	if err := v.BidSig.Encode(w); err != nil {
		return err
	}
	if err := v.ObjSig.Encode(w); err != nil {
		return err
	}
	return nil
}

func (v *AucUnlockScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &v.Type); err != nil {
		return err
	}
	if err := v.BidPks.Decode(r); err != nil {
		return err
	}
	if err := v.ObjPks.Decode(r); err != nil {
		return err
	}
	if err := v.BidSig.Decode(r); err != nil {
		return err
	}
	if err := v.ObjSig.Decode(r); err != nil {
		return err
	}
	return nil
}

//3方仲裁脚本
type ArbLockScript struct {
	Type   uint8   //类型 SCRIPT_AUCLOCK_TYPE
	Buyer  HASH160 //买家
	Seller HASH160 //卖家
	ArbId  HASH160 //第三方
	ObjId  HASH160 //物品hashid
}

func (ss ArbLockScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.Buyer.Encode(w); err != nil {
		return err
	}
	if err := ss.Seller.Encode(w); err != nil {
		return err
	}
	if err := ss.ArbId.Encode(w); err != nil {
		return err
	}
	if err := ss.ObjId.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *ArbLockScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.Buyer.Decode(r); err != nil {
		return err
	}
	if err := ss.Seller.Decode(r); err != nil {
		return err
	}
	if err := ss.ArbId.Decode(r); err != nil {
		return err
	}
	if err := ss.ObjId.Decode(r); err != nil {
		return err
	}
	return nil
}

func (s Script) ToArbLock() (*ArbLockScript, error) {
	buf := bytes.NewReader(s)
	ss := &ArbLockScript{}
	if err := ss.Decode(buf); err != nil {
		return nil, err
	}
	return ss, nil
}

//竞价锁定脚本 txout 使用
//每个物品生成密钥对，物品id为公钥的hash160
//当需要拍卖某个id时，生成txout 竞价脚本 SCRIPT_AUCLOCK_TYPE
//私钥丢失由丢失方负责
//当竞价失败需要退回积分需要2方签名，BidId，ObjId,2个私钥签名消费竞价TxOut
//竞价完成需要由双方提供签名将积分退回或者转到相关的账户
//竞价一但成功打包的区块链就不可逆转，具体后续由双发协商
//错误的objectid将可能丢失积分
type AucLockScript struct {
	Type  uint8    //类型 SCRIPT_AUCLOCK_TYPE
	BidId HASH160  //出价人id
	ObjId ObjectId //物品id
}

func (ss AucLockScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.BidId.Encode(w); err != nil {
		return err
	}
	if err := ss.ObjId.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *AucLockScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.BidId.Decode(r); err != nil {
		return err
	}
	if err := ss.ObjId.Decode(r); err != nil {
		return err
	}
	return nil
}

func (ss AucLockScript) ToScript() Script {
	buf := &bytes.Buffer{}
	err := ss.Encode(buf)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (s Script) ToAucLock() (*AucLockScript, error) {
	buf := bytes.NewReader(s)
	ss := &AucLockScript{}
	if err := ss.Decode(buf); err != nil {
		return nil, err
	}
	return ss, nil
}
