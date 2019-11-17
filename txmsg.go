package xginx

const (
	//交易类型
	InvTypeTx = uint8(1)
	//块类型
	InvTypeBlock = uint8(2)
)

//加以区块库存列表
type Inventory struct {
	Typ uint8
	ID  HASH256
}

func (m Inventory) Encode(w IWriter) error {
	if err := w.TWrite(m.Typ); err != nil {
		return err
	}
	if err := m.ID.Encode(w); err != nil {
		return err
	}
	return nil
}

func (m *Inventory) Decode(r IReader) error {
	if err := r.TRead(&m.Typ); err != nil {
		return err
	}
	if err := m.ID.Decode(r); err != nil {
		return err
	}
	return nil
}

//

type GetInvMsg struct {
	Invs []Inventory
}

func (m GetInvMsg) Type() uint8 {
	return NT_GET_INV
}

func (m *GetInvMsg) AddInv(typ uint8, id HASH256) {
	m.Invs = append(m.Invs, Inventory{
		Typ: typ,
		ID:  id,
	})
}

func (m GetInvMsg) Encode(w IWriter) error {
	if err := VarInt(len(m.Invs)).Encode(w); err != nil {
		return err
	}
	for _, v := range m.Invs {
		if err := v.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

func (m *GetInvMsg) Decode(r IReader) error {
	num := VarInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Invs = make([]Inventory, num.ToInt())
	for i, _ := range m.Invs {
		v := Inventory{}
		err := v.Decode(r)
		if err != nil {
			return err
		}
		m.Invs[i] = v
	}
	return nil
}

//

type InvMsg struct {
	Invs []Inventory
}

func (m InvMsg) Type() uint8 {
	return NT_INV
}

func (m *InvMsg) AddInv(typ uint8, id HASH256) {
	m.Invs = append(m.Invs, Inventory{
		Typ: typ,
		ID:  id,
	})
}

func (m InvMsg) Encode(w IWriter) error {
	if err := VarInt(len(m.Invs)).Encode(w); err != nil {
		return err
	}
	for _, v := range m.Invs {
		if err := v.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

func (m *InvMsg) Decode(r IReader) error {
	num := VarInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Invs = make([]Inventory, num.ToInt())
	for i, _ := range m.Invs {
		v := Inventory{}
		err := v.Decode(r)
		if err != nil {
			return err
		}
		m.Invs[i] = v
	}
	return nil
}

//NT_BLOCK

type BlockMsg struct {
	Blk BlockInfo
}

func (m BlockMsg) Type() uint8 {
	return NT_BLOCK
}

func (m BlockMsg) Encode(w IWriter) error {
	return m.Blk.Encode(w)
}

func (m *BlockMsg) Decode(r IReader) error {
	return m.Blk.Decode(r)
}

// NT_TX
type TxMsg struct {
	Tx TX
}

func (m TxMsg) Type() uint8 {
	return NT_TX
}

func (m TxMsg) Encode(w IWriter) error {
	return m.Tx.Encode(w)
}

func (m *TxMsg) Decode(r IReader) error {
	return m.Tx.Decode(r)
}
