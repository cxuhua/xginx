package xginx

import "bytes"

//消费积分进行拍卖规则
//每个拍卖项生成唯一hash160(拍卖id)
//当需要拍卖某个id时，生成txout 回收积分脚本 SCRIPT_RECOVER_TYPE
//脚本包含叫价人公钥hash160，拍卖id，叫价公钥hash160， 还有（积分+叫价人hashid+拍卖人id+叫价公钥hash160）签名，叫价私钥进行签名
//每次叫价，叫价者都会生成一对密钥用来签名，多次叫价不可用相同的密钥对
//一个区块中针对一个拍卖id出价最高的输出不能转出，永远被消耗掉获得拍卖id控制权
//其他叫价低的输出还是可以交易转出，拍卖id是唯一的，一但被区块打包将永远无效，所以需要记录拍卖id在整个区块链中不可重复
//但进行物品控制时，需要提供叫价私钥对区块中对应的拍卖idSCRIPT_RECOVER_TYPE类输出进行签名验证，开发需要注意区块验证成功才能对物品进行
//相应的控制,叫价公钥hash160也必须一致，签名也必须一致，所以叫价私钥一旦丢失，你拍卖到的物品也会被其他人利用

//定义回收消费脚本
type AuctionScript struct {
	Type   uint8    //类型 SCRIPT_RECOVER_TYPE
	Bidder UserID   //出价人id
	Object UserID   //物品id，在单个区块中会有重复的
	BidOne UserID   //此次叫价id，由叫价人生成密钥对，私钥生成此id
	RSig   SigBytes //叫价人签名，签名还要包括输出积分TxOut.Value
}

//生成拍卖消费脚本
func (rs AuctionScript) ToScript(value VarUInt, pri *PrivateKey) (Script, error) {
	buf := &bytes.Buffer{}
	if err := value.Encode(buf); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(rs.Type); err != nil {
		return nil, err
	}
	if _, err := buf.Write(rs.Bidder[:]); err != nil {
		return nil, err
	}
	if _, err := buf.Write(rs.Object[:]); err != nil {
		return nil, err
	}
	if _, err := buf.Write(rs.BidOne[:]); err != nil {
		return nil, err
	}
	hash := HASH256(buf.Bytes())
	sig, err := pri.Sign(hash)
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(sig.Encode()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
