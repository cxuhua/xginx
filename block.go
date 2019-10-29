package xginx

//一个块由n个条目组成

//块信息
type BlockInfo struct {
	Ver       uint32
	Prev      HashID
	Merkle    HashID //Merkle tree root
	Timestamp uint32
	Bits      uint32
	Nonce     uint32
	DisBase   UnitDisBase
	Bodys     []UnitBody //n个记录单元,至少有一个记录
}

//区块打包者奖励条目
type UnitDisBase struct {
	ClientID  UserID   //指定用户id
	Custom    VarBytes //4-100字节的自定义数据
	TagDisSum VarUInt  //标签定位记录单元新增和 Bodys 单元距离和
	CliDisSum VarUInt  //用户定位
}

//条目
type UnitBody struct {
	ClientID  UserID      //用户公钥的hash160
	OutHash   HashID      //上个输出块hash
	OutIndex  VarUInt     //上个块所在的元素 Bodys索引
	Items     []UnitBlock //多个连续的记录信息，记录client链
	TagDisSum VarUInt     //标签距离合计，后一个经纬度与前一个距离之和 单位：米
	CliDisSum VarUInt     //用户定位合计,单位:米
}
