package xginx

//MsgFilterLoad 设置过滤器，调用后client的bloom生效
//过滤器设置后只会提交符合条件的信息给客户端
type MsgFilterLoad struct {
	Funcs  uint32
	Tweak  uint32
	Filter VarBytes
}

//Type 消息类型
func (m MsgFilterLoad) Type() NTType {
	return NtFilterLoad
}

//ID 消息ID
func (m MsgFilterLoad) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Encode 编码消息
func (m MsgFilterLoad) Encode(w IWriter) error {
	if err := w.TWrite(m.Funcs); err != nil {
		return err
	}
	if err := w.TWrite(m.Tweak); err != nil {
		return err
	}
	if err := m.Filter.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码消息
func (m *MsgFilterLoad) Decode(r IReader) error {
	if err := r.TRead(&m.Funcs); err != nil {
		return err
	}
	if err := r.TRead(&m.Tweak); err != nil {
		return err
	}
	if err := m.Filter.Decode(r); err != nil {
		return err
	}
	return nil
}

//MsgFilterAdd 添加过滤器key
type MsgFilterAdd struct {
	Key VarBytes
}

//Type 消息类型
func (m MsgFilterAdd) Type() NTType {
	return NtFilterAdd
}

//ID 消息ID
func (m MsgFilterAdd) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Encode 编码消息
func (m MsgFilterAdd) Encode(w IWriter) error {
	return m.Key.Encode(w)
}

//Decode 解码消息
func (m *MsgFilterAdd) Decode(r IReader) error {
	return m.Key.Decode(r)
}

//MsgFilterClear 清除过滤器
type MsgFilterClear struct {
}

//ID 消息ID
func (m MsgFilterClear) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Type 消息类型
func (m MsgFilterClear) Type() NTType {
	return NtFilterClear
}

//Encode 编码消息
func (m MsgFilterClear) Encode(w IWriter) error {
	return nil
}

//Decode 解码消息
func (m *MsgFilterClear) Decode(r IReader) error {
	return nil
}
