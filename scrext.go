package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
)

//扩展存储，数据由矿工打包存储，并收取交易费为数据打包费
//消费成功需要矿工和付款人签名
//step1 发出存储交易,告诉矿工，需要存储的数据大小和hash值
//step2 矿工收到交易后确认，输入，输出，交易费是否合适，接受填写 MPks MSig Msig之前的内容签名，发送给发起人
//step3 发起人收到矿工的回复后确认，然后添加签名 SPks SSig,然后发给矿工
//step4 矿工收到后验证数据hash，大小，发起人签名是否正确，然后开始挖矿并获得交易费，并且将就存储到区块
type ExtLockScript struct {
	Type  uint8    //类型 SCRIPT_EXTLOCKED_TYPE s1 s2 s3
	MPkh  HASH160  //消费公钥hash，矿工填写,决定最终这个交易谁可以消费 s2 s3
	Size  VarUInt  //数据大小 s1 s2 s3
	Hash  HASH256  //数据hash s1 s2 s3
	MPks  PKBytes  //矿工接受后返回公钥和签名 s2 s3
	MSig  SigBytes //矿工签名 对之上数据签名 s3
	Bytes VarBytes //需要存储的数据，不能太大 < MAX_EXT_SCRIPT_SIZE，存储在ext中 s3
	SPks  PKBytes  //发起人公钥
	SSig  SigBytes //发起人签名 对之上数据签名
}

//func (v ExtLockScript) Check() error {
//	return nil
//}
//
//func (v ExtLockScript) Encode(w IWriter) error {
//	if err := binary.Write(w, Endian, v.Type); err != nil {
//		return err
//	}
//	if err := v.MPkh.Encode(w); err != nil {
//		return err
//	}
//	if err := v.Hash.Encode(w); err != nil {
//		return err
//	}
//	return nil
//}
//
//func (v *ExtLockScript) Decode(r IReader) error {
//	if err := binary.Read(r, Endian, &v.Type); err != nil {
//		return err
//	}
//	if err := v.MPkh.Decode(r); err != nil {
//		return err
//	}
//	if err := v.Hash.Decode(r); err != nil {
//		return err
//	}
//	return nil
//}

type ExtUnlockScript struct {
	Type uint8    //SCRIPT_EXTUNLOCK_TYPE
	Pks  PKBytes  //公钥
	Sig  SigBytes //签名
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
