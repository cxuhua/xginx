package xginx

import (
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
	sort.Slice(as, func(i, j int) bool {
		return as[i] < as[j]
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i] < ds[j]
	})
	as, ds = sys.cmptags(ostr, nstr)
	assert.Equal(t, []string{}, as)
	assert.Equal(t, []string{"1", "3", "5"}, ds)

	ostr = []string{}
	nstr = []string{"1", "3", "5"}
	sort.Slice(as, func(i, j int) bool {
		return as[i] < as[j]
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i] < ds[j]
	})
	as, ds = sys.cmptags(ostr, nstr)
	assert.Equal(t, []string{"1", "3", "5"}, as)
	assert.Equal(t, []string{}, ds)
}

func TestUpdateDocument(t *testing.T) {
	str := strings.Repeat("zip", 1024)
	doc1 := &Document{
		ID:   HASH160{1},
		Tags: []string{"小学", "中学", "大学", "狗儿子"},
		Body: []byte(str),
		Time: 90,
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
	doc2.Time = 100
	doc2.Tags = []string{"小学", "中学", "新标签"}
	doc2.Body = []byte("新的内容")
	err = fs.Update(&doc2)
	require.NoError(t, err)
	count := 0
	err = fs.Find("新标签").Tags(true).Each(func(doc *Document) error {
		assert.Equal(t, doc2.ID, doc.ID)
		assert.Equal(t, 3, len(doc.Tags))
		assert.Equal(t, int64(100), doc.Time)
		assert.Equal(t, "新的内容", string(doc.Body))
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDocumentListTime(t *testing.T) {
	doc1 := &Document{
		ID:   HASH160{1},
		Tags: []string{"小学", "中学", "大学", "狗儿子"},
		Body: []byte("这个是学校文档1"),
		Time: 90,
	}
	doc2 := &Document{
		ID:   HASH160{2},
		Tags: []string{"拉布拉多", "金毛狮王", "猎狗", "小狗", "毛线"},
		Body: []byte("这个是狗子文档2"),
		Time: 100,
	}
	doc3 := &Document{
		ID:   HASH160{3},
		Tags: []string{"拉布拉多", "金毛狮王", "猎狗", "小狗"},
		Body: []byte("这个是狗子文档3"),
		Time: 110,
	}
	fs, err := OpenDocSystem(NewTempDir())
	require.NoError(t, err)
	defer fs.Close()
	err = fs.Insert(doc1, doc2, doc3)
	require.NoError(t, err)

	type item struct {
		v    []int64
		next bool
		c    int
	}

	datas := []item{
		{nil, true, 3},                    //default next
		{[]int64{doc1.Time - 1}, true, 3}, // >
		{[]int64{doc1.Time}, true, 3},     // >=
		{[]int64{doc2.Time}, true, 2},     // >=
		{[]int64{doc3.Time}, true, 1},     // >=
		{[]int64{doc3.Time + 1}, true, 0}, // >= out

		{nil, false, 3},                    //default prev
		{[]int64{doc3.Time + 1}, false, 3}, //<
		{[]int64{doc3.Time}, false, 3},     //<=
		{[]int64{doc2.Time}, false, 2},     //<=
		{[]int64{doc1.Time}, false, 1},     //<=
		{[]int64{doc1.Time - 1}, false, 0}, //<= out
	}

	for _, data := range datas {
		count := 0
		if data.next {
			err = fs.ByTime(data.v...).ByNext().Each(func(doc *Document) error {
				count++
				return nil
			})
		} else {
			err = fs.ByTime(data.v...).ByPrev().Each(func(doc *Document) error {
				count++
				return nil
			})
		}
		require.NoError(t, err)
		assert.Equal(t, data.c, count, "%v %v", data.v, data.c)
	}
}

func TestDocumentInsert(t *testing.T) {
	doc1 := &Document{
		ID:   HASH160{1},
		Tags: []string{"小学", "中学", "大学", "狗儿子"},
		Body: []byte("这个是学校文档1"),
		Time: 90,
	}
	doc2 := &Document{
		ID:   HASH160{2},
		Tags: []string{"拉布拉多", "金毛狮王", "猎狗", "小狗", "毛线"},
		Body: []byte("这个是狗子文档2"),
		Time: 100,
	}
	doc3 := &Document{
		ID:   HASH160{3},
		Tags: []string{"拉布拉多", "金毛狮王", "猎狗", "小狗"},
		Body: []byte("这个是狗子文档3"),
		Time: 110,
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
