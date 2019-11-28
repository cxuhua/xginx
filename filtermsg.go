package xginx

//设置过滤器，调用后client的bloom生效
//过滤器设置后只会提交符合条件的信息给客户端
type MsgFilterLoad struct {
	Funcs  uint32
	Tweak  uint32
	Filter VarBytes
}

func (m MsgFilterLoad) Type() uint8 {
	return NT_FILTER_LOAD
}

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

//添加过滤器key

type MsgFilterAdd struct {
	Key VarBytes
}

func (m MsgFilterAdd) Type() uint8 {
	return NT_FILTER_ADD
}

func (m MsgFilterAdd) Encode(w IWriter) error {
	return m.Key.Encode(w)
}

func (m *MsgFilterAdd) Decode(r IReader) error {
	return m.Key.Decode(r)
}

//清除过滤器

type MsgFilterClear struct {
}

func (m MsgFilterClear) Type() uint8 {
	return NT_FILTER_CLEAR
}

func (m MsgFilterClear) Encode(w IWriter) error {
	return nil
}

func (m *MsgFilterClear) Decode(r IReader) error {
	return nil
}
