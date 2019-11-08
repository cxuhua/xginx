package xginx

import (
	"bytes"
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"github.com/willf/bloom"

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
	state DBImp
	tags  DBImp
	blk   IDataStore
	rev   IDataStore
	once  sync.Once
	dir   string
	ctrs  map[TagUID]*uint32
}

func NewLevelDBStore(dir string) IStore {
	l := &leveldbstore{}
	l.Init(dir)
	return l
}

func (ss *leveldbstore) Sync() {
	ss.index.Sync()
	ss.state.Sync()
	ss.tags.Sync()
	ss.blk.Sync()
	ss.rev.Sync()
}

//新建索引数据库
func (ss *leveldbstore) newdb(subdir string) DBImp {
	dbdir := ss.dir + Separator + subdir
	opts := &opt.Options{
		Filter: filter.NewBloomFilter(10),
	}
	sdb, err := leveldb.OpenFile(dbdir, opts)
	if err != nil {
		panic(err)
	}
	return NewDB(sdb)
}

//新建存储数据库
func (ss *leveldbstore) newdata(ext string, subdir string, maxsiz int64) IDataStore {
	return &sstore{
		ext:   ext,
		files: map[uint32]*sfile{},
		size:  maxsiz,
		dir:   subdir,
		base:  ss.dir,
	}
}

func (ss *leveldbstore) Init(arg ...interface{}) {
	ss.once.Do(func() {
		if len(arg) < 1 {
			panic(errors.New("args error"))
		}
		ss.dir = arg[0].(string)
		ss.tags = ss.newdb("tag")
		ss.index = ss.newdb("blocks")
		ss.state = ss.newdb("state")
		ss.blk = ss.newdata(".blk", "blocks", 1024*1024*256)
		ss.rev = ss.newdata(".rev", "blocks", 1024*1024*32)
		ss.ctrs = map[TagUID]*uint32{}
		ss.blk.Init()
		ss.rev.Init()
	})
}

//新的值必须比旧的大
func (ss *leveldbstore) SetTagCtr(id TagUID, nv uint32) error {
	vptr, ok := ss.ctrs[id]
	if !ok {
		return errors.New("tags ctr info miss")
	}
	//确保原子性
	ov := atomic.LoadUint32(vptr)
	if nv <= ov || !atomic.CompareAndSwapUint32(vptr, ov, nv) {
		return errors.New("set ctr error")
	}
	//更新值到数据库
	ck := GetDBKey(CTR_PREFIX, id[:])
	cb := []byte{0, 0, 0, 0}
	Endian.PutUint32(cb, nv)
	return ss.tags.Put(ck, cb)
}

func (ss *leveldbstore) Close() {
	ss.index.Close()
	ss.state.Close()
	ss.tags.Close()
	ss.blk.Close()
	ss.rev.Close()
}

//加载所有的标签计数器
func (ss *leveldbstore) LoadAllTags(bf *bloom.BloomFilter) {
	log.Println("start load tags ctr info")
	rp := NewPrefix(CTR_PREFIX)
	iter := ss.tags.Iterator(rp)
	defer iter.Close()
	for iter.Next() {
		tk := iter.Key()
		v := Endian.Uint32(iter.Value())
		id := TagUID{}
		copy(id[:], tk[1:])
		ss.ctrs[id] = &v
		bf.Add(id[:])
	}
	log.Println("load", len(ss.ctrs), "tag ctr")
}

//获取最高块信息
func (ss *leveldbstore) GetBestValue() BestValue {
	bv := BestValue{}
	b, err := ss.state.Get(BestBlockKey)
	if err != nil {
		return InvalidBest
	}
	if err := bv.From(b); err != nil {
		return InvalidBest
	}
	return bv
}

//验证是否有验证成功的单元hash
func (ss *leveldbstore) HasUnitash(id HASH256) (HASH160, error) {
	pkh := HASH160{}
	ck, err := ss.tags.Get(HUNIT_PREFIX, id[:])
	if err != nil {
		return pkh, err
	}
	copy(pkh[:], ck)
	return pkh, nil
}

//打包确认后可移除单元hash
func (ss *leveldbstore) DelUnitHash(id HASH256) error {
	return ss.tags.Del(HUNIT_PREFIX, id[:])
}

//添加一个验证成功的单元hash
func (ss *leveldbstore) PutUnitHash(id HASH256, cli PKBytes) error {
	pkh := cli.Hash()
	return ss.tags.Put(HUNIT_PREFIX, id[:], pkh[:])
}

func (ss *leveldbstore) SaveTag(tag *TTagInfo) error {
	buf := &bytes.Buffer{}
	if err := tag.Encode(buf); err != nil {
		return err
	}
	batch := NewBatch()
	batch.Put(CTR_PREFIX, tag.UID[:], []byte{0, 0, 0, 0})
	batch.Put(TAG_PREFIX, tag.UID[:], buf.Bytes())
	return ss.tags.Write(batch)
}

func (ss *leveldbstore) LoadTagInfo(id TagUID) (*TTagInfo, error) {
	bb, err := ss.tags.Get(TAG_PREFIX, id[:])
	if err != nil {
		return nil, err
	}
	buf := bytes.NewReader(bb)
	t := &TTagInfo{}
	err = t.Decode(buf)
	return t, err
}

//索引数据库
func (ss *leveldbstore) Index() DBImp {
	return ss.index
}

//区块状态数据库
func (ss *leveldbstore) State() DBImp {
	return ss.state
}

//标签数据库
func (ss *leveldbstore) Tags() DBImp {
	return ss.tags
}

//区块数据文件
func (ss *leveldbstore) Blk() IDataStore {
	return ss.blk
}

//事物回退文件
func (ss *leveldbstore) Rev() IDataStore {
	return ss.rev
}
