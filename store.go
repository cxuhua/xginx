package xginx

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/bson"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SetValue map[string]interface{}
type IncValue map[string]int

type DBImp interface {
	context.Context
	//get trans raw data
	GetTag(id []byte, v interface{}) error
	//save or update tans data
	SetTag(id []byte, v interface{}) error
	//exists tx
	HasTag(id []byte) bool
	//delete tx
	DelTag(id []byte) error
	//事物处理
	Transaction(fn func(sdb DBImp) error) error
}

type TagKey [16]byte

//保存数据库中的结构
type TTagInfo struct {
	UID  []byte    `bson:"_id"`  //uid
	Ver  uint32    `bson:"ver"`  //版本 from tag
	Loc  []float64 `bson:"loc"`  //uint32-uint32 位置 from tag
	PKS  string    `bson:"pks"`  //标签公钥 from tag
	Keys [5]TagKey `bson:"keys"` //
	CTR  uint      `bson:"ctr"`  //ctr int
}

func LoadTagInfo(id TagUID, db DBImp) (*TTagInfo, error) {
	iv := &TTagInfo{}
	return iv, db.GetTag(id[:], iv)
}

func (tag TTagInfo) Save(db DBImp) error {
	return db.SetTag(tag.UID[:], tag)
}

func (tag TTagInfo) SetCtr(ctr uint, db DBImp) error {
	iv := SetValue{}
	iv["ctr"] = ctr
	return db.SetTag(tag.UID[:], iv)
}

var (
	client *mongo.Client = nil
	dbonce               = sync.Once{}
)

type mongoDBImp struct {
	context.Context
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

//save tans data
func (m *mongoDBImp) SetTag(id []byte, v interface{}) error {
	switch v.(type) {
	case IncValue:
		ds := bson.M{}
		for k, v := range v.(IncValue) {
			ds[k] = v
		}
		if len(ds) > 0 {
			_, err := m.tags().UpdateOne(m, bson.M{"_id": id}, bson.M{"$inc": ds})
			return err
		}
	case SetValue:
		ds := bson.M{}
		for k, v := range v.(SetValue) {
			ds[k] = v
		}
		if len(ds) > 0 {
			_, err := m.tags().UpdateOne(m, bson.M{"_id": id}, bson.M{"$set": ds})
			return err
		}
	default:
		opt := options.Update().SetUpsert(true)
		_, err := m.tags().UpdateOne(m, bson.M{"_id": id}, bson.M{"$set": v}, opt)
		return err
	}
	return nil
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
