package xginx

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client *mongo.Client = nil
	dbonce               = sync.Once{}
)

type mongoDBStore struct {
	context.Context
}

func (m *mongoDBStore) collection(t string) *mongo.Collection {
	return m.database().Collection(t)
}

func (m *mongoDBStore) txs() *mongo.Collection {
	return m.database().Collection("txs")
}

func (m *mongoDBStore) blocks() *mongo.Collection {
	return m.database().Collection("blocks")
}

func (m *mongoDBStore) units() *mongo.Collection {
	return m.database().Collection("units")
}

func (m *mongoDBStore) tags() *mongo.Collection {
	return m.database().Collection("tags")
}

func (m *mongoDBStore) client() *mongo.Client {
	return m.Context.(mongo.SessionContext).Client()
}

func (m *mongoDBStore) database() *mongo.Database {
	return m.client().Database("xginx")
}

//设置计数器，必须比数据库中的大
func (m *mongoDBStore) SetCtr(id []byte, ctr uint) error {
	c := bson.M{"_id": id, "ctr": bson.M{"$lt": ctr}}
	d := bson.M{"$set": bson.M{"ctr": ctr}}
	res := m.tags().FindOneAndUpdate(m, c, d)
	return res.Err()
}

func NewDBImp(ctx context.Context) DBImp {
	return &mongoDBStore{Context: ctx}
}

//delete data
func (m *mongoDBStore) DelTag(id []byte) error {
	_, err := m.tags().DeleteOne(m, bson.M{"_id": id})
	if err != nil {
		return err
	}
	return err
}

//get tx data
func (m *mongoDBStore) GetTag(id []byte, v interface{}) error {
	ret := m.tags().FindOne(m, bson.M{"_id": id})
	if err := ret.Err(); err != nil {
		return err
	}
	return ret.Decode(v)
}

//check tx exists
func (m *mongoDBStore) HasTag(id []byte) bool {
	ret := m.tags().FindOne(m, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"_id": 1}))
	return ret.Err() == nil
}

func (m *mongoDBStore) set(t string, id []byte, v interface{}) error {
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

//删除块
func (m *mongoDBStore) DelUnit(id []byte) error {
	_, err := m.units().DeleteOne(m, bson.M{"_id": id})
	if err != nil {
		return err
	}
	return err
}

//获取块
func (m *mongoDBStore) GetUnit(id []byte, v interface{}) error {
	ret := m.units().FindOne(m, bson.M{"_id": id})
	if err := ret.Err(); err != nil {
		return err
	}
	return ret.Decode(v)
}

//存在
func (m *mongoDBStore) HasUnit(id []byte) bool {
	ret := m.units().FindOne(m, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"_id": 1}))
	return ret.Err() == nil
}

func (m *mongoDBStore) SetUnit(id []byte, v interface{}) error {
	return m.set("units", id, v)
}

//save tans data
func (m *mongoDBStore) SetTag(id []byte, v interface{}) error {
	return m.set("tags", id, v)
}

func (m *mongoDBStore) Transaction(fn func(sdb DBImp) error) error {
	sess := m.Context.(mongo.SessionContext)
	_, err := sess.WithTransaction(m, func(sctx mongo.SessionContext) (i interface{}, e error) {
		return nil, fn(NewDBImp(sctx))
	})
	return err
}

func initMongoDB(ctx context.Context) *mongo.Client {
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

type mongoDBSession struct {
}

func (m *mongoDBSession) UseSession(ctx context.Context, fn func(db DBImp) error) error {
	client = initMongoDB(ctx)
	return client.UseSession(ctx, func(sess mongo.SessionContext) error {
		return fn(NewDBImp(sess))
	})
}
