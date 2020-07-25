package xginx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"regexp"
)

//Document 文档关键字查询存储系统
type Document struct {
	ID   HASH160  //文档ID
	Tags []string //标签
	Time int64    //时间纳秒
	Body []byte   //内容
}

func GetTimeBytes(v int64) []byte {
	return EndianUInt64(uint64(v))
}

func (doc Document) TimeBytes() []byte {
	return GetTimeBytes(doc.Time)
}

func (doc Document) Encode() []byte {
	w := NewWriter()
	_ = w.WriteFull(GetTimeBytes(doc.Time))
	_ = w.WriteFull(doc.Body)
	return w.Bytes()
}

func (doc *Document) Decode(b []byte) {
	doc.Time = int64(binary.BigEndian.Uint64(b))
	doc.Body = b[8:]
}

type IDocIter interface {
	//遍历文档
	Each(fn func(doc *Document) error) error
	//跳过文档
	Skip(skip int) IDocIter
	//限制数量
	Limit(skip int) IDocIter
	//是否查询包含的所有tags 有损一点性能
	Tags(v bool) IDocIter
	//向下迭代
	ByNext() IDocIter
	//向上迭代
	ByPrev() IDocIter
}

//标签索引库接口
type IDocSystem interface {
	//追加tag
	AddTag(id HASH160, tags ...string) error
	//删除tag
	DelTag(id HASH160, tags ...string) error
	//添加文档
	Insert(doc ...*Document) error
	//删除文档
	Delete(id ...HASH160) error
	//更新文档
	Update(doc ...*Document) error
	//根据id获取文档内容 qtag是否查询tags
	Get(id HASH160, qtag ...bool) (*Document, error)
	//固定key查询
	Find(key string) IDocIter
	//按前缀查询文档
	Prefix(key string) IDocIter
	//模糊查询文档使用正则
	Like(str string) IDocIter
	//关闭文件系统
	Close()
	//按时间索引迭代 v = 0 从小到大 v = -1 从大到小
	ByTime(v ...int64) IDocIter
}

var (
	fkprefix = []byte{1} //正向索引前缀
	bkprefix = []byte{2} //方向索引前缀
	ckprefix = []byte{3} //固定值前缀
	tkprefix = []byte{4} //时间索引前缀
)

type leveldbdocsystem struct {
	db DBImp
}

func (sys *leveldbdocsystem) Close() {
	if sys.db != nil {
		sys.db.Close()
	}
}

//检测id文档是否存在
func (sys *leveldbdocsystem) exists(id HASH160) bool {
	return sys.db.Has(ckprefix, id[:])
}

//追加tag
func (sys *leveldbdocsystem) AddTag(id HASH160, tags ...string) error {
	if id.Equal(ZERO160) {
		return fmt.Errorf("id error nil")
	}
	if !sys.exists(id) {
		return fmt.Errorf("doc %v not exists", id)
	}
	bt := sys.db.NewBatch()
	//添加索引
	for _, tag := range tags {
		//正向索引key prefix_tag_id ->id
		bt.Put(fkprefix, []byte(tag), id[:], id[:])
		//反向索引key prefix_id_tag -> tag
		bt.Put(bkprefix, id[:], []byte(tag), []byte(tag))
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
func (sys *leveldbdocsystem) DelTag(id HASH160, tags ...string) error {
	if id.Equal(ZERO160) {
		return fmt.Errorf("id error nil")
	}
	if !sys.exists(id) {
		return fmt.Errorf("doc %v not exists", id)
	}
	bt := sys.db.NewBatch()
	//删除索引
	bp := GetDBKey(bkprefix, id[:])
	//可从方向索引推导出正向索引tag
	iter := sys.db.Iterator(NewPrefix(bp))
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
	if doc.ID.Equal(ZERO160) {
		return fmt.Errorf("id error nil")
	}
	if sys.exists(doc.ID) {
		return fmt.Errorf("doc %v exists", doc.ID)
	}
	//添加索引
	for _, tag := range doc.Tags {
		sys.settag(bt, tag, doc.ID, false)
	}
	//添加内容
	bt.Put(ckprefix, doc.ID[:], doc.Encode())
	//添加时间排序索引
	bt.Put(tkprefix, doc.TimeBytes(), doc.ID[:], doc.ID[:])
	//一次性写入
	return nil
}

//利用反向索引搜索文档有哪些tag
func (sys *leveldbdocsystem) gettags(id HASH160) []string {
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
func (sys *leveldbdocsystem) Get(id HASH160, qtag ...bool) (*Document, error) {
	//先获取原内容
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
func (sys *leveldbdocsystem) Delete(ids ...HASH160) error {
	bt := sys.db.NewBatch()
	for _, id := range ids {
		err := sys.delete(bt, id)
		if err != nil {
			return err
		}
	}
	return sys.db.Write(bt)
}

//删除文档
func (sys *leveldbdocsystem) delete(bt *Batch, id HASH160) error {
	if id.Equal(ZERO160) {
		return fmt.Errorf("id error nil")
	}
	//先获取原内容
	doc, err := sys.Get(id)
	if err != nil {
		return err
	}
	//删除时间索引
	bt.Del(tkprefix, doc.TimeBytes(), id[:])
	//删除内容
	bt.Del(ckprefix, id[:])
	//删除所有的相关索引
	//可从反向索引推导出正向索引tag
	bp := GetDBKey(bkprefix, id[:])
	iter := sys.db.Iterator(NewPrefix(bp))
	for iter.Next() {
		//反向key
		bt.Del(iter.Key())
		//正向key
		bt.Del(fkprefix, iter.Value(), id[:])
	}
	return nil
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
	amap := map[string]bool{}
	dmap := map[string]bool{}
	omap := sys.listtomap(o)
	nmap := sys.listtomap(n)
	for tag := range nmap {
		if !omap[tag] {
			amap[tag] = true
		}
	}
	for tag := range omap {
		if !nmap[tag] {
			dmap[tag] = true
		}
	}
	return sys.maptolist(amap), sys.maptolist(dmap)
}

func (sys *leveldbdocsystem) settag(bt *Batch, tag string, id HASH160, del bool) {
	if del {
		bt.Del(fkprefix, []byte(tag), id[:])
		bt.Del(bkprefix, id[:], []byte(tag))
	} else {
		bt.Put(fkprefix, []byte(tag), id[:], id[:])
		bt.Put(bkprefix, id[:], []byte(tag), []byte(tag))
	}
}

//更新文档
func (sys *leveldbdocsystem) update(bt *Batch, ndoc *Document) error {
	if ndoc.ID.Equal(ZERO160) {
		return fmt.Errorf("id error nil")
	}
	//查询原文档包括标签
	odoc, err := sys.Get(ndoc.ID, true)
	if err != nil {
		return err
	}
	//比较处理标签
	tadds, tdels := sys.cmptags(odoc.Tags, ndoc.Tags)
	for _, tag := range tadds {
		sys.settag(bt, tag, ndoc.ID, false)
	}
	for _, tag := range tdels {
		sys.settag(bt, tag, odoc.ID, true)
	}
	//如果内容或者时间不同
	if ndoc.Time != ndoc.Time || !bytes.Equal(odoc.Body, ndoc.Body) {
		//更新时间索引
		bt.Del(tkprefix, odoc.TimeBytes(), odoc.ID[:])
		bt.Put(tkprefix, ndoc.TimeBytes(), ndoc.ID[:], ndoc.ID[:])
		//只要时间或者内容不同都要更新内容
		bt.Del(ckprefix, odoc.ID[:]) //先删除再添加
		bt.Put(ckprefix, ndoc.ID[:], ndoc.Encode())
	}
	return nil
}

//更新文档
func (sys *leveldbdocsystem) Update(ndoc ...*Document) error {
	bt := sys.db.NewBatch()
	for _, doc := range ndoc {
		err := sys.update(bt, doc)
		if err != nil {
			return err
		}
	}
	return sys.db.Write(bt)
}

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
	bytime  bool
	ittime  *int64
	byprev  bool //向上
	bynext  bool //向下
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
func (it *dociter) newfkdoc(keys []string, id HASH160) (*Document, error) {
	body, err := it.sys.db.Get(ckprefix, id[:])
	if err != nil {
		return nil, err
	}
	doc := &Document{}
	doc.ID = id
	doc.Tags = keys
	doc.Decode(body)
	return doc, nil
}

//如果是find查询并且找到
func (it *dociter) isfinded(k []byte, id HASH160) bool {
	ckey := GetDBKey(fkprefix, []byte(it.key), id[:])
	return bytes.Equal(k, ckey)
}

func (it *dociter) isliked(kkey []byte) bool {
	return it.regex.Match(kkey)
}

func (it *dociter) getTime(k []byte) int64 {
	return int64(binary.BigEndian.Uint64(k[1:]))
}

func (it *dociter) getTimeIterator(tv int64) *Iterator {
	s := GetDBKey(tkprefix, GetTimeBytes(tv))
	l := GetDBKey(tkprefix, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	return it.sys.db.Iterator(NewRange(s, l))
}

func (it *dociter) eachbytime(fn func(doc *Document) error) error {
	fp := GetDBKey(tkprefix)
	iter := it.sys.db.Iterator(NewPrefix(fp))
	limit := 0
	skip := 0
	finded := false
	first := true
	for {
		if it.ittime != nil && first {
			titer := it.getTimeIterator(*it.ittime)
			if titer.Next() {
				finded = iter.Seek(titer.Key())
			} else if it.byprev {
				finded = iter.Last()
			} else if it.bynext {
				finded = iter.First()
			}
			titer.Close()
			first = false
		} else if it.bynext {
			if first {
				finded = iter.First()
				first = false
			} else {
				finded = iter.Next()
			}
		} else if it.byprev {
			if first {
				finded = iter.Last()
				first = false
			} else {
				finded = iter.Prev()
			}
		}
		if !finded {
			break
		}
		if tv := it.getTime(iter.Key()); it.ittime != nil {
			if it.bynext && tv < *it.ittime {
				continue
			}
			if it.byprev && tv > *it.ittime {
				continue
			}
		}
		hid := NewHASH160(iter.Value())
		//前缀和模糊查询时记录反向tags
		keys := []string{}
		if it.stags != nil && *it.stags {
			keys = it.sys.gettags(hid)
		}
		if it.skip != nil && skip < *it.skip {
			skip++
			continue
		}
		doc, err := it.newfkdoc(keys, hid)
		if err != nil {
			panic(err)
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

func (it *dociter) Each(fn func(doc *Document) error) error {
	if it.bytime {
		return it.eachbytime(fn)
	}
	var fp []byte
	if it.qfind || it.qprefix {
		fp = GetDBKey(fkprefix, []byte(it.key))
	} else if it.qlike {
		fp = GetDBKey(fkprefix)
	} else {
		panic(fmt.Errorf("find and like error"))
	}
	//分组map记录id是否已经处理,当文档有很多tag时可能会遍历到多次
	gmap := map[HASH160]bool{}
	iter := it.sys.db.Iterator(NewPrefix(fp))
	ilen := len(ZERO160)
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
		hid := NewHASH160(iter.Value())
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
		keys := []string{string(kkey)}
		//前缀和模糊查询时记录反向tags
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
			panic(err)
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
func (sys *leveldbdocsystem) ByTime(v ...int64) IDocIter {
	iter := &dociter{
		sys:     sys,
		qprefix: false,
		qlike:   false,
		qfind:   false,
		bynext:  true,
		bytime:  true,
		byprev:  false,
	}
	if len(v) > 0 {
		iter.ittime = &v[0]
	}
	return iter
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

//模糊查询文档
func (sys *leveldbdocsystem) Like(str string) IDocIter {
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

//func (db *tagsdb) Insert(doc []byte, keys ...[]byte) error {
//	bt := db.db.NewBatch()
//	for _, key := range keys {
//		//正向索引key prefix_key_doc ->doc
//		bt.Put(fprefix, key, doc, doc)
//		//反向索引key prefix_doc_key -> key
//		bt.Put(bprefix, doc, key, key)
//	}
//	return db.db.Write(bt)
//}
