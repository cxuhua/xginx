package xginx

//定义高度
const (
	// 无效的块高度
	InvalidHeight = ^uint32(0)
)

// 区块定义
var (
	BestBlockKey = []byte("BestBlockKey") //最高区块数据保存key
	InvalidBest  = NewInvalidBest()       //无效的状态
)

//BestValue 最优区块
type BestValue struct {
	ID     HASH256
	Height uint32
}

//Curr 获取当前高度
func (bv BestValue) Curr() uint32 {
	if !bv.IsValid() {
		return 0
	}
	return bv.Height
}

//LastID 最后一个区块ID
func (bv BestValue) LastID() HASH256 {
	if bv.IsValid() {
		return bv.ID
	}
	return conf.genesis
}

//Next 获取下一个高度
func (bv BestValue) Next() uint32 {
	return NextHeight(bv.Height)
}

//NextHeight 获取下一个高度
func NextHeight(h uint32) uint32 {
	if h == InvalidHeight {
		return 0
	}
	return h + 1
}

//NewInvalidBest 不合法的区块高度
func NewInvalidBest() BestValue {
	return BestValue{
		ID:     ZERO256,
		Height: InvalidHeight,
	}
}

//BestValueBytes 编码区块状态数据
func BestValueBytes(id HASH256, h uint32) []byte {
	v := &BestValue{
		ID:     id,
		Height: h,
	}
	return v.Bytes()
}

//IsValid 高度是否有效
func (bv BestValue) IsValid() bool {
	return bv.Height != InvalidHeight
}

//Bytes 生成二进制数据用于存储
func (bv BestValue) Bytes() []byte {
	w := NewWriter()
	err := bv.ID.Encode(w)
	if err != nil {
		panic(err)
	}
	err = w.TWrite(bv.Height)
	if err != nil {
		panic(err)
	}
	return w.Bytes()
}

//From 从二进制数据获取
func (bv *BestValue) From(b []byte) error {
	r := NewReader(b)
	if err := bv.ID.Decode(r); err != nil {
		return err
	}
	if err := r.TRead(&bv.Height); err != nil {
		return err
	}
	return nil
}
