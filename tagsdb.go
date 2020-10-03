package xginx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"regexp"

	"github.com/cxuhua/lzma"
)

//Document 文档关键字查询存储系统
//next prev为存在链式关系时启用
type Document struct {
	ID    DocumentID //文档ID
	Tags  []string   //标签
	TxID  HASH256    //文档对应交易ID
	Index VarUInt    //文档对应的输出索引
	Body  VarBytes   //内容
	Next  DocumentID //下个关系
	Prev  DocumentID //上个关系
}

func NewDocument() *Document {
	return &Document{}
}

func (doc Document) Encode() []byte {
	w := NewWriter()
	err := doc.TxID.Encode(w)
	if err != nil {
		panic(err)
	}
	err = doc.Index.Encode(w)
	if err != nil {
		panic(err)
	}
	err = doc.Body.Encode(w)
	if err != nil {
		panic(err)
	}
	err = doc.Next.Encode(w)
	if err != nil {
		panic(err)
	}
	err = doc.Prev.Encode(w)
	if err != nil {
		panic(err)
	}
	bb := w.Bytes()
	if len(bb) < 1024 {
		return append([]byte{0}, bb...)
	}
	//启用1klzma压缩
	zb, err := lzma.Compress(bb)
	if err != nil {
		panic(err)
	}
	return append([]byte{1}, zb...)
}

func (doc *Document) GetNext(db IDocSystem, qtag ...bool) (*Document, error) {
	if doc.Next.Equal(NilDocumentID) {
		return nil, fmt.Errorf("next null")
	}
	return db.Get(doc.Next, qtag...)
}

func (doc *Document) GetPrev(db IDocSystem, qtag ...bool) (*Document, error) {
	if doc.Prev.Equal(NilDocumentID) {
		return nil, fmt.Errorf("prev null")
	}
	return db.Get(doc.Prev, qtag...)
}

func (doc *Document) Decode(b []byte) {
	if len(b) < 1 {
		panic(fmt.Errorf("body zip flags  error"))
	}
	rb := b[1:]
	if b[0] > 0 {
		uzb, err := lzma.Uncompress(rb)
		if err != nil {
			panic(err)
		}
		rb = uzb
	}
	if len(rb) < 8 {
		panic(fmt.Errorf("body bytes error"))
	}
	r := NewReader(rb)
	err := doc.TxID.Decode(r)
	if err != nil {
		panic(err)
	}
	err = doc.Index.Decode(r)
	if err != nil {
		panic(err)
	}
	err = doc.Body.Decode(r)
	if err != nil {
		panic(err)
	}
	err = doc.Next.Decode(r)
	if err != nil {
		panic(err)
	}
	err = doc.Prev.Decode(r)
	if err != nil {
		panic(err)
	}
}

type IDocIter interface {
	//遍历文档
	Each(fn func(doc *Document) error) error
	//跳过文档
	Skip(skip int) IDocIter
	//限制数量
	Limit(limit int) IDocIter
	//是否查询包含的所有tags 有损一点性能
	Tags(v bool) IDocIter
	//ByNext 向下迭代
	ByNext() IDocIter
	//ByPrev 向上迭代
	ByPrev() IDocIter
	//获取最后一个key
	LastKey() []byte
	//设置lastkey
	SetLastKey(lk []byte)
}

//文档编码器
type ICoder interface {
	Encode(bb []byte) ([]byte, error)
	Decode(bb []byte) ([]byte, error)
}

type lzmacoder struct {
}

func (coder lzmacoder) Encode(bb []byte) ([]byte, error) {
	var zb []byte
	var err error
	if len(bb) == 0 {
		return nil, nil
	}
	fb := byte(0)
	zb, err = lzma.Compress(bb)
	if err != nil {
		return nil, err
	}
	//如果压缩效果比数据少,启用压缩数据
	if len(zb) < len(bb) {
		fb = byte(1)
	} else {
		zb = bb
	}
	return append([]byte{fb}, zb...), nil
}

func (coder lzmacoder) Decode(bb []byte) ([]byte, error) {
	if len(bb) == 0 {
		return nil, nil
	}
	if bb[0] == 0 {
		return bb[1:], nil
	}
	return lzma.Uncompress(bb[1:])
}

var (
	LzmaCoder ICoder = lzmacoder{}
)

//标签索引库接口
type IDocSystem interface {
	//追加tag
	AddTag(id DocumentID, tags ...string) error
	//删除tag
	DelTag(id DocumentID, tags ...string) error
	//添加文档
	Insert(doc ...*Document) error
	//删除文档
	Delete(id ...DocumentID) error
	//更新文档
	Update(doc ...*Document) error
	//根据id获取文档内容 qtag是否查询tags
	Get(id DocumentID, qtag ...bool) (*Document, error)
	//文档是否存在
	Has(id DocumentID) (bool, error)
	//固定key查询
	Find(key string) IDocIter
	//获取所有文档 prefix存在时拼接前缀
	All(prefix ...[]byte) IDocIter
	//按前缀查询文档
	Prefix(key string) IDocIter
	//模糊查询文档使用正则
	Regex(str string) IDocIter
	//写入磁盘
	Sync()
	//关闭文件系统
	Close()
}

var (
	fkprefix = []byte{1} //正向索引前缀
	bkprefix = []byte{2} //反向索引前缀
	ckprefix = []byte{3} //固定值前缀
)

type leveldbdocsystem struct {
	db DBImp
}

func (sys *leveldbdocsystem) Sync() {
	sys.db.Sync()
}

func (sys *leveldbdocsystem) Close() {
	sys.db.Close()
}

//检测id文档是否存在
func (sys *leveldbdocsystem) Has(id DocumentID) (bool, error) {
	return sys.db.Has(ckprefix, id[:])
}

//追加tag
func (sys *leveldbdocsystem) AddTag(id DocumentID, tags ...string) error {
	if id.Equal(NilDocumentID) {
		return fmt.Errorf("id error nil")
	}
	if has, err := sys.Has(id); err != nil {
		return err
	} else if !has {
		return fmt.Errorf("doc %v not exists", id)
	}
	bt := sys.db.NewBatch()
	//添加索引
	for _, tag := range tags {
		sys.settag(bt, tag, id, false)
	}
	return sys.db.Write(bt)
}

func (sys *leveldbdocsystem) hastags(tags []string, tag []byte) bool {
	for _, v := range tags {
		if v == string(tag) {
			return true
		}
	}
	return false
}

//删除tag
func (sys *leveldbdocsystem) DelTag(id DocumentID, tags ...string) error {
	if id.Equal(NilDocumentID) {
		return fmt.Errorf("id error nil")
	}
	if has, err := sys.Has(id); err != nil {
		return err
	} else if !has {
		return fmt.Errorf("doc %v not exists", id)
	}
	bt := sys.db.NewBatch()
	//删除索引
	bp := GetDBKey(bkprefix, id[:])
	//可从反向索引推导出正向索引tag
	iter := sys.db.Iterator(NewPrefix(bp))
	defer iter.Close()
	for iter.Next() {
		tag := iter.Value()
		if !sys.hastags(tags, tag) {
			continue
		}
		//反向key
		bt.Del(iter.Key())
		//正向key
		bt.Del(fkprefix, tag, id[:])
	}
	return sys.db.Write(bt)
}

func (sys *leveldbdocsystem) Insert(docs ...*Document) error {
	bt := sys.db.NewBatch()
	for _, doc := range docs {
		err := sys.insert(bt, doc)
		if err != nil {
			return err
		}
	}
	return sys.db.Write(bt)
}

//添加文档
func (sys *leveldbdocsystem) insert(bt *Batch, doc *Document) error {
	if len(doc.Body) == 0 {
		return fmt.Errorf("body empty")
	}
	if doc.ID.Equal(NilDocumentID) {
		return fmt.Errorf("id error nil")
	}
	if has, err := sys.Has(doc.ID); err != nil {
		return err
	} else if has {
		return fmt.Errorf("doc %v exists", doc.ID)
	}
	//添加索引
	for _, tag := range doc.Tags {
		sys.settag(bt, tag, doc.ID, false)
	}
	//添加内容
	bb := doc.Encode()
	bt.Put(ckprefix, doc.ID[:], bb)
	//创建链接
	return sys.insertlink(bt, doc)
}

//利用反向索引搜索文档有哪些tag
func (sys *leveldbdocsystem) gettags(id DocumentID) []string {
	strs := []string{}
	fp := GetDBKey(bkprefix, id[:])
	//id固定长度,所以根据这个前缀查询到的值肯定是标签
	iter := sys.db.Iterator(NewPrefix(fp))
	for iter.Next() {
		strs = append(strs, string(iter.Value()))
	}
	iter.Close()
	return strs
}

//获取单个文档
func (sys *leveldbdocsystem) Get(id DocumentID, qtag ...bool) (*Document, error) {
	doc := &Document{}
	bb, err := sys.db.Get(ckprefix, id[:])
	if err != nil {
		return nil, err
	}
	if len(qtag) > 0 && qtag[0] {
		doc.Tags = sys.gettags(id)
	}
	doc.ID = id
	doc.Decode(bb)
	return doc, nil
}

//删除文档
func (sys *leveldbdocsystem) Delete(ids ...DocumentID) error {
	bt := sys.db.NewBatch()
	for _, id := range ids {
		err := sys.delete(bt, id)
		if err != nil {
			return err
		}
	}
	return sys.db.Write(bt)
}

//插入链接关系
func (sys *leveldbdocsystem) updatelink(bt *Batch, odoc *Document, ndoc *Document) error {
	if !odoc.Next.Equal(ndoc.Next) && !odoc.Prev.Equal(ndoc.Prev) {
		err := sys.deletelink(bt, odoc)
		if err != nil {
			return err
		}
	} else if !odoc.Next.Equal(ndoc.Next) {
		next, err := odoc.GetNext(sys, true)
		if err == nil {
			next.Prev = NilDocumentID
			err = sys.update(bt, next, true)
		}
		if err != nil {
			return err
		}
	} else if !odoc.Prev.Equal(ndoc.Prev) {
		prev, err := odoc.GetPrev(sys, true)
		if err == nil {
			prev.Next = NilDocumentID
			err = sys.update(bt, prev, true)
		}
		if err != nil {
			return err
		}
	}
	return sys.insertlink(bt, ndoc)
}

//插入链接关系
func (sys *leveldbdocsystem) insertlink(bt *Batch, doc *Document) error {
	prev, perr := doc.GetPrev(sys, true)
	next, nerr := doc.GetNext(sys, true)
	//有上一个
	if perr == nil {
		prev.Next = doc.ID
		err := sys.update(bt, prev, true)
		if err != nil {
			return err
		}
	}
	//有下一个
	if nerr == nil {
		next.Prev = doc.ID
		err := sys.update(bt, next, true)
		if err != nil {
			return err
		}
	}
	return nil
}

//删除更新链接关系
func (sys *leveldbdocsystem) deletelink(bt *Batch, doc *Document) error {
	uprev, unext := false, false
	prev, perr := doc.GetPrev(sys, true)
	next, nerr := doc.GetNext(sys, true)
	if perr == nil && nerr != nil {
		//有上一个没下一个
		prev.Next = NilDocumentID
		uprev = true
	} else if perr == nil && nerr == nil {
		//有上一个也有下一个
		prev.Next = next.ID
		next.Prev = prev.ID
		uprev = true
		unext = true
	} else if perr != nil && nerr == nil {
		//没有上一个,有下一个
		next.Prev = NilDocumentID
		unext = true
	} else {
		//都没有直接返回
		return nil
	}
	if uprev {
		err := sys.update(bt, prev, true)
		if err != nil {
			return err
		}
	}
	if unext {
		err := sys.update(bt, next, true)
		if err != nil {
			return err
		}
	}
	return nil
}

//删除文档
func (sys *leveldbdocsystem) delete(bt *Batch, id DocumentID) error {
	if id.Equal(NilDocumentID) {
		return fmt.Errorf("id error nil")
	}
	//先获取原内容
	old, err := sys.Get(id)
	if err != nil {
		return fmt.Errorf("doc miss")
	}
	//删除内容
	bt.Del(ckprefix, id[:])
	//删除所有的相关索引
	//可从反向索引推导出正向索引tag
	bp := GetDBKey(bkprefix, id[:])
	iter := sys.db.Iterator(NewPrefix(bp))
	defer iter.Close()
	for iter.Next() {
		//反向key
		bt.Del(iter.Key())
		//正向key
		bt.Del(fkprefix, iter.Value(), id[:])
	}
	//检测是否更新链接关系
	return sys.deletelink(bt, old)
}

func (sys *leveldbdocsystem) listtomap(ss []string) map[string]bool {
	ret := map[string]bool{}
	for _, str := range ss {
		ret[str] = true
	}
	return ret
}

func (sys *leveldbdocsystem) maptolist(smap map[string]bool) []string {
	ret := []string{}
	for str := range smap {
		ret = append(ret, str)
	}
	return ret
}

//比较原标签和新标签,返回需要删除和添加的标签
func (sys *leveldbdocsystem) cmptags(o []string, n []string) ([]string, []string) {
	amap, dmap := map[string]bool{}, map[string]bool{}
	omap, nmap := sys.listtomap(o), sys.listtomap(n)
	//获取新增的标签
	for tag := range nmap {
		if !omap[tag] {
			amap[tag] = true
		}
	}
	//获取删除的标签
	for tag := range omap {
		if !nmap[tag] {
			dmap[tag] = true
		}
	}
	//返回新增,删除的
	return sys.maptolist(amap), sys.maptolist(dmap)
}

func (sys *leveldbdocsystem) settag(bt *Batch, tag string, id DocumentID, del bool) {
	if del {
		bt.Del(fkprefix, []byte(tag), id[:])
		bt.Del(bkprefix, id[:], []byte(tag))
	} else {
		bt.Put(fkprefix, []byte(tag), id[:], id[:])
		bt.Put(bkprefix, id[:], []byte(tag), []byte(tag))
	}
}

//更新文档
func (sys *leveldbdocsystem) update(bt *Batch, ndoc *Document, only bool) error {
	if ndoc.ID.Equal(NilDocumentID) {
		return fmt.Errorf("id error nil")
	}
	//查询原文档包括标签
	odoc, err := sys.Get(ndoc.ID, true)
	if err != nil {
		return err
	}
	//比较处理标签
	tadds, tdels := sys.cmptags(odoc.Tags, ndoc.Tags)
	//新增的标签
	for _, tag := range tadds {
		sys.settag(bt, tag, ndoc.ID, false)
	}
	//删除的标签
	for _, tag := range tdels {
		sys.settag(bt, tag, odoc.ID, true)
	}
	//先删除再添加
	bt.Del(ckprefix, odoc.ID[:])
	bt.Put(ckprefix, ndoc.ID[:], ndoc.Encode())
	if only {
		return nil
	}
	return sys.updatelink(bt, odoc, ndoc)
}

//更新文档
func (sys *leveldbdocsystem) Update(ndoc ...*Document) error {
	bt := sys.db.NewBatch()
	for _, doc := range ndoc {
		err := sys.update(bt, doc, false)
		if err != nil {
			return err
		}
	}
	return sys.db.Write(bt)
}

//查询迭代处理
type dociter struct {
	sys     *leveldbdocsystem
	qprefix bool //前缀查询
	qlike   bool //模糊查询 使用正则表达式
	regex   *regexp.Regexp
	qfind   bool //直接查询
	key     string
	skip    *int
	limit   *int
	stags   *bool //是否使用反向索引查询文档对应的所有tag
	byprev  bool  //向上
	bynext  bool  //向下
	qall    bool
	lkey    []byte   //qall时保存最后一个key
	prefix  [][]byte //qall 时使用的前缀
}

//获取最后一个key
func (it *dociter) LastKey() []byte {
	return it.lkey
}

func (it *dociter) SetLastKey(lk []byte) {
	it.lkey = lk
}

func (it *dociter) ByNext() IDocIter {
	it.bynext = true
	it.byprev = false
	return it
}

func (it *dociter) ByPrev() IDocIter {
	it.byprev = true
	it.bynext = false
	return it
}

func (it *dociter) Skip(skip int) IDocIter {
	it.skip = &skip
	return it
}

func (it *dociter) Limit(limit int) IDocIter {
	it.limit = &limit
	return it
}
func (it *dociter) Tags(v bool) IDocIter {
	it.stags = &v
	return it
}

//正向获取
func (it *dociter) newfkdoc(tags []string, id DocumentID) (*Document, error) {
	doc, err := it.sys.Get(id)
	if err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		doc.Tags = tags
	}
	return doc, nil
}

//如果是find查询并且找到
func (it *dociter) isfinded(k []byte, id DocumentID) bool {
	ckey := GetDBKey(fkprefix, []byte(it.key), id[:])
	return bytes.Equal(k, ckey)
}

func (it *dociter) isliked(kkey []byte) bool {
	return it.regex.Match(kkey)
}

func (it *dociter) getTime(k []byte) VarUInt {
	return VarUInt(binary.BigEndian.Uint64(k[1:]))
}

func (it *dociter) eachquery(fn func(doc *Document) error) error {
	var fp []byte
	if it.qfind || it.qprefix {
		fp = GetDBKey(fkprefix, []byte(it.key))
	} else if it.qlike {
		fp = GetDBKey(fkprefix)
	} else {
		panic(fmt.Errorf("find and like error"))
	}
	//分组map记录id是否已经处理,当文档有很多tag时可能会遍历到多次
	gmap := map[DocumentID]bool{}
	iter := it.sys.db.Iterator(NewPrefix(fp))
	defer iter.Close()
	ilen := DocumentIDLen
	flen := len(fkprefix)
	limit := 0
	skip := 0
	for iter.Next() {
		key := iter.Key()
		klen := len(key)
		if klen < ilen+flen {
			return fmt.Errorf("key %v length error", key)
		}
		kkey := key[flen : klen-ilen]
		hid := NewDocumentIDFrom(iter.Value())
		if it.qfind && !it.isfinded(key, hid) {
			return fmt.Errorf("not found %s", it.key)
		}
		//like不匹配直接下一个
		if it.qlike && !it.isliked(kkey) {
			continue
		}
		//如果文档已经处理过直接到下一个
		if _, has := gmap[hid]; has {
			continue
		}
		//是否查询相关tags
		keys := []string{string(kkey)}
		if it.stags != nil && *it.stags {
			keys = it.sys.gettags(hid)
		}
		gmap[hid] = true
		if it.skip != nil && skip < *it.skip {
			skip++
			continue
		}
		doc, err := it.newfkdoc(keys, hid)
		if err != nil {
			return err
		}
		if it.limit != nil && limit >= *it.limit {
			break
		}
		if err := fn(doc); err != nil {
			return err
		}
		limit++
	}
	return nil
}

func (it *dociter) eachall(fn func(doc *Document) error) error {
	ckp := append([]byte{}, ckprefix...)
	for _, bb := range it.prefix {
		ckp = append(ckp, bb...)
	}
	has := true
	iter := it.sys.db.Iterator(NewPrefix(ckp))
	//seek不存志不会继续获取
	if it.lkey != nil {
		has = iter.Seek(it.lkey)
	}
	defer iter.Close()
	limit := 0
	first := true
	for has {
		var has bool
		if it.bynext {
			has = iter.Next()
			first = false
		} else if it.byprev {
			if first {
				has = iter.Last()
			} else {
				has = iter.Prev()
			}
			first = false
		} else {
			return fmt.Errorf("bynext or byprev args error")
		}
		if !has {
			break
		}
		if it.limit != nil && limit >= *it.limit {
			break
		}
		key := iter.Key()
		doc := &Document{}
		doc.ID = NewDocumentIDFrom(key[len(ckprefix):])
		bb := iter.Value()
		doc.Decode(bb)
		if err := fn(doc); err != nil {
			return err
		}
		it.lkey = key
		limit++
	}
	return nil
}

func (it *dociter) Each(fn func(doc *Document) error) error {
	if it.qall {
		return it.eachall(fn)
	}
	return it.eachquery(fn)
}

//遍历所有文档
func (sys *leveldbdocsystem) All(prefix ...[]byte) IDocIter {
	return &dociter{
		sys:    sys,
		qall:   true,
		bynext: true, //默认向下
		prefix: prefix,
	}
}

//按前缀查询文档
func (sys *leveldbdocsystem) Find(key string) IDocIter {
	return &dociter{
		sys:     sys,
		qprefix: false,
		key:     key,
		qlike:   false,
		qfind:   true,
		bynext:  true,
		byprev:  false,
	}
}

//按前缀查询文档
func (sys *leveldbdocsystem) Prefix(key string) IDocIter {
	return &dociter{
		sys:     sys,
		qprefix: true,
		key:     key,
		qlike:   false,
		qfind:   false,
		bynext:  true,
		byprev:  false,
	}
}

//使用正则模糊查询文档
func (sys *leveldbdocsystem) Regex(str string) IDocIter {
	return &dociter{
		sys:     sys,
		qprefix: false,
		qlike:   true,
		regex:   regexp.MustCompile(str),
		qfind:   false,
		bynext:  true,
		byprev:  false,
	}
}

//OpenDocSystem 打开文档系统,不存在自动创建
func OpenDocSystem(dir string) (IDocSystem, error) {
	db, err := NewDBImp(dir)
	if err != nil {
		return nil, err
	}
	return &leveldbdocsystem{
		db: db,
	}, nil
}
