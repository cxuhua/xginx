package xginx

import (
	"crypto/md5"
	"errors"
)

//MsgGetMerkle 获取交易验证merkle树
type MsgGetMerkle struct {
	TxID HASH256 //交易id
}

//Type 消息类型
func (m MsgGetMerkle) Type() NTType {
	return NtGetMerkle

}

//ID 消息ID
func (m MsgGetMerkle) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Encode 编码消息
func (m MsgGetMerkle) Encode(w IWriter) error {
	return m.TxID.Encode(w)
}

//Decode 解码消息
func (m *MsgGetMerkle) Decode(r IReader) error {
	return m.TxID.Decode(r)
}

//MsgTxMerkle 返回交易验证merkle树
type MsgTxMerkle struct {
	TxID  HASH256   //当前交易id
	Trans VarInt    //交易锁在块的交易数量
	Hashs []HASH256 //基于merkle树的验证hash
	Bits  VarBytes  //
}

//Type 消息类型
func (m MsgTxMerkle) Type() NTType {
	return NtTxMerkle
}

//ID 消息ID
func (m MsgTxMerkle) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Verify 验证数据
func (m MsgTxMerkle) Verify(bi *BlockIndex) error {
	bits := BitSetFrom(m.Bits)
	nt := GetMerkleTree(m.Trans.ToInt(), m.Hashs, bits)
	merkle, err := nt.ExtractRoot()
	if err != nil {
		return err
	}
	txv, err := bi.LoadTxValue(m.TxID)
	if err != nil {
		return err
	}
	bh, err := bi.GetBlockHeader(txv.BlkID)
	if err != nil {
		return err
	}
	if !bh.Merkle.Equal(merkle) {
		return errors.New("merkle verify error")
	}
	return nil
}

//Encode 编码消息
func (m MsgTxMerkle) Encode(w IWriter) error {
	if err := m.TxID.Encode(w); err != nil {
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

//Decode 解码消息
func (m *MsgTxMerkle) Decode(r IReader) error {
	if err := m.TxID.Decode(r); err != nil {
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
	for i := range m.Hashs {
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

//inv类型定义
const (
	//交易类型
	InvTypeTx = uint8(1)
	//块类型
	InvTypeBlock = uint8(2)
)

//Inventory 加以区块库存列表
type Inventory struct {
	InvType uint8
	InvID   HASH256
}

//Encode 编码消息
func (m Inventory) Encode(w IWriter) error {
	if err := w.TWrite(m.InvType); err != nil {
		return err
	}
	if err := m.InvID.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码消息
func (m *Inventory) Decode(r IReader) error {
	if err := r.TRead(&m.InvType); err != nil {
		return err
	}
	if err := m.InvID.Decode(r); err != nil {
		return err
	}
	return nil
}

//GetMsgGetInv 获取参数发送相关数据
func (bi *BlockIndex) GetMsgGetInv(msg *MsgGetInv, c *Client) {
	for _, inv := range msg.Invs {
		if inv.InvType == InvTypeTx {
			tx, err := bi.txp.Get(inv.InvID)
			if err != nil {
				tx, err = bi.LoadTX(inv.InvID)
			}
			if err != nil {
				continue
			}
			c.SendMsg(NewMsgTx(tx))
		} else if inv.InvType == InvTypeBlock {
			blk, err := bi.LoadBlock(inv.InvID)
			if err != nil {
				continue
			}
			c.SendMsg(NewMsgBlock(blk))
		}
	}
}

//MsgGetInv 获取库存
type MsgGetInv struct {
	Invs []Inventory
}

//Type 消息类型
func (m MsgGetInv) Type() NTType {
	return NtGetInv
}

//ID 消息ID
func (m MsgGetInv) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//AddInv 添加库存
func (m *MsgGetInv) AddInv(typ uint8, id HASH256) {
	m.Invs = append(m.Invs, Inventory{
		InvType: typ,
		InvID:   id,
	})
}

//Encode 编码消息
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

//Decode 解码消息
func (m *MsgGetInv) Decode(r IReader) error {
	num := VarInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Invs = make([]Inventory, num.ToInt())
	for i := range m.Invs {
		v := Inventory{}
		err := v.Decode(r)
		if err != nil {
			return err
		}
		m.Invs[i] = v
	}
	return nil
}

//MsgInv 交易消息
type MsgInv struct {
	Invs []Inventory
}

//Type 消息类型
func (m MsgInv) Type() NTType {
	return NtInv
}

//ID 消息ID
func (m MsgInv) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//AddInv 添加
func (m *MsgInv) AddInv(typ uint8, id HASH256) {
	m.Invs = append(m.Invs, Inventory{
		InvType: typ,
		InvID:   id,
	})
}

//Encode 编码
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

//Decode 解码
func (m *MsgInv) Decode(r IReader) error {
	num := VarInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Invs = make([]Inventory, num.ToInt())
	for i := range m.Invs {
		v := Inventory{}
		err := v.Decode(r)
		if err != nil {
			return err
		}
		m.Invs[i] = v
	}
	return nil
}

//MsgGetTxPool 获取本节点没有的
type MsgGetTxPool struct {
	Skip []HASH256 //忽略的交易id
}

//Type 消息类型
func (m MsgGetTxPool) Type() NTType {
	return NtGetTxPool
}

//Add 添加不需要的交易Id
func (m *MsgGetTxPool) Add(id HASH256) {
	m.Skip = append(m.Skip, id)
}

//ID 消息ID
func (m MsgGetTxPool) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Has 检测id是否在忽略列表中
func (m MsgGetTxPool) Has(id HASH256) bool {
	for _, v := range m.Skip {
		if v.Equal(id) {
			return true
		}
	}
	return false
}

//Encode 编码消息
func (m MsgGetTxPool) Encode(w IWriter) error {
	if err := VarUInt(len(m.Skip)).Encode(w); err != nil {
		return err
	}
	for _, v := range m.Skip {
		err := v.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

//Decode 解码消息
func (m *MsgGetTxPool) Decode(r IReader) error {
	num := VarUInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Skip = make([]HASH256, num.ToInt())
	for i := range m.Skip {
		id := HASH256{}
		err := id.Decode(r)
		if err != nil {
			return err
		}
		m.Skip[i] = id
	}
	return nil
}

//MsgTxPool 返回交易池数据
type MsgTxPool struct {
	Txs []*TX
}

//Type 消息类型
func (m MsgTxPool) Type() NTType {
	return NtTxPool
}

//ID 消息ID
func (m MsgTxPool) ID() (MsgID, error) {
	return ErrMsgID, ErrNotID
}

//Add 添加交易
func (m *MsgTxPool) Add(tx *TX) {
	m.Txs = append(m.Txs, tx)
}

//Encode 编码消息
func (m MsgTxPool) Encode(w IWriter) error {
	if err := VarUInt(len(m.Txs)).Encode(w); err != nil {
		return err
	}
	for _, tx := range m.Txs {
		err := tx.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

//Decode 解码消息
func (m *MsgTxPool) Decode(r IReader) error {
	num := VarUInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Txs = make([]*TX, num.ToInt())
	for i := range m.Txs {
		tx := &TX{}
		err := tx.Decode(r)
		if err != nil {
			return err
		}
		m.Txs[i] = tx
	}
	return nil
}

//MsgTx 交易信息
type MsgTx struct {
	Txs []*TX
}

//NewMsgTx 从tx创建交易消息
func NewMsgTx(tx *TX) *MsgTx {
	return &MsgTx{Txs: []*TX{tx}}
}

//ID 交易ID
func (m MsgTx) ID() (MsgID, error) {
	sum := md5.New()
	for _, v := range m.Txs {
		id, err := v.ID()
		if err != nil {
			return ErrMsgID, err
		}
		sum.Write(id[:])
	}
	id := MsgID{}
	copy(id[:], sum.Sum(nil))
	return id, nil
}

//Type 消息类型
func (m MsgTx) Type() NTType {
	return NtTx
}

//Add 添加交易
func (m *MsgTx) Add(tx *TX) {
	m.Txs = append(m.Txs, tx)
}

//Encode 编码消息
func (m MsgTx) Encode(w IWriter) error {
	if err := VarUInt(len(m.Txs)).Encode(w); err != nil {
		return err
	}
	for _, tx := range m.Txs {
		err := tx.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

//Decode 解码消息
func (m *MsgTx) Decode(r IReader) error {
	num := VarUInt(0)
	if err := num.Decode(r); err != nil {
		return err
	}
	m.Txs = make([]*TX, num.ToInt())
	for i := range m.Txs {
		tx := &TX{}
		err := tx.Decode(r)
		if err != nil {
			return err
		}
		m.Txs[i] = tx
	}
	return nil
}
