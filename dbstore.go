package xginx

import (
	"errors"
	"log"
	"runtime"
	"sync"

	"github.com/syndtr/goleveldb/leveldb/filter"

	"github.com/syndtr/goleveldb/leveldb/iterator"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

//Batch 批量写入
type Batch struct {
	db DBImp
	bt *leveldb.Batch
	rb *Batch //事务回退日志
}

//GetRev 获取写入时的回退日志
func (b *Batch) GetRev() *Batch {
	return b.rb
}

//NewRev 设置回退并返回
func (b *Batch) NewRev() *Batch {
	b.rb = b.db.NewBatch()
	return b.rb
}

//Load 加载日志
func (b *Batch) Load(d []byte) error {
	return b.bt.Load(d)
}

//Dump 导出日志
func (b *Batch) Dump() []byte {
	return b.bt.Dump()
}

//Len 日志长度
func (b *Batch) Len() int {
	return b.bt.Len()
}

//Put 最后一个是数据，前面都是key
func (b *Batch) Put(ks ...[]byte) {
	k, v := getDBKeyValue(ks...)
	if b.rb != nil {
		b.rb.Del(k)
	}
	b.bt.Put(k, v)
}

//Del 删除
func (b *Batch) Del(ks ...[]byte) {
	k := getDBKey(ks...)
	b.bt.Delete(k)
}

//GetBatch 获取leveldb日志
func (b *Batch) GetBatch() *leveldb.Batch {
	return b.bt
}

//Reset 重置
func (b *Batch) Reset() {
	b.bt.Reset()
}

func loadBatch(d []byte) (*Batch, error) {
	bp := newBatch()
	err := bp.Load(d)
	return bp, err
}

func newBatch() *Batch {
	return &Batch{
		bt: &leveldb.Batch{},
	}
}

//Range 查询范围
type Range struct {
	r *util.Range
}

//NewRange 创建一个范围
func NewRange(s []byte, l []byte) *Range {
	return &Range{
		r: &util.Range{Start: s, Limit: l},
	}
}

//NewPrefix 创建按前缀的范围
func NewPrefix(p []byte) *Range {
	return &Range{
		r: util.BytesPrefix(p),
	}
}

//Iterator 查询迭代器
type Iterator struct {
	iter iterator.Iterator
	rptr *Range
}

//Close 关闭
func (it *Iterator) Close() {
	it.iter.Release()
}

//Next 下一个
func (it *Iterator) Next() bool {
	return it.iter.Next()
}

//First 第一个
func (it *Iterator) First() bool {
	return it.iter.First()
}

//Prev 上一个
func (it *Iterator) Prev() bool {
	return it.iter.Prev()
}

//Last 最后一个
func (it *Iterator) Last() bool {
	return it.iter.Last()
}

//Key 当前Key
func (it *Iterator) Key() []byte {
	return it.iter.Key()
}

//Value 当前值
func (it *Iterator) Value() []byte {
	return it.iter.Value()
}

//Valid 是否有效
func (it *Iterator) Valid() bool {
	return it.iter.Valid()
}

//Seek 定位到key
func (it *Iterator) Seek(k []byte) bool {
	return it.iter.Seek(k)
}

type leveldbimp struct {
	lptr *leveldb.DB
}

//NewDB 创建基于leveldb的数据库接口
func NewDB(dbp *leveldb.DB) DBImp {
	return &leveldbimp{lptr: dbp}
}

//LoadBatch 加载日志
func (db *leveldbimp) LoadBatch(d []byte) (*Batch, error) {
	bt, err := loadBatch(d)
	if err != nil {
		return nil, err
	}
	bt.db = db
	return bt, nil
}

//NewBatch 创建日志
func (db *leveldbimp) NewBatch() *Batch {
	bt := newBatch()
	bt.db = db
	return bt
}

//Iterator 创建迭代器
func (db *leveldbimp) Iterator(slice ...*Range) *Iterator {
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	var rptr *util.Range = nil
	if len(slice) > 0 {
		rptr = slice[0].r
	}
	iter := &Iterator{
		iter: db.lptr.NewIterator(rptr, opts),
	}
	if len(slice) > 0 {
		iter.rptr = slice[0]
	}
	return iter
}

func (db *leveldbimp) Compact(r *Range) error {
	return db.lptr.CompactRange(*r.r)
}

func (db *leveldbimp) Close() {
	err := db.lptr.Close()
	if err != nil {
		log.Println("close db error", err)
	}
}

func (db *leveldbimp) Has(ks ...[]byte) (bool, error) {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.lptr.Has(k, opts)
}

func (db *leveldbimp) Put(ks ...[]byte) error {
	k, v := getDBKeyValue(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.lptr.Put(k, v, opts)
}

func (db *leveldbimp) Get(ks ...[]byte) ([]byte, error) {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.lptr.Get(k, opts)
}

func (db *leveldbimp) Del(ks ...[]byte) error {
	k := getDBKey(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.lptr.Delete(k, opts)
}

func (db *leveldbimp) Transaction() (TRImp, error) {
	tr, err := db.lptr.OpenTransaction()
	if err != nil {
		return nil, err
	}
	return &leveldbtr{tr: tr}, nil
}

func (db *leveldbimp) SizeOf(r []*Range) ([]int64, error) {
	rgs := []util.Range{}
	for _, v := range r {
		rgs = append(rgs, *v.r)
	}
	return db.lptr.SizeOf(rgs)
}

func (db *leveldbimp) Sync() {
	tr, err := db.lptr.OpenTransaction()
	if err == nil {
		err = tr.Commit()
	}
	if err != nil {
		LogError("sync error", err)
	}
}

func (db *leveldbimp) Write(b *Batch, sync ...bool) error {
	opts := &opt.WriteOptions{
		Sync:         len(sync) > 0 && sync[0],
		NoWriteMerge: len(sync) > 1 && sync[1],
	}
	return db.lptr.Write(b.GetBatch(), opts)
}

//
type leveldbtr struct {
	tr *leveldb.Transaction
}

func (db *leveldbtr) Has(ks ...[]byte) (bool, error) {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.tr.Has(k, opts)
}

func (db *leveldbtr) Put(ks ...[]byte) error {
	k, v := getDBKeyValue(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.tr.Put(k, v, opts)
}

func (db *leveldbtr) Get(ks ...[]byte) ([]byte, error) {
	k := getDBKey(ks...)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.tr.Get(k, opts)
}

func (db *leveldbtr) Del(ks ...[]byte) error {
	k := getDBKey(ks...)
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.tr.Delete(k, opts)
}

func (db *leveldbtr) Write(b *Batch, sync ...bool) error {
	opts := &opt.WriteOptions{
		Sync:         len(sync) > 0 && sync[0],
		NoWriteMerge: len(sync) > 1 && sync[1],
	}
	return db.tr.Write(b.bt, opts)
}

func (db *leveldbtr) Iterator(slice ...*Range) *Iterator {
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

func (db *leveldbtr) Commit() error {
	return db.tr.Commit()
}

func (db *leveldbtr) Discard() {
	db.tr.Discard()
}

type leveldbstore struct {
	index DBImp       //区块索引
	blk   IChunkStore //区块存储器
	rev   IChunkStore //回退日志
	once  sync.Once   //
	dir   string      //存储路径
}

//NewDBImp 创建数据库接口
func NewDBImp(dir string, opts ...*opt.Options) (DBImp, error) {
	dopt := &opt.Options{
		OpenFilesCacheCapacity: 16,
		BlockCacheCapacity:     16 / 2 * opt.MiB,
		WriteBuffer:            16 / 4 * opt.MiB,
		Filter:                 filter.NewBloomFilter(10),
	}
	if len(opts) > 0 {
		dopt = opts[0]
	}
	sdb, err := leveldb.OpenFile(dir, dopt)
	if err != nil {
		return nil, err
	}
	return NewDB(sdb), nil
}

//NewLevelDBStore 创建leveldb存储器
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

//新建存储数据库
func (ss *leveldbstore) newdata(ext string, maxsiz int64) IChunkStore {
	fs := &sstore{
		ext:   ext,
		files: map[uint32]*sfile{},
		size:  maxsiz,
		dir:   ss.dir,
	}
	runtime.SetFinalizer(fs, (*sstore).Close)
	return fs
}

func (ss *leveldbstore) Init(arg ...interface{}) {
	ss.once.Do(func() {
		if len(arg) < 1 {
			panic(errors.New("args error"))
		}
		ss.dir = arg[0].(string)
		if db, err := NewDBImp(ss.dir); err != nil {
			panic(err)
		} else {
			ss.index = db
		}
		ss.blk = ss.newdata(".blk", 1024*1024*256)
		if err := ss.blk.Init(); err != nil {
			panic(err)
		}
		ss.rev = ss.newdata(".rev", 1024*1024*64)
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

//扩展存储
func (ss *leveldbstore) Blk() IChunkStore {
	return ss.blk
}

//事物回退文件
func (ss *leveldbstore) Rev() IChunkStore {
	return ss.rev
}
