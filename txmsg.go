package xginx

import "errors"

//获取交易验证merkle树
type MsgGetMerkle struct {
	TxId HASH256 //交易id
}

func (m MsgGetMerkle) Type() uint8 {
	return NT_GET_MERKLE
}

func (m MsgGetMerkle) Encode(w IWriter) error {
	return m.TxId.Encode(w)
}

func (m *MsgGetMerkle) Decode(r IReader) error {
	return m.TxId.Decode(r)
}

//返回交易验证merkle树
type MsgTxMerkle struct {
	TxId  HASH256   //当前交易id
	Trans VarInt    //交易锁在块的交易数量
	Hashs []HASH256 //基于merkle树的验证hash
	Bits  VarBytes  //
}

func (m MsgTxMerkle) Type() uint8 {
	return NT_TX_MERKLE
}

func (m MsgTxMerkle) Verify(bi *BlockIndex) error {
	bits := NewBitSet(m.Bits)
	nt := GetMerkleTree(m.Trans.ToInt(), m.Hashs, bits)
	root, hashs, idx := nt.Extract()
	if len(idx) != 1 || !hashs[0].Equal(m.TxId) {
		return errors.New("id not found,veriry error")
	}
	txv, err := bi.LoadTxValue(m.TxId)
	if err != nil {
		return err
	}
	bh, err := bi.GetBlockHeader(txv.BlkId)
	if err != nil {
		return err
	}
	if !bh.Merkle.Equal(root) {
		return errors.New("merkle verify error")
	}
	return nil
}

func (m MsgTxMerkle) Encode(w IWriter) error {
	if err := m.TxId.Encode(w); err != nil {
		return err
	}
	if err := m.Trans.Encode(w); err != nil {
		return err
	}
	if err := VarInt(len(m.Hashs)).Encode(w); err != nil {
		return err
	}
	for _, v := range m.Hashs {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	if err := m.Bits.Encode(w); err != nil {
		return err
	}
	return nil
}

func (m *MsgTxMerkle) Decode(r IReader) error {
	if err := m.TxId.Decode(r); err != nil {
		return err
	}
	if err := m.Trans.Decode(r); err != nil {
		return err
	}
	num := VarInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Hashs = make([]HASH256, num.ToInt())
	for i, _ := range m.Hashs {
		v := HASH256{}
		err := v.Decode(r)
		if err != nil {
			return err
		}
		m.Hashs[i] = v
	}
	if err := m.Bits.Decode(r); err != nil {
		return err
	}
	return nil
}

//

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
