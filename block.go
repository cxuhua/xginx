package xginx

//一个块由n个条目组成

//块信息
//Bodys记录中不能用相同的clientid，items必须时间上连续，hash能前后衔接
//txs交易部分和比特币类似
//块大小限制为4M大小
type BlockInfo struct {
	Ver       uint32
	Prev      HashID
	Merkle    HashID //Merkle tree root
	Timestamp uint32
	Bits      uint32
	Nonce     uint32
	Bodys     []UnitBody //n个记录单元,至少有1个记录
	Txs       []UnitTX   //距离交易，类似比特币,第一个coinbase交易
}

//交易输入
type UnitTxIn struct {
	OutHash  HashID  //输出交易hash
	OutIndex VarUInt //对应的输出索引
	Script   Script  //解锁脚本
}

//是否基本单元，txs的第一个一定是base，输出为奖励计算的距离
func (in UnitTxIn) IsBase() bool {
	return in.OutHash.IsZero() && in.OutIndex == 0
}

//交易输出
type UnitTxOut struct {
	Value  VarUInt //距离奖励 GetRewardRate 计算比例
	Script Script  //锁定脚本
}

//交易
type UnitTX struct {
	Ver  VarUInt     //版本
	Ins  []UnitTxIn  //输入
	Outs []UnitTxOut //输出
}

//条目
type UnitBody struct {
	ClientID  UserID  //用户公钥的hash160
	PrevHash  HashID  //上个块hash
	PrevIndex VarUInt //上个块所在Bodys索引
	//多个连续的记录信息，记录client链,至少有两个记录
	//两个点之间的时间超过1天将忽略距离
	//定位点与标签点差距超过1km，距离递减 获得1-距离/10的比例,10km后完全获得不了距离了 GetDisRate 计算
	//以上都不影响链的链接，只是会减少距离提成
	Items []UnitBlock
	//标签距离合计，后一个经纬度与前一个距离之和 单位：米,如果有prevhash需要计算第一个与prevhash指定的最后一个单元距离
	//clientid最终距离
	Distance VarUInt
}
