package xginx

import (
	"container/list"
	"errors"
	"sync"
)

//未确认单元数据

type CliUnits struct {
	mu   sync.Mutex
	liss []*list.List
	cli  HASH160
}

func NewCliUnits(cli HASH160) *CliUnits {
	return &CliUnits{
		liss: []*list.List{},
		cli:  cli,
	}
}

func (uts *CliUnits) linkList(lis *list.List, uv *Unit) *list.Element {
	for p := lis.Front(); p != nil; p = p.Next() {
		cv := p.Value.(*Unit)
		if uv.Prev.Equal(cv.Hash()) {
			return lis.InsertAfter(uv, p)
		}
		if cv.Prev.Equal(uv.Hash()) {
			return lis.InsertBefore(uv, p)
		}
	}
	return nil
}

func (uts *CliUnits) cmpTime(l1 *list.List, l2 *list.List) int {
	if l1.Len() != l2.Len() || l1.Len() == 0 || l2.Len() == 0 {
		return 0
	}
	v1 := l1.Front().Value.(*Unit)
	v2 := l2.Front().Value.(*Unit)
	if v1.STime < v2.STime {
		return -1
	}
	if v1.STime > v2.STime {
		return +1
	}
	return 0
}

//转换为数组
func (uts *CliUnits) ToUnits(lis *list.List) (Units, error) {
	uns := Units{}
	for e := lis.Front(); e != nil; e = e.Next() {
		uns = append(uns, e.Value.(*Unit))
	}
	if !uns.IsConsecutive() {
		return nil, errors.New("list not consecutive")
	}
	return uns, nil
}

//移除多个连续数据
func (uts *CliUnits) RemoveUnits(uvs Units) error {
	uts.mu.Lock()
	defer uts.mu.Unlock()
	hass := map[HASH256]bool{}
	if len(uvs) < 2 {
		return nil
	}
	for i := 0; i < len(uvs); i++ {
		cv := uvs[i]
		hass[cv.Hash()] = true
		if i == 0 {
			continue
		}
		pv := uvs[i-1]
		if !cv.Prev.Equal(pv.Hash()) {
			return errors.New("uvs not continue")
		}
		pv = cv
	}
	for _, lis := range uts.liss {
		var next *list.Element = nil
		for e := lis.Front(); e != nil; e = next {
			next = e.Next()
			uv := e.Value.(*Unit)
			if _, has := hass[uv.Hash()]; !has {
				lis.Remove(e)
			}
		}
	}
	return nil
}

//移除连续的数据
func (uts *CliUnits) RemoveList(uvs *list.List) error {
	units, err := uts.ToUnits(uvs)
	if err != nil {
		return err
	}
	return uts.RemoveUnits(units)
}

//获取最长的链
func (uts *CliUnits) MaxList() *list.List {
	uts.mu.Lock()
	defer uts.mu.Unlock()
	if len(uts.liss) == 0 {
		return nil
	}
	cur := uts.liss[0]
	for i := 1; i < len(uts.liss); i++ {
		cv := uts.liss[i]
		if cv.Len() > cur.Len() {
			cur = cv
		} else if uts.cmpTime(cv, cur) < 0 {
			cur = cv
		}
	}
	//清除空链
	nss := []*list.List{}
	for _, lv := range uts.liss {
		if lv.Len() == 0 {
			continue
		}
		nss = append(nss, lv)
	}
	uts.liss = nss
	return cur
}

//将l2元素追加到l1之后，并清空 l2
func (uts *CliUnits) append(l1 *list.List, l2 *list.List) {
	for p := l2.Front(); p != nil; p = p.Next() {
		l1.PushBack(p.Value)
	}
	l2.Init()
}

func (uts *CliUnits) merge() {
	//合并链
	if len(uts.liss) < 2 {
		return
	}
	for j := 0; j < len(uts.liss); j++ {
		jv := uts.liss[j]
		if jv.Len() == 0 {
			continue
		}
		for i := j + 1; i < len(uts.liss); i++ {
			iv := uts.liss[i]
			if iv.Len() == 0 {
				continue
			}
			b := jv.Back().Value.(*Unit)
			f := iv.Front().Value.(*Unit)
			if f.Prev.Equal(b.Hash()) {
				uts.append(jv, iv)
				continue
			}
			b = iv.Back().Value.(*Unit)
			f = jv.Front().Value.(*Unit)
			if f.Prev.Equal(b.Hash()) {
				uts.append(iv, jv)
				jv = iv
				continue
			}
		}
	}
}

func (uts *CliUnits) Push(unit *Unit) {
	uts.mu.Lock()
	defer uts.mu.Unlock()
	if !uts.cli.Equal(unit.CPks.Hash()) {
		return
	}
	linked := false
	var nlis *list.List = nil
	for _, lis := range uts.liss {
		ele := uts.linkList(lis, unit)
		if ele != nil {
			linked = true
			break
		}
		//获取空链
		if nlis == nil && lis.Len() == 0 {
			nlis = lis
		}
	}
	//没有可用的链生成新的
	if !linked {
		if nlis == nil {
			nlis = list.New()
			uts.liss = append(uts.liss, nlis)
		}
		nlis.PushBack(unit)
	}
	uts.merge()
}
