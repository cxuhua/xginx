package xginx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

const (
	SCRIPT_COINBASE_TYPE = uint8(0) //coinbase input 0
	SCRIPT_LOCKED_TYPE   = uint8(1) //标准锁定脚本 HASH160(pubkey)
	SCRIPT_WITNESS_TYPE  = uint8(2) //witness type
)

type Script []byte

func (s Script) Len() int {
	return len(s)
}

func (s Script) Type() uint8 {
	return s[0]
}

func getwitnesssize() int {
	x := WitnessScript{}
	x.Type = SCRIPT_WITNESS_TYPE
	buf := &bytes.Buffer{}
	_ = x.Encode(buf)
	return buf.Len()
}

func getcoinbaseminsize() int {
	return GetCoinbaseScript(0, []byte{}).Len()
}

func getlockedsize() int {
	x := LockedScript{}
	x.Type = SCRIPT_LOCKED_TYPE
	buf := &bytes.Buffer{}
	_ = x.Encode(buf)
	return buf.Len()
}

var (
	lockedsize     = getlockedsize()
	conbaseminsize = getcoinbaseminsize()
)

//in script
func (s Script) IsCoinBase() bool {
	return s.Len() >= conbaseminsize && s.Len() <= 128 && s[0] == SCRIPT_COINBASE_TYPE
}

//in script
func (s Script) IsWitness() bool {
	return s.Len() > 3 && s.Len() < ACCOUNT_KEY_MAX_SIZA*128 && s[0] == SCRIPT_WITNESS_TYPE
}

//out script
func (s Script) IsLocked() bool {
	return s.Len() == lockedsize && s[0] == SCRIPT_LOCKED_TYPE
}

//从锁定脚本获取输出地址
func (s Script) GetAddress() string {
	if !s.IsLocked() {
		panic(errors.New("script type error"))
	}
	addr, err := EncodeAddress(s.GetPkh())
	if err != nil {
		panic(err)
	}
	return addr
}

//coinbase交易没有pkh
func (s Script) GetPkh() HASH160 {
	pkh := HASH160{}
	copy(pkh[:], s[1:1+len(pkh)])
	return pkh
}

//获取coinbase中的区块高度
func (s Script) Height() uint32 {
	return Endian.Uint32(s[1:5])
}

func (s Script) Encode(w IWriter) error {
	return VarBytes(s).Encode(w)
}

func (s Script) ForID(w IWriter) error {
	if s.IsCoinBase() {
		return VarBytes(s).Encode(w)
	} else if wit, err := s.ToWitness(); err != nil {
		return err
	} else {
		return wit.ForID(w)
	}
}

//签名，验证写入
func (s Script) ForVerify(w IWriter) error {
	if wit, err := s.ToWitness(); err != nil {
		return err
	} else {
		return wit.ForID(w)
	}
}

func (s *Script) Decode(r IReader) error {
	return (*VarBytes)(s).Decode(r)
}

func (s Script) ToWitness() (WitnessScript, error) {
	if !s.IsWitness() {
		panic(errors.New("witness error"))
	}
	buf := bytes.NewReader(s)
	wit := WitnessScript{}
	err := wit.Decode(buf)
	if err != nil {
		return wit, err
	}
	return wit, nil
}

func GetCoinbaseScript(h uint32, bs ...[]byte) Script {
	s := Script{SCRIPT_COINBASE_TYPE}
	hb := []byte{0, 0, 0, 0}
	//当前块高度
	Endian.PutUint32(hb, h)
	s = append(s, hb...)
	//加当前时间戳
	Endian.PutUint32(hb, uint32(time.Now().Unix()))
	s = append(s, hb...)
	//加点随机值
	rv := uint32(0)
	SetRandInt(&rv)
	Endian.PutUint32(hb, rv)
	s = append(s, hb...)
	//自定义数据
	for _, v := range bs {
		s = append(s, v...)
	}
	return s
}

//标准锁定脚本
type LockedScript struct {
	Type uint8
	Pkh  HASH160
}

func (ss LockedScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *LockedScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Decode(r); err != nil {
		return err
	}
	return nil
}

func NewLockedScript(pkh HASH160) (Script, error) {
	std := &LockedScript{}
	std.Type = SCRIPT_LOCKED_TYPE
	std.Pkh = pkh
	buf := &bytes.Buffer{}
	err := std.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//隔离见证
type WitnessScript struct {
	Type uint8      //SCRIPT_WITNESS_TYPE
	Num  uint8      //签名数量
	Less uint8      //至少正确的数量
	Pks  []PKBytes  //公钥
	Sig  []SigBytes //签名
}

//id计算
func (ss WitnessScript) ForID(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, ss.Num); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, ss.Less); err != nil {
		return err
	}
	return nil
}

//编码
func (ss WitnessScript) Encode(w IWriter) error {
	if err := binary.Write(w, Endian, ss.Type); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, ss.Num); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, ss.Less); err != nil {
		return err
	}
	if err := binary.Write(w, Endian, uint8(len(ss.Pks))); err != nil {
		return err
	}
	for _, pk := range ss.Pks {
		if err := pk.Encode(w); err != nil {
			return err
		}
	}
	if err := binary.Write(w, Endian, uint8(len(ss.Sig))); err != nil {
		return err
	}
	for _, sig := range ss.Sig {
		if err := sig.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

func (ss *WitnessScript) Decode(r IReader) error {
	if err := binary.Read(r, Endian, &ss.Type); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &ss.Num); err != nil {
		return err
	}
	if err := binary.Read(r, Endian, &ss.Less); err != nil {
		return err
	}
	pnum := uint8(0)
	if err := binary.Read(r, Endian, &pnum); err != nil {
		return err
	}
	ss.Pks = make([]PKBytes, pnum)
	for i, _ := range ss.Pks {
		pk := PKBytes{}
		if err := pk.Decode(r); err != nil {
			return err
		}
		ss.Pks[i] = pk
	}
	snum := uint8(0)
	if err := binary.Read(r, Endian, &snum); err != nil {
		return err
	}
	ss.Sig = make([]SigBytes, snum)
	for i, _ := range ss.Sig {
		sig := SigBytes{}
		if err := sig.Decode(r); err != nil {
			return err
		}
		ss.Sig[i] = sig
	}
	return nil
}

func (ss WitnessScript) Hash() (HASH160, error) {
	return HashPks(ss.Num, ss.Less, ss.Pks)
}

//hash公钥。地址也将又这个方法生成
func HashPks(num uint8, less uint8, pks []PKBytes) (HASH160, error) {
	if int(num) != len(pks) {
		panic(errors.New("pub num error"))
	}
	id := HASH160{}
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, Endian, num); err != nil {
		return id, err
	}
	if err := binary.Write(buf, Endian, less); err != nil {
		return id, err
	}
	for _, pk := range pks {
		buf.Write(pk[:])
	}
	copy(id[:], Hash160(buf.Bytes()))
	return id, nil
}

func (ss WitnessScript) ToScript() (Script, error) {
	buf := &bytes.Buffer{}
	err := ss.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//csp=true 检查签名证书数量
func (ss WitnessScript) Check(csp bool) error {
	if ss.Type != SCRIPT_WITNESS_TYPE {
		return errors.New("type errpor")
	}
	if ss.Num == 0 || ss.Num > 16 || ss.Less == 0 || ss.Less > 16 || ss.Less > ss.Num {
		return errors.New("num less error")
	}
	if csp && len(ss.Pks) != int(ss.Num) {
		return errors.New("pks num error")
	}
	if csp && len(ss.Sig) < int(ss.Less) {
		return errors.New("sig num error")
	}
	return nil
}
