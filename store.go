package xginx

import (
	"context"
	"errors"
)

type SetValue map[string]interface{}
type IncValue map[string]int

type DBImp interface {
	context.Context
	//删除块
	DelBlock(id []byte) error
	//是否存在
	HasBlock(id []byte) bool
	//获取区块数据
	GetBlock(id []byte, v interface{}) error
	//保存区块数据
	SetBlock(id []byte, meta interface{}, bb []byte) error
	//删除记录
	DelUnit(id []byte) error
	//获取记录
	GetUnit(id []byte, v interface{}) error
	//是否存在记录
	HasUnit(id []byte) bool
	//保存记录
	SetUnit(id []byte, v interface{}) error
	//get trans raw data
	GetTag(id []byte, v interface{}) error
	//save or update tans data
	SetTag(id []byte, v interface{}) error
	//更新计数器
	SetCtr(id []byte, ctr uint) error
	//exists
	HasTag(id []byte) bool
	//delete
	DelTag(id []byte) error
	//事务处理
	Transaction(fn func(sdb DBImp) error) error
}

type DBSession interface {
	UseSession(ctx context.Context, fn func(db DBImp) error) error
}

type TBMeta struct {
	Ver    uint32 `bson:"ver"`    //block ver
	Prev   []byte `bson:"prev"`   //pre block hash
	Merkle []byte `bson:"merkle"` //txs Merkle tree hash + Units hash
	Time   uint32 `bson:"time"`   //时间戳
	Bits   uint32 `bson:"bits"`   //难度
	Nonce  uint32 `bson:"nonce"`  //随机值
	Uts    uint32 `bson:"uts"`    //Units数量
	Txs    uint32 `bson:"txs"`    //tx数量
}

//单元块数据,打卡记录
//块中的一小部分

type UnitKey HashID

type TUnit struct {
	Hash  []byte   `bson:"_id"`   //block hash
	TTS   []byte   `bson:"tts"`   //TT状态 url +2,激活后OO tam map
	TVer  uint32   `bson:"ver"`   //版本 from tag
	TLoc  []uint32 `bson:"tloc"`  //uint32-uint32 位置 from tag
	TPKH  []byte   `bson:"tpkh"`  //标签所有者公钥hash
	TUID  []byte   `bson:"tuid"`  //标签id from tag
	TCTR  uint     `bson:"tctr"`  //标签记录计数器 from tag map
	TMAC  []byte   `bson:"tmac"`  //标签CMAC值 from tag url + 16
	CLoc  []uint32 `bson:"cloc"`  //用户定位信息user location
	Prev  []byte   `bson:"prev"`  //上个hash
	CTime int64    `bson:"ctime"` //客户端时间，不能和服务器相差太大
	CPks  []byte   `bson:"cpks"`  //用户公钥
	CSig  []byte   `bson:"csig"`  //用户签名
	Nonce int64    `bson:"nonce"` //随机值 server full
	STime int64    `bson:"stime"` //服务器时间
	SPks  []byte   `bson:"spks"`  //公钥
	SSig  []byte   `bson:"ssig"`  //签名
}

func (b *TUnit) Save(db DBImp) error {
	return db.SetUnit(b.Hash[:], b)
}

var (
	store DBSession = &mongoDBSession{}
)

//标签数据
type TagKey [16]byte

//保存数据库中的结构
type TTagInfo struct {
	UID  []byte    `bson:"_id"`  //uid
	Ver  uint32    `bson:"ver"`  //版本 from tag
	Loc  []float64 `bson:"loc"`  //uint32-uint32 位置 from tag
	ASV  uint8     `bson:"asv"`  //分配比例
	PKH  []byte    `bson:"pkh"`  //所属公钥hash160
	Keys [5]TagKey `bson:"keys"` //ntag424 5keys
	CTR  uint      `bson:"ctr"`  //ctr int
}

func LoadTagInfo(id TagUID, db DBImp) (*TTagInfo, error) {
	iv := &TTagInfo{}
	return iv, db.GetTag(id[:], iv)
}

func (tag TTagInfo) Mackey() []byte {
	idx := (tag.Ver >> 28) & 0xF
	return tag.Keys[idx][:]
}

func (tag *TTagInfo) SetMacKey(idx int) {
	if idx < 0 || idx >= len(tag.Keys) {
		panic(errors.New("idx out bound"))
	}
	tag.Ver |= uint32((idx & 0xf) << 28)
}

func (tag TTagInfo) Save(db DBImp) error {
	return db.SetTag(tag.UID[:], tag)
}
