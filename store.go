package xginx

import (
	"context"
	"errors"
	"sync"

	"go.mongodb.org/mongo-driver/bson"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SetValue map[string]interface{}
type IncValue map[string]int

type DBImp interface {
	context.Context
	SetBlock(id []byte, v interface{}) error
	//get trans raw data
	GetTag(id []byte, v interface{}) error
	//save or update tans data
	SetTag(id []byte, v interface{}) error
	//findandmodify
	AtomicCtr(id []byte, ctr uint) error
	//exists tx
	HasTag(id []byte) bool
	//delete tx
	DelTag(id []byte) error
	//事物处理
	Transaction(fn func(sdb DBImp) error) error
}

//块数据,打卡记录

type BlockKey HashID

type TBlockInfo struct {
	Hash  []byte   `bson:"_id"`   //block hash
	TTS   []byte   `bson:"tts"`   //TT状态 url +2,激活后OO tam map
	TVer  uint32   `bson:"ver"`   //版本 from tag
	TLoc  []uint32 `bson:"tloc"`  //uint32-uint32 位置 from tag
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
	SPks  []byte   `bson:"spks"`  //服务器公钥
	SSig  []byte   `bson:"ssig"`  //服务器签名
}

func (b *TBlockInfo) Save(db DBImp) error {
	return db.SetBlock(b.Hash[:], b)
}

//标签数据
type TagKey [16]byte

//保存数据库中的结构
type TTagInfo struct {
	UID  []byte    `bson:"_id"`  //uid
	Ver  uint32    `bson:"ver"`  //版本 from tag
	Loc  []float64 `bson:"loc"`  //uint32-uint32 位置 from tag
	Keys [5]TagKey `bson:"keys"` //
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

var (
	client *mongo.Client = nil
	dbonce               = sync.Once{}
)

type mongoDBImp struct {
	context.Context
}

func (m *mongoDBImp) collection(t string) *mongo.Collection {
	return m.database().Collection(t)
}

func (m *mongoDBImp) blocks() *mongo.Collection {
	return m.database().Collection("blocks")
}

func (m *mongoDBImp) tags() *mongo.Collection {
	return m.database().Collection("tags")
}

func (m *mongoDBImp) client() *mongo.Client {
	return m.Context.(mongo.SessionContext).Client()
}

func (m *mongoDBImp) database() *mongo.Database {
	return m.client().Database("xginx")
}

//设置计数器，必须比数据库中的大
func (m *mongoDBImp) AtomicCtr(id []byte, ctr uint) error {
	c := bson.M{"_id": id, "ctr": bson.M{"$lt": ctr}}
	d := bson.M{"$set": bson.M{"ctr": ctr}}
	res := m.tags().FindOneAndUpdate(m, c, d)
	return res.Err()
}

func NewDBImp(ctx context.Context) DBImp {
	return &mongoDBImp{Context: ctx}
}

//delete data
func (m *mongoDBImp) DelTag(id []byte) error {
	_, err := m.tags().DeleteOne(m, bson.M{"_id": id})
	if err != nil {
		return err
	}
	return err
}

//get tx data
func (m *mongoDBImp) GetTag(id []byte, v interface{}) error {
	ret := m.tags().FindOne(m, bson.M{"_id": id})
	if err := ret.Err(); err != nil {
		return err
	}
	return ret.Decode(v)
}

//check tx exists
func (m *mongoDBImp) HasTag(id []byte) bool {
	ret := m.tags().FindOne(m, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"_id": 1}))
	return ret.Err() == nil
}

func (m *mongoDBImp) set(t string, id []byte, v interface{}) error {
	tbl := m.collection(t)
	switch v.(type) {
	case IncValue:
		ds := bson.M{}
		for k, v := range v.(IncValue) {
			ds[k] = v
		}
		if len(ds) > 0 {
			_, err := tbl.UpdateOne(m, bson.M{"_id": id}, bson.M{"$inc": ds})
			return err
		}
	case SetValue:
		ds := bson.M{}
		for k, v := range v.(SetValue) {
			ds[k] = v
		}
		if len(ds) > 0 {
			_, err := tbl.UpdateOne(m, bson.M{"_id": id}, bson.M{"$set": ds})
			return err
		}
	default:
		opt := options.Update().SetUpsert(true)
		_, err := tbl.UpdateOne(m, bson.M{"_id": id}, bson.M{"$set": v}, opt)
		return err
	}
	return nil
}

func (m *mongoDBImp) SetBlock(id []byte, v interface{}) error {
	return m.set("blocks", id, v)
}

//save tans data
func (m *mongoDBImp) SetTag(id []byte, v interface{}) error {
	return m.set("tags", id, v)
}

func (m *mongoDBImp) Transaction(fn func(sdb DBImp) error) error {
	sess := m.Context.(mongo.SessionContext)
	_, err := sess.WithTransaction(m, func(sctx mongo.SessionContext) (i interface{}, e error) {
		return nil, fn(NewDBImp(sctx))
	})
	return err
}

func InitDB(ctx context.Context) *mongo.Client {
	dbonce.Do(func() {
		c := options.Client().ApplyURI("mongodb://127.0.0.1:27017/")
		cptr, err := mongo.NewClient(c)
		if err != nil {
			panic(err)
		}
		err = cptr.Connect(ctx)
		if err != nil {
			panic(err)
		}
		client = cptr
	})
	return client
}

func UseTransaction(ctx context.Context, fn func(sdb DBImp) error) error {
	return UseSession(ctx, func(db DBImp) error {
		return db.Transaction(func(sdb DBImp) error {
			return fn(sdb)
		})
	})
}

func UseSession(ctx context.Context, fn func(db DBImp) error) error {
	client = InitDB(ctx)
	return client.UseSession(ctx, func(sess mongo.SessionContext) error {
		return fn(NewDBImp(sess))
	})
}
