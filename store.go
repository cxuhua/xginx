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

	"github.com/willf/bloom"

	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	dbptr *leveldb.DB = nil
	once  sync.Once
)

var (
	CTR_PREFIX   = []byte{'C'} //标签计数器前缀
	TAG_PREFIX   = []byte{'T'} //标签信息前缀
	BLOCK_PREFIX = []byte{'B'} //块头信息前缀
	HUNIT_PREFIX = []byte{'H'} //单元hash，签名hash存在说明数据验证通过签名
)

const (
	BestBlockKey = "BestBlockKey"
)

func GetBestBlock() HASH256 {
	id := HASH256{}
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	b, err := DB().Get([]byte(BestBlockKey), opts)
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
	opts := &opt.WriteOptions{
		NoWriteMerge: true,
		Sync:         false,
	}
	return DB().Put(ck, cb, opts)
}

//加载所有的标签
func LoadAllTags(tfb *bloom.BloomFilter) {
	log.Println("start load tags ctr info")
	rp := util.BytesPrefix(CTR_PREFIX)
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	iter := DB().NewIterator(rp, opts)
	defer iter.Release()
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
	Ver     uint32  //block ver
	Prev    HASH256 //pre block hash
	Merkle  HASH256 //txs Merkle tree hash + Units hash
	Time    uint32  //时间戳
	Bits    uint32  //难度
	Nonce   uint32  //随机值
	Uts     VarUInt //Units数量
	Txs     VarUInt //tx数量
	FileId  VarUInt //所在文件id 0000000.blk
	FileOff VarUInt //文件偏移
	FileLen VarUInt //块长度
}

func (h *TBMeta) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, h.Ver); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, h.Prev[:]); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, h.Merkle[:]); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, h.Time); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, h.Bits); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, h.Nonce); err != nil {
		return err
	}
	if err := h.Uts.Encode(w); err != nil {
		return err
	}
	if err := h.Txs.Encode(w); err != nil {
		return err
	}
	if err := h.FileId.Encode(w); err != nil {
		return err
	}
	if err := h.FileOff.Encode(w); err != nil {
		return err
	}
	if err := h.FileLen.Encode(w); err != nil {
		return err
	}
	return nil
}

func (h *TBMeta) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &h.Ver); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &h.Prev); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &h.Merkle); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &h.Time); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &h.Bits); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &h.Nonce); err != nil {
		return err
	}
	if err := h.Uts.Decode(r); err != nil {
		return err
	}
	if err := h.Txs.Decode(r); err != nil {
		return err
	}
	if err := h.FileId.Decode(r); err != nil {
		return err
	}
	if err := h.FileId.Decode(r); err != nil {
		return err
	}
	if err := h.FileLen.Decode(r); err != nil {
		return err
	}
	return nil
}

func (s *sstore) getlastfile() {
	blks := []string{}
	err := filepath.Walk(conf.DataDir, func(spath string, info os.FileInfo, err error) error {
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
}

var (
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
	return fmt.Sprintf("%s%c%06d%s", conf.DataDir, os.PathSeparator, id, s.ext)
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
	}
	rev = &sstore{
		ext:   ".rev",
		files: map[uint32]*sfile{},
		size:  1024 * 1024 * 32,
	}
)

//.blk保存块数据
//初始化db
func initdb(db *leveldb.DB) {
	blk.getlastfile()
	rev.getlastfile()
}

func dbclose() {
	_ = DB().Close()
	blk.close()
	rev.close()
}

func DB() *leveldb.DB {
	once.Do(func() {
		bf := filter.NewBloomFilter(10)
		opts := &opt.Options{
			Filter: bf,
		}
		sdb, err := leveldb.OpenFile(conf.DataDir, opts)
		if err != nil {
			panic(err)
		}
		dbptr = sdb
		initdb(dbptr)
	})
	return dbptr
}

//验证是否有验证成功的单元hash
func HasUnitash(id HASH256) (HASH160, error) {
	hk := GetDBKey(HUNIT_PREFIX, id[:])
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	pkh := HASH160{}
	ck, err := DB().Get(hk, opts)
	if err != nil {
		return pkh, err
	}
	copy(pkh[:], ck)
	return pkh, nil
}

//添加一个验证成功的单元hash
func PutUnitHash(id HASH256, cli PKBytes) error {
	hk := GetDBKey(HUNIT_PREFIX, id[:])
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         false,
	}
	pkh := cli.Hash()
	return DB().Put(hk, pkh[:], opts)
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
	opts := &opt.ReadOptions{
		DontFillCache: false,
		Strict:        opt.StrictReader,
	}
	bb, err := DB().Get(tk, opts)
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
	batch := &leveldb.Batch{}
	tk := GetDBKey(TAG_PREFIX, tag.UID[:])
	buf := &bytes.Buffer{}
	if err := tag.Encode(buf); err != nil {
		return err
	}
	ck := GetDBKey(CTR_PREFIX, tag.UID[:])
	cb := []byte{0, 0, 0, 0}
	batch.Put(ck, cb)
	batch.Put(tk, buf.Bytes())
	opts := &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         true,
	}
	return DB().Write(batch, opts)
}
