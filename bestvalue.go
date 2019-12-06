package xginx

var (
	BestBlockKey = []byte("BestBlockKey") //最高区块数据保存key
	InvalidBest  = NewInvalidBest()       //无效的状态
)

type BestValue struct {
	Id     HASH256
	Height uint32
}

//获取当前高度
func (bv BestValue) Curr() uint32 {
	if !bv.IsValid() {
		return 0
	} else {
		return bv.Height
	}
}

func (bv BestValue) LastID() HASH256 {
	if bv.IsValid() {
		return bv.Id
	} else {
		return conf.genesis
	}
}

//获取下一个高度
func (bv BestValue) Next() uint32 {
	return NextHeight(bv.Height)
}

func NextHeight(h uint32) uint32 {
	if h == InvalidHeight {
		return 0
	} else {
		return h + 1
	}
}

func NewInvalidBest() BestValue {
	return BestValue{
		Id:     ZERO256,
		Height: InvalidHeight,
	}
}

func BestValueBytes(id HASH256, h uint32) []byte {
	v := &BestValue{
		Id:     id,
		Height: h,
	}
	return v.Bytes()
}

func (v BestValue) IsValid() bool {
	return v.Height != InvalidHeight
}

func (v BestValue) Bytes() []byte {
	w := NewWriter()
	err := v.Id.Encode(w)
	if err != nil {
		panic(err)
	}
	err = w.TWrite(v.Height)
	if err != nil {
		panic(err)
	}
	return w.Bytes()
}

func (v *BestValue) From(b []byte) error {
	r := NewReader(b)
	if err := v.Id.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&v.Height); err != nil {
		return err
	}
	return nil
}
