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

//获取参数发送相关数据
func (bi *BlockIndex) GetMsgGetInv(msg *MsgGetInv, c *Client) {
	for _, inv := range msg.Invs {
		if inv.Typ == InvTypeTx {
			tx, err := bi.txp.Get(inv.ID)
			if err != nil {
				tx, err = bi.LoadTX(inv.ID)
			}
			if err != nil {
				continue
			}
			c.SendMsg(NewMsgTx(tx))
		} else if inv.Typ == InvTypeBlock {
			blk, err := bi.LoadBlock(inv.ID)
			if err != nil {
				continue
			}
			c.SendMsg(NewMsgBlock(blk))
		}
	}
}

//

type MsgGetInv struct {
	Invs []Inventory
}

func (m MsgGetInv) Type() uint8 {
	return NT_GET_INV
}

func (m *MsgGetInv) AddInv(typ uint8, id HASH256) {
	m.Invs = append(m.Invs, Inventory{
		Typ: typ,
		ID:  id,
	})
}

func (m MsgGetInv) Encode(w IWriter) error {
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

func (m *MsgGetInv) Decode(r IReader) error {
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

//交易消息

type MsgInv struct {
	Invs []Inventory
}

func (m MsgInv) Type() uint8 {
	return NT_INV
}

func (m *MsgInv) AddInv(typ uint8, id HASH256) {
	m.Invs = append(m.Invs, Inventory{
		Typ: typ,
		ID:  id,
	})
}

func (m MsgInv) Encode(w IWriter) error {
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

func (m *MsgInv) Decode(r IReader) error {
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

// NT_TX
type MsgTx struct {
	Tx *TX
}

func NewMsgTx(tx *TX) *MsgTx {
	return &MsgTx{Tx: tx}
}

func (m MsgTx) Type() uint8 {
	return NT_TX
}

func (m MsgTx) Encode(w IWriter) error {
	return m.Tx.Encode(w)
}

func (m *MsgTx) Decode(r IReader) error {
	m.Tx = &TX{}
	return m.Tx.Decode(r)
}
