package xginx

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestLzma(t *testing.T) {
	s := "123456"
	z, err := LzmaCoder.Encode([]byte(s))
	require.NoError(t, err)
	assert.Equal(t, z[0], byte(0))
	uz, err := LzmaCoder.Decode(z)
	require.NoError(t, err)
	assert.Equal(t, s, string(uz))
	//zip
	s = strings.Repeat("123456", 128)
	z, err = LzmaCoder.Encode([]byte(s))
	require.NoError(t, err)
	assert.Equal(t, z[0], byte(1))
	uz, err = LzmaCoder.Decode(z)
	require.NoError(t, err)
	assert.Equal(t, s, string(uz))
}

func TestCmpMap(t *testing.T) {
	sys := &leveldbdocsystem{}
	ostr := []string{"1", "2", "3"}
	nstr := []string{"4", "5", "6"}
	as, ds := sys.cmptags(ostr, nstr)
	sort.Slice(as, func(i, j int) bool {
		return as[i] < as[j]
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i] < ds[j]
	})
	assert.Equal(t, nstr, as)
	assert.Equal(t, ostr, ds)

	ostr = []string{"1", "5", "3"}
	nstr = []string{"4", "5", "6"}
	as, ds = sys.cmptags(ostr, nstr)
	sort.Slice(as, func(i, j int) bool {
		return as[i] < as[j]
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i] < ds[j]
	})
	assert.Equal(t, []string{"4", "6"}, as)
	assert.Equal(t, []string{"1", "3"}, ds)

	ostr = []string{"1", "5", "3"}
	nstr = []string{"1", "5", "3"}
	sort.Slice(as, func(i, j int) bool {
		return as[i] < as[j]
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i] < ds[j]
	})
	as, ds = sys.cmptags(ostr, nstr)
	assert.Equal(t, []string{}, as)
	assert.Equal(t, []string{}, ds)

	ostr = []string{"1", "3", "5"}
	nstr = []string{}
	as, ds = sys.cmptags(ostr, nstr)
	sort.Slice(as, func(i, j int) bool {
		return as[i] < as[j]
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i] < ds[j]
	})
	assert.Equal(t, []string{}, as)
	assert.Equal(t, []string{"1", "3", "5"}, ds)

	ostr = []string{}
	nstr = []string{"1", "3", "5"}
	as, ds = sys.cmptags(ostr, nstr)
	sort.Slice(as, func(i, j int) bool {
		return as[i] < as[j]
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i] < ds[j]
	})
	assert.Equal(t, []string{"1", "3", "5"}, as)
	assert.Equal(t, []string{}, ds)
}

//检测doc,是否在一条链上
func checklink(fs IDocSystem, docs ...*Document) error {
	if len(docs) <= 1 {
		return nil
	}
	first, err := fs.Get(docs[0].ID)
	if err != nil {
		return err
	}
	for i := 1; i < len(docs); i++ {
		next, err := fs.Get(docs[i].ID)
		if err != nil {
			return err
		}
		if !first.Next.Equal(next.ID) {
			return fmt.Errorf("doc next id error 1")
		}
		if !next.Prev.Equal(first.ID) {
			return fmt.Errorf("doc next id error 2")
		}
		first = next
	}
	return nil
}

func TestNextPrev(t *testing.T) {
	doc0 := &Document{
		ID:    DocumentID{1},
		Tags:  []string{"小学", "中学", "大学", "狗儿子"},
		Body:  []byte("doc1"),
		TxID:  HASH256{1, 2, 3},
		Index: VarUInt(100),
	}
	doc1 := &Document{
		ID:    DocumentID{2},
		Tags:  []string{"小学", "中学", "大学", "狗儿子"},
		Body:  []byte("doc1"),
		TxID:  HASH256{1, 2, 3},
		Index: VarUInt(100),
	}
	doc2 := &Document{
		ID:    DocumentID{3},
		Tags:  []string{"小学", "中学", "大学", "狗儿子"},
		Body:  []byte("doc1"),
		TxID:  HASH256{1, 2, 3},
		Index: VarUInt(103),
	}
	//创建双向链
	//0 -> 1
	doc0.Next = doc1.ID
	//1 <- 0
	doc1.Prev = doc0.ID
	//1 -> 2
	doc1.Next = doc2.ID
	//2 <- 1
	doc2.Prev = doc1.ID

	fs, err := OpenDocSystem(NewTempDir())
	require.NoError(t, err)
	defer fs.Close()
	err = fs.Insert(doc0, doc1, doc2)
	require.NoError(t, err)

	err = checklink(fs, doc0, doc1, doc2)
	require.NoError(t, err)

	err = fs.Delete(doc1.ID)
	require.NoError(t, err)
	err = checklink(fs, doc0, doc2)
	require.NoError(t, err)

	err = fs.Insert(doc1)
	require.NoError(t, err)
	err = checklink(fs, doc0, doc1, doc2)
	require.NoError(t, err)

	doc1.Next = NilDocumentID
	doc1.Prev = doc0.ID
	err = fs.Update(doc1)
	require.NoError(t, err)
	err = checklink(fs, doc0, doc1)
	require.NoError(t, err)

	doc1.Next = doc2.ID
	doc1.Prev = NilDocumentID
	err = fs.Update(doc1)
	require.NoError(t, err)
	err = checklink(fs, doc1, doc2)
	require.NoError(t, err)
}

func TestUpdateTxID(t *testing.T) {
	str := strings.Repeat("zip", 1024)
	doc1 := &Document{
		ID:    DocumentID{1},
		Tags:  []string{"小学", "中学", "大学", "狗儿子"},
		Body:  []byte(str),
		TxID:  HASH256{1, 2, 3},
		Index: VarUInt(100),
	}
	fs, err := OpenDocSystem(NewTempDir())
	require.NoError(t, err)
	defer fs.Close()
	err = fs.Insert(doc1)
	require.NoError(t, err)
	doc3, err := fs.Get(doc1.ID)
	require.NoError(t, err)
	assert.Equal(t, doc1.TxID, doc3.TxID)
	doc3.TxID = HASH256{4, 5, 6}
	err = fs.Update(doc3)
	require.NoError(t, err)
	doc3, err = fs.Get(doc1.ID)
	require.NoError(t, err)
	assert.Equal(t, HASH256{4, 5, 6}, doc3.TxID)
}

func TestUpdateDocument(t *testing.T) {
	str := strings.Repeat("zip", 1024)
	doc1 := &Document{
		ID:   DocumentID{1},
		Tags: []string{"小学", "中学", "大学", "狗儿子"},
		Body: []byte(str),
	}
	fs, err := OpenDocSystem(NewTempDir())
	require.NoError(t, err)
	defer fs.Close()
	err = fs.Insert(doc1)
	require.NoError(t, err)
	doc3, err := fs.Get(doc1.ID)
	require.NoError(t, err)
	assert.Equal(t, doc1.Body, doc3.Body)
	doc2 := *doc1
	doc2.Tags = []string{"小学", "中学", "新标签"}
	doc2.Body = []byte("新的内容")
	err = fs.Update(&doc2)
	require.NoError(t, err)
	count := 0
	err = fs.Find("新标签").Tags(true).Each(func(doc *Document) error {
		assert.Equal(t, doc2.ID, doc.ID)
		assert.Equal(t, 3, len(doc.Tags))
		assert.Equal(t, "新的内容", string(doc.Body))
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDocumentInsert(t *testing.T) {
	doc1 := &Document{
		ID:   DocumentID{1},
		Tags: []string{"小学", "中学", "大学", "狗儿子"},
		Body: []byte("这个是学校文档1"),
	}
	doc2 := &Document{
		ID:   DocumentID{2},
		Tags: []string{"拉布拉多", "金毛狮王", "猎狗", "小狗", "毛线"},
		Body: []byte("这个是狗子文档2"),
	}
	doc3 := &Document{
		ID:   DocumentID{3},
		Tags: []string{"拉布拉多", "金毛狮王", "猎狗", "小狗"},
		Body: []byte("这个是狗子文档3"),
	}
	fs, err := OpenDocSystem(NewTempDir())
	require.NoError(t, err)
	defer fs.Close()
	err = fs.Insert(doc1, doc2, doc3)
	require.NoError(t, err)

	type item struct {
		fn    IDocIter
		skip  int
		limit int
		c     int
	}

	datas := []item{
		{fs.Prefix("毛线"), 0, 100, 1},
		{fs.Prefix("小"), 0, 100, 3},
		{fs.Find("小狗"), 0, 100, 2},
		{fs.Find("中学"), 0, 100, 1},
		{fs.Regex("金毛"), 0, 100, 2},
		{fs.Regex("狗"), 0, 100, 3},
		{fs.Prefix("金"), 0, 1, 1},
		{fs.Prefix("金"), 0, 0, 0},
	}
	for idx, data := range datas {
		count := 0
		err = data.fn.Skip(data.skip).Limit(data.limit).Each(func(doc *Document) error {
			count++
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, data.c, count, "%v", idx)
	}
}
