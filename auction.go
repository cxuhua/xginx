package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
)

//消费积分进行拍卖规则
//每个拍卖项生成唯一hash160(拍卖id)
//当需要拍卖某个id时，生成txout 回收积分脚本 SCRIPT_RECOVER_TYPE 或者拍卖脚本
//脚本包含叫价人公钥hash160，拍卖id，叫价公钥hash160， 还有（积分+叫价人Hash256+拍卖人id+叫价公钥hash160）签名，叫价私钥进行签名
//每次叫价，叫价者都会生成一对密钥用来签名，多次叫价不可用相同的密钥对
//一个区块中针对一个拍卖id出价最高的输出不能转出，永远被消耗掉获得拍卖id控制权
//其他叫价低的输出还是可以交易转出，拍卖id是唯一的，一但被区块打包将永远无效，所以需要记录拍卖id在整个区块链中不可重复
//但进行物品控制时，需要提供叫价私钥对区块中对应的拍卖idSCRIPT_RECOVER_TYPE类输出进行签名验证，开发需要注意区块验证成功才能对物品进行
//相应的控制,叫价公钥hash160也必须一致，签名也必须一致，所以叫价私钥一旦丢失，你拍卖到的物品也会被其他人利用
//SCRIPT_AUCTION_TYPE 这个类型的脚本可消费，表示物品所属人获得积分
//如果脚本类型和物品设定的不一致，拍卖会无效,特别是如果设置成SCRIPT_RECOVER_TYPE类型积分会丢失
//拍卖脚本
type AuctionScript struct {
	Type   uint8    //类型 SCRIPT_RECOVER_TYPE
	Owner  Hash160  //竞价失败后积分可由此id消费,如果类型是SCRIPT_RECOVER_TYPE并且出价最高积分将永远不能转出
	ObjId  Hash160  //物品id
	Time   int64    //出价时间，如果价格一致先出价获得
	BidPks PKBytes  //此次叫价公钥，由叫价人生成密钥对
	BidSig SigBytes //叫价人签名，签名还要包括输出积分TxOut.Value
	value  VarUInt  //当前对应的输出积分
}

func (ss AuctionScript) Equal(vv AuctionScript) bool {
	eq := ss.Type == vv.Type && ss.Owner.Equal(vv.Owner) && ss.ObjId.Equal(vv.ObjId)
	eq = eq && bytes.Equal(ss.BidPks[:], vv.BidPks[:])
	eq = eq && bytes.Equal(ss.BidSig[:], vv.BidSig[:])
	return eq
}

func (ss AuctionScript) EncodeWriter(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.Owner.Encode(w); err != nil {
		return err
	}
	if err := ss.ObjId.Encode(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, ss.Time); err != nil {
		return err
	}
	if err := ss.BidPks.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss AuctionScript) Encode(w IWriter) error {
	if err := ss.EncodeWriter(w); err != nil {
		return err
	}
	if err := ss.BidSig.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *AuctionScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.Owner.Decode(r); err != nil {
		return err
	}
	if err := ss.ObjId.Decode(r); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &ss.Time); err != nil {
		return err
	}
	if err := ss.BidPks.Decode(r); err != nil {
		return err
	}
	if err := ss.BidSig.Decode(r); err != nil {
		return err
	}
	return nil
}

func (ss AuctionScript) ToScript() (Script, error) {
	buf := &bytes.Buffer{}
	if err := ss.Encode(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s Script) ToAuction() (*AuctionScript, error) {
	buf := bytes.NewReader(s)
	ss := &AuctionScript{}
	if err := ss.Decode(buf); err != nil {
		return nil, err
	}
	return ss, nil
}

//校验签名
func (ss AuctionScript) Verify(value VarUInt) error {
	pub, err := NewPublicKey(ss.BidPks[:])
	if err != nil {
		return err
	}
	sig, err := NewSigValue(ss.BidSig[:])
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err := value.Encode(buf); err != nil {
		return err
	}
	if err := ss.EncodeWriter(buf); err != nil {
		return err
	}
	hash := HASH256(buf.Bytes())
	if !pub.Verify(hash, sig) {
		return errors.New("verify auction script error")
	}
	return nil
}

//生成拍卖消费脚本
//签名数据
func (ss *AuctionScript) Sign(value VarUInt, pbk *PrivateKey) error {
	ss.BidPks.Set(pbk.PublicKey())
	buf := &bytes.Buffer{}
	if err := value.Encode(buf); err != nil {
		return err
	}
	if err := ss.EncodeWriter(buf); err != nil {
		return err
	}
	hash := HASH256(buf.Bytes())
	sig, err := pbk.Sign(hash)
	if err != nil {
		return err
	}
	ss.BidSig.Set(sig)
	return nil
}
