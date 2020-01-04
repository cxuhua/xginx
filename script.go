package xginx

import (
	"errors"
)

const (
	InvalidArb           = ^uint8(0) //无效的仲裁
	SCRIPT_COINBASE_TYPE = uint8(0)  //coinbase script
	SCRIPT_LOCKED_TYPE   = uint8(1)  //标准锁定脚本
	SCRIPT_WITNESS_TYPE  = uint8(2)  //隔离见证多重签名脚本
)

type Script []byte

func (s Script) Len() int {
	return len(s)
}

func (s Script) Type() uint8 {
	return s[0]
}

func getwitnessminsize() int {
	pri, err := NewPrivateKey()
	if err != nil {
		panic(err)
	}
	x := WitnessScript{}
	x.Type = SCRIPT_WITNESS_TYPE
	x.Pks = append(x.Pks, pri.PublicKey().GetPks())
	buf := NewWriter()
	_ = x.Encode(buf)
	return buf.Len()
}

func getcoinbaseminsize() int {
	return NewCoinbaseScript(0, []byte{}).Len()
}

func getlockedminsize() int {
	x := LockedScript{}
	x.Type = SCRIPT_LOCKED_TYPE
	buf := NewWriter()
	_ = x.Encode(buf)
	return buf.Len()
}

var (
	lockedminsize  = getlockedminsize()
	conbaseminsize = getcoinbaseminsize()
	witnessminsize = getwitnessminsize()
)

//in script
func (s Script) IsCoinBase() bool {
	return s.Len() >= conbaseminsize && s.Len() <= 128 && s[0] == SCRIPT_COINBASE_TYPE
}

//in script
func (s Script) IsWitness() bool {
	return s.Len() >= witnessminsize && s.Len() < ACCOUNT_KEY_MAX_SIZE*128 && s[0] == SCRIPT_WITNESS_TYPE
}

//out script
func (s Script) IsLocked() bool {
	return s.Len() >= lockedminsize && s.Len() < (lockedminsize+MAX_EXT_SIZE+4) && s[0] == SCRIPT_LOCKED_TYPE
}

//从锁定脚本获取输出地址
func (s Script) GetAddress() (Address, error) {
	if pkh, err := s.GetPkh(); err != nil {
		return "", err
	} else if addr, err := EncodeAddress(pkh); err != nil {
		return "", err
	} else {
		return addr, nil
	}
}

func (s Script) MustPkh() HASH160 {
	pkh, err := s.GetPkh()
	if err != nil {
		panic(err)
	}
	return pkh
}

//coinbase交易没有pkh
func (s Script) GetPkh() (HASH160, error) {
	pkh := HASH160{}
	if s.IsLocked() {
		ss, err := s.ToLocked()
		if err != nil {
			return pkh, err
		}
		return ss.Pkh, nil
	} else if s.IsWitness() {
		ws, err := s.ToWitness()
		if err != nil {
			return pkh, err
		}
		return ws.Hash()
	} else {
		return pkh, errors.New("script typ not pkh")
	}
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
		return s.Encode(w)
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

func (s Script) ToLocked() (LockedScript, error) {
	rs := LockedScript{}
	if !s.IsLocked() {
		return rs, errors.New("script type error")
	}
	buf := NewReader(s)
	err := rs.Decode(buf)
	if err != nil {
		return rs, err
	}
	return rs, nil
}

func (s Script) ToWitness() (WitnessScript, error) {
	wit := WitnessScript{}
	if !s.IsWitness() {
		return wit, errors.New("witness error")
	}
	buf := NewReader(s)
	err := wit.Decode(buf)
	if err != nil {
		return wit, err
	}
	return wit, nil
}

func NewCoinbaseScript(h uint32, ip []byte, bs ...[]byte) Script {
	s := Script{SCRIPT_COINBASE_TYPE}
	hb := []byte{0, 0, 0, 0}
	//当前块高度必须存在
	Endian.PutUint32(hb, h)
	s = append(s, hb...)
	//加入ip地址
	s = append(s, ip...)
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
	Ext  VarBytes
}

func (ss LockedScript) Encode(w IWriter) error {
	if err := w.TWrite(ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Encode(w); err != nil {
		return err
	}
	if err := ss.Ext.Encode(w); err != nil {
		return err
	}
	return nil
}

func (ss *LockedScript) Decode(r IReader) error {
	if err := r.TRead(&ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Decode(r); err != nil {
		return err
	}
	if err := ss.Ext.Decode(r); err != nil {
		return err
	}
	return nil
}

func NewLockedScript(pkh HASH160, vbs ...[]byte) (Script, error) {
	std := &LockedScript{Ext:VarBytes{}}
	std.Type = SCRIPT_LOCKED_TYPE
	std.Pkh = pkh
	for _, vb := range vbs {
		if len(vb) > MAX_EXT_SIZE {
			return nil, errors.New("ext size > MAX_EXT_SIZE")
		}
		std.Ext = append(std.Ext, vb...)
	}
	if std.Ext.Len() > MAX_EXT_SIZE {
		return nil, errors.New("ext size > MAX_EXT_SIZE")
	}
	buf := NewWriter()
	err := std.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//隔离见证脚本
type WitnessScript struct {
	Type uint8      //SCRIPT_WITNESS_TYPE
	Num  uint8      //签名数量
	Less uint8      //至少正确的数量
	Arb  uint8      //是否启用仲裁
	Pks  []PKBytes  //公钥
	Sig  []SigBytes //签名
}

//是否启用仲裁
func (ss WitnessScript) IsEnableArb() bool {
	return ss.Arb != InvalidArb
}

//id计算
func (ss WitnessScript) ForID(w IWriter) error {
	if err := w.TWrite(ss.Type); err != nil {
		return err
	}
	if err := w.TWrite(ss.Num); err != nil {
		return err
	}
	if err := w.TWrite(ss.Less); err != nil {
		return err
	}
	if err := w.TWrite(ss.Arb); err != nil {
		return err
	}
	return nil
}

//编码
func (ss WitnessScript) Encode(w IWriter) error {
	if err := w.TWrite(ss.Type); err != nil {
		return err
	}
	if err := w.TWrite(ss.Num); err != nil {
		return err
	}
	if err := w.TWrite(ss.Less); err != nil {
		return err
	}
	if err := w.TWrite(ss.Arb); err != nil {
		return err
	}
	//公钥数量和公钥
	if err := w.TWrite(uint8(len(ss.Pks))); err != nil {
		return err
	}
	for _, pk := range ss.Pks {
		if err := pk.Encode(w); err != nil {
			return err
		}
	}
	//签名数量和签名
	if err := w.TWrite(uint8(len(ss.Sig))); err != nil {
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
	num := uint8(0)
	if err := r.TRead(&ss.Type); err != nil {
		return err
	}
	if err := r.TRead(&ss.Num); err != nil {
		return err
	}
	if err := r.TRead(&ss.Less); err != nil {
		return err
	}
	if err := r.TRead(&ss.Arb); err != nil {
		return err
	}
	if err := r.TRead(&num); err != nil {
		return err
	}
	ss.Pks = make([]PKBytes, num)
	for i, _ := range ss.Pks {
		pk := PKBytes{}
		if err := pk.Decode(r); err != nil {
			return err
		}
		ss.Pks[i] = pk
	}
	if err := r.TRead(&num); err != nil {
		return err
	}
	ss.Sig = make([]SigBytes, num)
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
	return HashPks(ss.Num, ss.Less, ss.Arb, ss.Pks)
}

//hash公钥。地址hash也将由这个方法生成
func HashPks(num uint8, less uint8, arb uint8, pks []PKBytes) (HASH160, error) {
	id := ZERO160
	if int(num) != len(pks) {
		return id, errors.New("pub num error")
	}
	if less > num {
		return id, errors.New("args less num error")
	}
	if less == num && arb != InvalidArb {
		return id, errors.New("args less num arb error")
	}
	if arb != InvalidArb && arb != num-1 {
		return id, errors.New("args num arb error")
	}
	w := NewWriter()
	if err := w.TWrite(num);err != nil {
		return id, err
	}
	if err = w.TWrite(less);err != nil {
		return id, err
	}
	if err = w.TWrite(arb);err != nil {
		return id, err
	}
	for _, pk := range pks {
		err := w.TWrite(pk[:])
		if err != nil {
			return id, err
		}
	}
	id = Hash160From(w.Bytes())
	return id, nil
}

func (ss WitnessScript) ToScript() (Script, error) {
	buf := NewWriter()
	err := ss.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//检查指定的签名
func (ss WitnessScript) CheckSigs(sigs []SigBytes) error {
	if ss.Type != SCRIPT_WITNESS_TYPE {
		return errors.New("type errpor")
	}
	if ss.Num == 0 || ss.Num > ACCOUNT_KEY_MAX_SIZE {
		return errors.New("num error")
	}
	if ss.Less == 0 || ss.Less > ACCOUNT_KEY_MAX_SIZE || ss.Less > ss.Num {
		return errors.New("less error")
	}
	//启用arb的情况下，less不能和num相等
	if ss.IsEnableArb() && ss.Num == ss.Less {
		return errors.New("arb set error")
	}
	//仲裁签名必须是最后一个
	if ss.IsEnableArb() && ss.Arb != ss.Num-1 {
		return errors.New("arb set error")
	}
	//公钥数量必须正确
	if len(ss.Pks) != int(ss.Num) {
		return errors.New("pks num error")
	}
	if ss.IsEnableArb() && len(sigs) < 1 {
		return errors.New("arb sig num error")
	} else if !ss.IsEnableArb() && len(sigs) < int(ss.Less) {
		return errors.New("sig num error")
	}
	return nil
}

//csp=true 检查签名证书数量
func (ss WitnessScript) Check() error {
	return ss.CheckSigs(ss.Sig)
}
