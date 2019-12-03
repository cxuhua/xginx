package xginx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

//文件数据状态
type BlkChunk struct {
	Id  VarUInt //数据所在文件id
	Off VarUInt //数据所在位置
	Len VarUInt //数据长度
}

func (f BlkChunk) HasData() bool {
	return f.Id >= 0 && f.Off >= 0 && f.Len > 0
}

func (f *BlkChunk) Decode(r IReader) error {
	if err := f.Id.Decode(r); err != nil {
		return err
	}
	if err := f.Off.Decode(r); err != nil {
		return err
	}
	if err := f.Len.Decode(r); err != nil {
		return err
	}
	return nil
}

func (f BlkChunk) Encode(w IWriter) error {
	if err := f.Id.Encode(w); err != nil {
		return err
	}
	if err := f.Off.Encode(w); err != nil {
		return err
	}
	if err := f.Len.Encode(w); err != nil {
		return err
	}
	return nil
}

type TBMeta struct {
	BlockHeader          //区块头
	Txs         VarUInt  //tx数量
	Blk         BlkChunk //数据状态
	Rev         BlkChunk //日志回退
	hasher      HashCacher
}

// 是否有区块数据
func (h TBMeta) HasBlk() bool {
	return h.Blk.HasData()
}

// 是否有回退数据
func (h TBMeta) HasRev() bool {
	return h.Rev.HasData()
}

func (h TBMeta) String() string {
	id, _ := h.BlockHeader.ID()
	return id.String()
}

func (h *TBMeta) Hash() HASH256 {
	if h, set := h.hasher.IsSet(); set {
		return h
	}
	buf := NewWriter()
	if err := h.Encode(buf); err != nil {
		panic(err)
	}
	return h.hasher.Hash(buf.Bytes())
}

func (h TBMeta) Bytes() ([]byte, error) {
	buf := NewWriter()
	err := h.Encode(buf)
	return buf.Bytes(), err
}

func (h TBMeta) Encode(w IWriter) error {
	if err := h.BlockHeader.Encode(w); err != nil {
		return err
	}
	if err := h.Txs.Encode(w); err != nil {
		return err
	}
	if err := h.Blk.Encode(w); err != nil {
		return err
	}
	if err := h.Rev.Encode(w); err != nil {
		return err
	}
	return nil
}

func (h *TBMeta) Decode(r IReader) error {
	if err := h.BlockHeader.Decode(r); err != nil {
		return err
	}
	if err := h.Txs.Decode(r); err != nil {
		return err
	}
	if err := h.Blk.Decode(r); err != nil {
		return err
	}
	if err := h.Rev.Decode(r); err != nil {
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

func (s *sstore) Sync(id ...uint32) {
	if len(id) == 0 {
		for _, f := range s.files {
			_ = f.Sync()
		}
	} else {
		for _, key := range id {
			f, ok := s.files[key]
			if !ok {
				continue
			}
			_ = f.Sync()
		}
	}
}

func (s *sstore) Init() error {
	blks := []string{}
	err := filepath.Walk(s.dir, func(spath string, info os.FileInfo, err error) error {
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
		return err
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
		return err
	}
	fi, err := os.Stat(s.fileIdPath(uint32(id)))
	if err == nil && fi.Size() >= s.size {
		id++
	}
	s.id = uint32(id)
	return nil
}

type sstore struct {
	id    uint32            //当前文件id
	mu    sync.Mutex        //
	files map[uint32]*sfile //文件指针缓存
	ext   string            //扩展名称
	size  int64             //单个文件最大长度
	dir   string            //目录名称
}

var (
	//需要切换到下一个文件存储
	nextFileErr = errors.New("next file")
)

//获取文件头标识和版本
func sfileHeaderBytes() []byte {
	w := NewWriter()
	err := w.TWrite(conf.Flags[:])
	if err != nil {
		panic(err)
	}
	err = w.TWrite(conf.Ver)
	if err != nil {
		panic(err)
	}
	return w.Bytes()
}

//存储文件
type sfile struct {
	rwm sync.Mutex
	*os.File
	size   int64
	flags  []byte
	ver    uint32
	path   string
	locker FLocker
}

func (s *sfile) close() {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	if s.locker != nil {
		_ = s.locker.Release()
	}
	if s.File != nil {
		_ = s.File.Close()
	}
}

//读取数据
func (s *sfile) read(off uint32, b []byte) error {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	if len(b) == 0 {
		return errors.New("buf args nil")
	}
	rl := len(b)
	pl := 0
	for pl < rl {
		_, err := s.Seek(int64(int(off)+pl), io.SeekStart)
		if err != nil {
			return err
		}
		cl, err := s.Read(b[pl:])
		if err != nil {
			return err
		}
		pl += cl
	}
	return nil
}

//写入数据，返回数据偏移
func (f *sfile) write(b []byte) (uint32, error) {
	f.rwm.Lock()
	defer f.rwm.Unlock()
	if len(b) == 0 {
		return 0, errors.New("b args nil")
	}
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
		_ = f.Sync()
		return uint32(off), nextFileErr
	}
	return uint32(off), nil
}

func (s sstore) newFile(id uint32, max int64) (*sfile, error) {
	spath := s.fileIdPath(id)
	locker := NewFLocker(spath+".lck", false)
	if err := locker.Lock(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(spath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		LogError("open file error", err)
		_ = locker.Release()
		return nil, err
	}
	sf := &sfile{
		path:   spath,
		File:   f,
		flags:  []byte{0, 0, 0, 0},
		size:   max,
		locker: locker,
	}
	return sf, nil
}

func (s sstore) fileIdPath(id uint32) string {
	return fmt.Sprintf("%s%s%06d%s", s.dir, Separator, id, s.ext)
}

func (f *sstore) Id() uint32 {
	return atomic.LoadUint32(&f.id)
}

func (s *sstore) checkmeta(id uint32, sf *sfile) (*sfile, error) {
	hbytes := sfileHeaderBytes()
	if err := sf.read(0, hbytes); err != nil {
		_ = sf.Close()
		return nil, err
	}
	r := NewReader(hbytes)
	if err := r.TRead(&sf.flags); err != nil {
		_ = sf.Close()
		return nil, err
	}
	if err := r.TRead(&sf.ver); err != nil {
		_ = sf.Close()
		return nil, err
	}
	if !bytes.Equal(sf.flags, conf.Flags[:]) {
		_ = sf.Close()
		return nil, errors.New("file meta error")
	}
	s.files[id] = sf
	return sf, nil
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
		return s.checkmeta(id, sf)
	}
	pos, err := sf.write(hbytes[fsiz:])
	if pos != uint32(fsiz) || err != nil {
		_ = sf.Close()
		return nil, err
	}
	_ = sf.Sync()
	s.files[id] = sf
	return sf, nil
}

func (s *sstore) Read(st BlkChunk) ([]byte, error) {
	if st.Len > MAX_BLOCK_SIZE {
		return nil, errors.New("data too big")
	}
	bb := make([]byte, st.Len)
	err := s.read(st.Id.ToUInt32(), st.Off.ToUInt32(), bb)
	if err != nil {
		return nil, err
	}
	return bb, nil
}

//读取数据
func (s *sstore) read(id uint32, off uint32, b []byte) error {
	f, err := s.openfile(id)
	if err != nil {
		return err
	}
	return f.read(off, b)
}

func (s *sstore) Write(b []byte) (BlkChunk, error) {
	fs := BlkChunk{
		Id:  VarUInt(s.Id()),
		Off: VarUInt(0),
		Len: VarUInt(len(b)),
	}
	if fs.Len > MAX_BLOCK_SIZE {
		return fs, errors.New("data too big")
	}
	off, err := s.write(b)
	if err != nil {
		return fs, err
	}
	fs.Off = VarUInt(off)
	return fs, nil
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

func (s *sstore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.files {
		v.close()
	}
}
