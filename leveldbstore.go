package xginx

import (
	"errors"
	"log"
	"sync"

	"github.com/syndtr/goleveldb/leveldb/filter"

	"github.com/syndtr/goleveldb/leveldb/iterator"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type Batch struct {
	bptr *leveldb.Batch
	rb   *Batch //事务回退日志
}

func (b *Batch) GetRev() *Batch {
	return b.rb
}

func (b *Batch) SetRev(r *Batch) *Batch {
	b.rb = r
	return r
}

func (b *Batch) Load(d []byte) error {
	return b.bptr.Load(d)
}

func (b *Batch) Dump() []byte {
	return b.bptr.Dump()
}

func (b *Batch) Len() int {
	return b.bptr.Len()
}

//最后一个是数据，前面都是key
func (b *Batch) Put(ks ...[]byte) {
	k, v := getDBKeyValue(ks...)
	if b.rb != nil {
		b.rb.Del(k)
	}
	b.bptr.Put(k, v)
}

func (b *Batch) Del(ks ...[]byte) {
	k := getDBKey(ks...)
	b.bptr.Delete(k)
}

func (b *Batch) Reset() {
	b.bptr.Reset()
}

func LoadBatch(d []byte) (*Batch, error) {
	bp := NewBatch()
	return bp, bp.Load(d)
}

func NewBatch() *Batch {
	return &Batch{
		bptr: &leveldb.Batch{},
	}
}

type Range struct {
	r *util.Range
}

func NewRange(s []byte, l []byte) *Range {
	return &Range{
		r: &util.Range{Start: s, Limit: l},
	}
}

func NewPrefix(p []byte) *Range {
	return &Range{
		r: util.BytesPrefix(p),
	}
}

type Iterator struct {
	iter iterator.Iterator
}

func (it *Iterator) Close() {
	it.iter.Release()
}

func (it *Iterator) Next() bool {
	return it.iter.Next()
}

func (it *Iterator) First() bool {
	return it.iter.First()
}
func (it *Iterator) Prev() bool {
	return it.iter.Prev()
}

func (it *Iterator) Last() bool {
	return it.iter.Last()
}

func (it *Iterator) Key() []byte {
	return it.iter.Key()
}

func (it *Iterator) Value() []byte {
	return it.iter.Value()
}

func (it *Iterator) Valid() bool {
	return it.iter.Valid()
}

func (it *Iterator) Seek(k []byte) bool {
	return it.iter.Seek(k)
}

type leveldbimp struct {
	l *leveldb.DB
}

func NewDB(dbp *leveldb.DB) DBImp {
	return &leveldbimp{l: dbp}
}

func (db *leveldbimp) Iterator(slice ...*Range) *Iterator {
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	var rptr *util.Range = nil
	if len(slice) > 0 {
		rptr = slice[0].r
	}
	return &Iterator{
		iter: db.l.NewIterator(rptr, opts),
	}
}

func (db *leveldbimp) Close() {
	err := db.l.Close()
	if err != nil {
		log.Println("close db error", err)
	}
}

func (db *leveldbimp) Has(ks ...[]byte) bool {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	b, err := db.l.Has(k, opts)
	if err != nil {
		panic(err)
	}
	return b
}

func (db *leveldbimp) Put(ks ...[]byte) error {
	k, v := getDBKeyValue(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.l.Put(k, v, opts)
}

func (db *leveldbimp) Get(ks ...[]byte) ([]byte, error) {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.l.Get(k, opts)
}

func (db *leveldbimp) Del(ks ...[]byte) error {
	k := getDBKey(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.l.Delete(k, opts)
}

func (db *leveldbimp) Transaction() (TRImp, error) {
	tr, err := db.l.OpenTransaction()
	if err != nil {
		return nil, err
	}
	return &leveldbtrimp{tr: tr}, nil
}

func (db *leveldbimp) Sync() {
	tr, err := db.l.OpenTransaction()
	if err == nil {
		_ = tr.Commit()
	}
}

func (db *leveldbimp) Write(b *Batch) error {
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.l.Write(b.bptr, opts)
}

type leveldbtrimp struct {
	tr *leveldb.Transaction
}

func (db *leveldbtrimp) Has(ks ...[]byte) (bool, error) {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.tr.Has(k, opts)
}

func (db *leveldbtrimp) Put(ks ...[]byte) error {
	k, v := getDBKeyValue(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.tr.Put(k, v, opts)
}

func (db *leveldbtrimp) Get(ks ...[]byte) ([]byte, error) {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.tr.Get(k, opts)
}

func (db *leveldbtrimp) Del(ks ...[]byte) error {
	k := getDBKey(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.tr.Delete(k, opts)
}

func (db *leveldbtrimp) Write(b *Batch) error {
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.tr.Write(b.bptr, opts)
}

func (db *leveldbtrimp) Iterator(slice ...*Range) *Iterator {
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	var rptr *util.Range = nil
	if len(slice) > 0 {
		rptr = slice[0].r
	}
	return &Iterator{
		iter: db.tr.NewIterator(rptr, opts),
	}
}

func (db *leveldbtrimp) Commit() error {
	return db.tr.Commit()
}

func (db *leveldbtrimp) Discard() {
	db.tr.Discard()
}

type leveldbstore struct {
	index DBImp
	blk   IChunkStore
	rev   IChunkStore
	once  sync.Once
	dir   string
}

func NewLevelDBStore(dir string) IBlkStore {
	l := &leveldbstore{}
	l.Init(dir)
	return l
}

func (ss *leveldbstore) Sync() {
	ss.index.Sync()
	ss.blk.Sync()
	ss.rev.Sync()
}

//新建索引数据库
func (ss *leveldbstore) newdb() (DBImp, error) {
	opts := &opt.Options{
		Filter: filter.NewBloomFilter(10),
	}
	sdb, err := leveldb.OpenFile(ss.dir, opts)
	if err != nil {
		return nil, err
	}
	return NewDB(sdb), nil
}

//新建存储数据库
func (ss *leveldbstore) newdata(ext string, maxsiz int64) IChunkStore {
	return &sstore{
		ext:   ext,
		files: map[uint32]*sfile{},
		size:  maxsiz,
		dir:   ss.dir,
	}
}

func (ss *leveldbstore) Init(arg ...interface{}) {
	ss.once.Do(func() {
		if len(arg) < 1 {
			panic(errors.New("args error"))
		}
		ss.dir = arg[0].(string)
		if db, err := ss.newdb(); err != nil {
			panic(err)
		} else {
			ss.index = db
		}
		ss.blk = ss.newdata(".blk", 1024*1024*256)
		if err := ss.blk.Init(); err != nil {
			panic(err)
		}
		ss.rev = ss.newdata(".rev", 1024*1024*32)
		if err := ss.rev.Init(); err != nil {
			panic(err)
		}
	})
}

func (ss *leveldbstore) Close() {
	ss.index.Close()
	ss.blk.Close()
	ss.rev.Close()
}

//索引数据库
func (ss *leveldbstore) Index() DBImp {
	return ss.index
}

//区块数据文件
func (ss *leveldbstore) Blk() IChunkStore {
	return ss.blk
}

//事物回退文件
func (ss *leveldbstore) Rev() IChunkStore {
	return ss.rev
}
