package main

import "github.com/cxuhua/xginx"

//分类数据存储,不考虑排序,只根据分类和key存储
//存储 xginx.ISerializable 数据
type ITypeDB interface {
	//添加数据
	Put(typ string, k []byte, v xginx.ISerializable) error
	//关闭数据库
	Close()
	//删除数据
	Del(typ string, k []byte) error
	//获取指定数据
	Get(typ string, k []byte, v xginx.ISerializable) error
	//遍历分类书,如果返回false,中断遍历
	Each(typ string, vptr xginx.ISerializable, fn func(k []byte, v xginx.ISerializable) bool) error
}

type typedb struct {
	dbptr xginx.DBImp
}

func NewTypeDB(dir string) (ITypeDB, error) {
	db, err := xginx.NewDBImp(dir)
	if err != nil {
		return nil, err
	}
	return &typedb{
		dbptr: db,
	}, nil
}

func (db *typedb) Sync() {
	db.dbptr.Sync()
}

func (db *typedb) Close() {
	db.dbptr.Close()
}

func (db *typedb) Put(typ string, k []byte, v xginx.ISerializable) error {
	w := xginx.NewWriter()
	err := v.Encode(w)
	if err != nil {
		return err
	}
	return db.dbptr.Put([]byte(typ), k, w.Bytes())
}

func (db *typedb) Del(typ string, k []byte) error {
	return db.dbptr.Del([]byte(typ), k)
}

func (db *typedb) Get(typ string, k []byte, v xginx.ISerializable) error {
	bb, err := db.dbptr.Get([]byte(typ), k)
	if err != nil {
		return err
	}
	r := xginx.NewReader(bb)
	return v.Decode(r)
}

func (db *typedb) Each(typ string, vptr xginx.ISerializable, fn func(k []byte, v xginx.ISerializable) bool) error {
	iter := db.dbptr.Iterator(xginx.NewPrefix([]byte(typ)))
	defer iter.Close()
	for iter.Next() {
		key := iter.Key()
		k := key[len(typ):]
		r := xginx.NewReader(iter.Value())
		err := vptr.Decode(r)
		if err != nil {
			return err
		}
		if !fn(k, vptr) {
			break
		}
	}
	return nil
}
