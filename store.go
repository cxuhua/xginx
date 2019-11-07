package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/syndtr/goleveldb/leveldb/iterator"

	"github.com/willf/bloom"

	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	//index db
	dbptr  DBImp = nil
	dbonce sync.Once
	//china state db
	stptr  DBImp = nil
	stonce sync.Once
	//tags db
	tsptr  DBImp = nil
	tsonce sync.Once
	//系统路径分隔符
	Separator = string(os.PathSeparator)
)

type Batch struct {
	bptr *leveldb.Batch
}

func (b *Batch) Put(k []byte, v []byte) {
	b.bptr.Put(k, v)
}

func (b *Batch) Del(k []byte) {
	b.bptr.Delete(k)
}

func (b *Batch) Reset() {
	b.bptr.Reset()
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
		r: &util.Range{
			Start: s,
			Limit: l,
		},
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

type DBImp interface {
	Has(k []byte) bool
	Put(k []byte, v []byte) error
	Get(k []byte) ([]byte, error)
	Del(k []byte) error
	Write(b *Batch) error
	Close()
	Iterator(slice *Range) *Iterator
}

type leveldbimp struct {
	l *leveldb.DB
}

func NewDB(dbp *leveldb.DB) DBImp {
	return &leveldbimp{l: dbp}
}

func (db *leveldbimp) Iterator(slice *Range) *Iterator {
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return &Iterator{
		iter: db.l.NewIterator(slice.r, opts),
	}
}

func (db *leveldbimp) Close() {
	err := db.l.Close()
	if err != nil {
		log.Println("close db error", err)
	}
}

func (db *leveldbimp) Has(k []byte) bool {
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

func (db *leveldbimp) Put(k []byte, v []byte) error {
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.l.Put(k, v, opts)
}

func (db *leveldbimp) Get(k []byte) ([]byte, error) {
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	return db.l.Get(k, opts)
}

func (db *leveldbimp) Del(k []byte) error {
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.l.Delete(k, opts)
}

func (db *leveldbimp) Write(b *Batch) error {
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	return db.l.Write(b.bptr, opts)
}

//标签数据库
func TagDB() DBImp {
	tsonce.Do(func() {
		dir := conf.DataDir + Separator + "tags"
		opts := &opt.Options{
			Filter: filter.NewBloomFilter(10),
		}
		sdb, err := leveldb.OpenFile(dir, opts)
		if err != nil {
			panic(err)
		}
		tsptr = NewDB(sdb)
	})
	return tsptr
}

//链状态数据库
func StateDB() DBImp {
	stonce.Do(func() {
		dir := conf.DataDir + Separator + "state"
		opts := &opt.Options{
			Filter: filter.NewBloomFilter(10),
		}
		sdb, err := leveldb.OpenFile(dir, opts)
		if err != nil {
			panic(err)
		}
		stptr = NewDB(sdb)
	})
	return stptr
}

//索引数据库
func IndexDB() DBImp {
	dbonce.Do(func() {
		dir := conf.DataDir + Separator + "index"
		opts := &opt.Options{
			Filter: filter.NewBloomFilter(10),
		}
		sdb, err := leveldb.OpenFile(dir, opts)
		if err != nil {
			panic(err)
		}
		dbptr = NewDB(sdb)
		initBlockFiles()
	})
	return dbptr
}

var (
	CTR_PREFIX   = []byte{'C'} //标签计数器前缀 TagDB()
	TAG_PREFIX   = []byte{'T'} //标签信息前缀 TagDB()
	BLOCK_PREFIX = []byte{'B'} //块头信息前缀 IndexDB()
	HUNIT_PREFIX = []byte{'H'} //单元hash，签名hash存在说明数据验证通过签名 TagDB()
	UXS_PREFIX   = []byte{'U'} //uts 所在区块前缀 数据为区块id+（uts索引+uv索引) StateDB()存储
	TXS_PREFIX   = []byte{'T'} //tx 所在区块前缀 数据为区块id+（txs索引 StateDB()存储
	CBI_PREFIX   = []byte{'C'} //用户最后单元块id StateDB()存储
)

const (
	BestBlockKey = "BestBlockKey"
)

func GetBestBlock() HASH256 {
	id := HASH256{}
	b, err := IndexDB().Get([]byte(BestBlockKey))
	if err != nil {
		return conf.genesisId
	}
	copy(id[:], b)
	return id
}

func GetDBKey(p []byte, id ...[]byte) []byte {
	tk := []byte{}
	tk = append(tk, p...)
	for _, v := range id {
		tk = append(tk, v...)
	}
	return tk
}

//标签计数器
var (
	TagsCtr = map[TagUID]*uint32{}
)

//新的值必须比旧的大
func SetTagCtr(id TagUID, nv uint32) error {
	vptr, ok := TagsCtr[id]
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
	return TagDB().Put(ck, cb)
}

//加载所有的标签计数器
func LoadAllTags(tfb *bloom.BloomFilter) {
	log.Println("start load tags ctr info")
	rp := NewPrefix(CTR_PREFIX)
	iter := TagDB().Iterator(rp)
	defer iter.Close()
	for iter.Next() {
		tk := iter.Key()
		v := Endian.Uint32(iter.Value())
		id := TagUID{}
		copy(id[:], tk[1:])
		TagsCtr[id] = &v
		tfb.Add(id[:])
	}
	log.Println("load", len(TagsCtr), "tag ctr")
}

type TBMeta struct {
	Header BlockHeader //区块头
	Uts    VarUInt     //Units数量
	Txs    VarUInt     //tx数量
	FsId   VarUInt     //所在文件id 0000000.blk
	FsOff  VarUInt     //文件偏移
	FsLen  VarUInt     //块长度
	hasher HashCacher
}

func (h *TBMeta) Hash() HASH256 {
	if h, set := h.hasher.IsSet(); set {
		return h
	}
	buf := &bytes.Buffer{}
	if err := h.Encode(buf); err != nil {
		panic(err)
	}
	return h.hasher.Hash(buf.Bytes())
}

func (h TBMeta) Encode(w IWriter) error {
	if err := h.Header.Encode(w); err != nil {
		return err
	}
	if err := h.Uts.Encode(w); err != nil {
		return err
	}
	if err := h.Txs.Encode(w); err != nil {
		return err
	}
	if err := h.FsId.Encode(w); err != nil {
		return err
	}
	if err := h.FsOff.Encode(w); err != nil {
		return err
	}
	if err := h.FsLen.Encode(w); err != nil {
		return err
	}
	return nil
}

func (h *TBMeta) Decode(r IReader) error {
	if err := h.Header.Decode(r); err != nil {
		return err
	}
	if err := h.Uts.Decode(r); err != nil {
		return err
	}
	if err := h.Txs.Decode(r); err != nil {
		return err
	}
	if err := h.FsId.Decode(r); err != nil {
		return err
	}
	if err := h.FsOff.Decode(r); err != nil {
		return err
	}
	if err := h.FsLen.Decode(r); err != nil {
		return err
	}
	return nil
}

func (s *sstore) exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func (s *sstore) getpath() string {
	if s.path == "" {
		s.path = conf.DataDir + Separator + s.dir
		if !s.exists(s.path) {
			_ = os.Mkdir(s.path, os.ModePerm)
		}
	}
	return s.path
}

func (s *sstore) getlastfile() {
	blks := []string{}
	err := filepath.Walk(s.getpath(), func(spath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path.Ext(info.Name()) == s.ext {
			fs := strings.Split(info.Name(), ".")
			blks = append(blks, fs[0])
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	sid := "0"
	if len(blks) != 0 {
		sort.Slice(blks, func(i, j int) bool {
			return blks[i] > blks[j]
		})
		sid = blks[0]
	}
	id, err := strconv.ParseInt(sid, 10, 32)
	if err != nil {
		panic(err)
	}
	fi, err := os.Stat(s.fileIdPath(uint32(id)))
	if err == nil && fi.Size() >= s.size {
		id++
	}
	s.id = uint32(id)
}

type sstore struct {
	id    uint32            //当前文件id
	mu    sync.Mutex        //
	files map[uint32]*sfile //指针缓存
	ext   string            //扩展名称
	size  int64             //单个文件最大长度
	dir   string            //目录名称
	path  string            //存储全路径
}

var (
	//需要切换到下一个文件存储
	nextFileErr = errors.New("next file")
)

func sfileHeaderBytes() []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, Endian, []byte(conf.Flags))
	_ = binary.Write(buf, Endian, conf.Ver)
	return buf.Bytes()
}

type sfile struct {
	mu sync.RWMutex
	*os.File
	size int64
}

func (f *sfile) flush() {
	if err := f.Sync(); err != nil {
		log.Println(f.Name(), "fsync error", err)
	}
}

//读取数据
func (s *sfile) read(off uint32, b []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rl := uint32(len(b))
	pl := uint32(0)
	for pl < rl {
		_, err := s.Seek(int64(off+pl), io.SeekStart)
		if err != nil {
			return err
		}
		cl, err := s.Read(b[pl:])
		if err != nil {
			return err
		}
		pl += uint32(cl)
	}
	return nil
}

//写入数据，返回数据偏移
func (f *sfile) write(b []byte) (uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	wl := len(b)
	pl := 0
	off := int(fi.Size())
	for pl < wl {
		cl, err := f.Write(b[pl:])
		if err != nil {
			return 0, err
		}
		pl += cl
	}
	if off+wl > int(f.size) {
		f.flush()
		return uint32(off), nextFileErr
	}
	return uint32(off), nil
}

func (s sstore) newFile(id uint32, max int64) (*sfile, error) {
	f, err := os.OpenFile(s.fileIdPath(id), os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}
	return &sfile{File: f, size: max}, nil
}

func (s sstore) fileIdPath(id uint32) string {
	return fmt.Sprintf("%s%s%06d%s", s.getpath(), Separator, id, s.ext)
}

func (f *sstore) Id() uint32 {
	return atomic.LoadUint32(&f.id)
}

func (s *sstore) openfile(id uint32) (*sfile, error) {
	hbytes := sfileHeaderBytes()
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.files[id]; ok {
		return f, nil
	}
	sf, err := s.newFile(id, s.size)
	if err != nil {
		return nil, err
	}
	fi, err := sf.Stat()
	if err != nil {
		_ = sf.Close()
		return nil, err
	}
	fsiz := int(fi.Size())
	if fsiz >= len(hbytes) {
		s.files[id] = sf
		return sf, nil
	}
	pos, err := sf.write(hbytes[fsiz:])
	if pos != uint32(fsiz) || err != nil {
		_ = sf.Close()
		return nil, err
	}
	sf.flush()
	s.files[id] = sf
	return sf, nil
}

//读取数据
func (s *sstore) read(id uint32, off uint32, b []byte) error {
	f, err := s.openfile(id)
	if err != nil {
		return err
	}
	return f.read(off, b)
}

//写入数据，返回数据便宜
func (s *sstore) write(b []byte) (uint32, error) {
	f, err := s.openfile(s.id)
	if err != nil {
		return 0, err
	}
	pos, err := f.write(b)
	if err == nextFileErr {
		atomic.AddUint32(&s.id, 1)
		return pos, nil
	}
	if err != nil {
		return 0, err
	}
	return pos, err
}

func (s *sstore) closefile(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.files[id]; ok {
		_ = f.Close()
		delete(s.files, id)
	}
}

func (s sstore) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.files {
		_ = v.Close()
	}
}

var (
	blk = &sstore{
		ext:   ".blk",
		files: map[uint32]*sfile{},
		size:  1024 * 1024 * 256,
		dir:   "blocks",
	}
	rev = &sstore{
		ext:   ".rev",
		files: map[uint32]*sfile{},
		size:  1024 * 1024 * 32,
		dir:   "blocks",
	}
)

//.blk保存块数据
//初始化db
func initBlockFiles() {
	blk.getlastfile()
	rev.getlastfile()
}

func dbclose() {
	IndexDB().Close()
	StateDB().Close()
	TagDB().Close()
	blk.close()
	rev.close()
}

//验证是否有验证成功的单元hash
func HasUnitash(id HASH256) (HASH160, error) {
	hk := GetDBKey(HUNIT_PREFIX, id[:])
	pkh := HASH160{}
	ck, err := TagDB().Get(hk)
	if err != nil {
		return pkh, err
	}
	copy(pkh[:], ck)
	return pkh, nil
}

//打包确认后可移除单元hash

func DelUnitHash(id HASH256) error {
	hk := GetDBKey(HUNIT_PREFIX, id[:])
	return TagDB().Del(hk)
}

//添加一个验证成功的单元hash
func PutUnitHash(id HASH256, cli PKBytes) error {
	hk := GetDBKey(HUNIT_PREFIX, id[:])
	pkh := cli.Hash()
	return TagDB().Put(hk, pkh[:])
}

//标签数据
type TTagKey [16]byte

//保存数据库中的结构
type TTagInfo struct {
	UID  TagUID     //uid
	Ver  uint32     //版本 from tag
	Loc  Location   //uint32-uint32 位置 from tag
	ASV  uint8      //分配比例
	PKH  HASH160    //所属公钥HASH160
	Keys [5]TTagKey //ntag424 5keys
}

func (t *TTagInfo) Decode(r IReader) error {
	if err := binary.Read(r, Endian, t.UID[:]); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &t.Ver); err != nil {
		return err
	}
	if err := t.Loc.Decode(r); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &t.ASV); err != nil {
		return err
	}
	if err := t.PKH.Decode(r); err != nil {
		return err
	}
	for i, _ := range t.Keys {
		err := binary.Read(r, Endian, &t.Keys[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (t TTagInfo) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, t.UID[:]); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, t.Ver); err != nil {
		return err
	}
	if err := t.Loc.Encode(w); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, t.ASV); err != nil {
		return err
	}
	if err := t.PKH.Encode(w); err != nil {
		return err
	}
	for _, v := range t.Keys {
		err := binary.Write(w, Endian, v[:])
		if err != nil {
			return err
		}
	}
	return nil
}

func LoadTagInfo(id TagUID) (*TTagInfo, error) {
	tk := GetDBKey(TAG_PREFIX, id[:])
	bb, err := TagDB().Get(tk)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewReader(bb)
	t := &TTagInfo{}
	err = t.Decode(buf)
	return t, err
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

func (tag TTagInfo) Save() error {
	batch := NewBatch()
	tk := GetDBKey(TAG_PREFIX, tag.UID[:])
	buf := &bytes.Buffer{}
	if err := tag.Encode(buf); err != nil {
		return err
	}
	ck := GetDBKey(CTR_PREFIX, tag.UID[:])
	cb := []byte{0, 0, 0, 0}
	batch.Put(ck, cb)
	batch.Put(tk, buf.Bytes())
	return TagDB().Write(batch)
}
