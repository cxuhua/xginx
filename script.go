package xginx

import (
	"errors"
	"fmt"
	"net"
)

//脚本类型定义
//Script 第一个字节表示脚本类型
const (
	InvalidArb            = ^uint8(0) //无效的仲裁
	ScriptCoinbaseType    = uint8(0)  //coinbase script
	ScriptLockedType      = uint8(1)  //标准锁定脚本 用于输出
	ScriptWitnessType     = uint8(2)  //隔离见证多重签名脚本 用于输入
	ScriptTxType          = uint8(3)  //交易脚本 用于控制交易是否打包进区块，是否进入交易池，是否发布上网
	MaxCoinbaseScriptSize = 256       //最大coinbase脚本长度
)

//TxScript 交易脚本
type TxScript struct {
	Type uint8
	//脚本最大执行时间，时间一半分配给交易脚本，一半分配给签名脚本
	//签名脚本每个输入签名只有 n分之一的一半时间 n为输入数量
	//单位:毫秒
	ExeTime uint32
	//执行脚本
	Exec VarBytes
}

//Encode 编码
func (ss TxScript) Encode(w IWriter) error {
	if err := w.TWrite(ss.Type); err != nil {
		return err
	}
	if err := w.TWrite(ss.ExeTime); err != nil {
		return err
	}
	if err := ss.Exec.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码
func (ss *TxScript) Decode(r IReader) error {
	if err := r.TRead(&ss.Type); err != nil {
		return err
	}
	if err := r.TRead(&ss.ExeTime); err != nil {
		return err
	}
	if err := ss.Exec.Decode(r); err != nil {
		return err
	}
	return nil
}

//MergeScript 合并脚本
func MergeScript(execs ...[]byte) (VarBytes, error) {
	result := VarBytes{}
	for _, ext := range execs {
		result = append(result, ext...)
	}
	if result.Len() > MaxExecSize {
		return nil, errors.New("execs size > MaxExecSize")
	}
	return result, nil
}

//NewTxScript 创建交易脚本
func NewTxScript(exetime uint32, execs ...[]byte) (Script, error) {
	std := &TxScript{Exec: VarBytes{}}
	std.Type = ScriptTxType
	std.ExeTime = exetime
	exec, err := MergeScript(execs...)
	if err != nil {
		return nil, err
	}
	std.Exec = exec
	buf := NewWriter()
	err = std.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gettxminsize() int {
	x := TxScript{}
	x.Type = ScriptTxType
	buf := NewWriter()
	_ = x.Encode(buf)
	return buf.Len()
}

//Script 脚本定义
type Script []byte

//Clone 复制脚本
func (s Script) Clone() Script {
	n := make(Script, len(s))
	copy(n, s)
	return n
}

//Len 脚本长度
func (s Script) Len() int {
	return len(s)
}

//Check 检测脚本错误
func (s Script) Check() error {
	if s.IsCoinBase() {
		return nil
	}
	if s.IsTxScript() {
		txs, err := s.ToTxScript()
		if err != nil {
			return err
		}
		if txs.Exec.Len() > MaxExecSize {
			return fmt.Errorf("tx script too big")
		}
		return nil
	}
	if s.IsLocked() {
		lcks, err := s.ToLocked()
		if err != nil {
			return err
		}
		if lcks.Exec.Len() > MaxExecSize {
			return fmt.Errorf("out script too big")
		}
		return nil
	}
	if s.IsWitness() {
		wits, err := s.ToWitness()
		if err != nil {
			return err
		}
		if wits.Exec.Len() > MaxExecSize {
			return fmt.Errorf("in script too big")
		}
		return nil
	}
	return fmt.Errorf("script type error")
}

//Type 脚本类型
func (s Script) Type() uint8 {
	return s[0]
}

func getwitnessminsize() int {
	pri, err := NewPrivateKey()
	if err != nil {
		panic(err)
	}
	x := WitnessScript{}
	x.Type = ScriptWitnessType
	x.Pks = append(x.Pks, pri.PublicKey().GetPks())
	buf := NewWriter()
	_ = x.Encode(buf)
	return buf.Len()
}

func getcoinbaseminsize() int {
	s, err := NewCoinbaseScript(0, net.ParseIP("127.0.0.1"))
	if err != nil {
		panic(err)
	}
	return s.Len()
}

func getlockedminsize() int {
	x := LockedScript{}
	x.Type = ScriptLockedType
	buf := NewWriter()
	_ = x.Encode(buf)
	return buf.Len()
}

var (
	lockedminsize  = getlockedminsize()
	conbaseminsize = getcoinbaseminsize()
	witnessminsize = getwitnessminsize()
	txminsize      = gettxminsize()
)

//IsCoinBase 是否是coinbase脚本
func (s Script) IsCoinBase() bool {
	return s.Len() >= conbaseminsize && s.Len() <= MaxCoinbaseScriptSize && s[0] == ScriptCoinbaseType
}

//IsWitness 是否是隔离见证脚本
func (s Script) IsWitness() bool {
	return s.Len() >= witnessminsize && s.Len() < (AccountKeyMaxSize*128+MaxExecSize) && s[0] == ScriptWitnessType
}

//IsLocked 是否是锁定脚本
func (s Script) IsLocked() bool {
	return s.Len() >= lockedminsize && s.Len() < (lockedminsize+MaxExecSize+4) && s[0] == ScriptLockedType
}

//IsTxScript 是否是交易脚本
func (s Script) IsTxScript() bool {
	return s.Len() >= txminsize && s.Len() < (txminsize+MaxExecSize) && s[0] == ScriptTxType
}

//GetAddress 从锁定脚本获取输出地址
func (s Script) GetAddress() (Address, error) {
	if pkh, err := s.GetPkh(); err != nil {
		return "", err
	} else if addr, err := EncodeAddress(pkh); err != nil {
		return "", err
	} else {
		return addr, nil
	}
}

//MustPkh 获取公钥hash
func (s Script) MustPkh() HASH160 {
	pkh, err := s.GetPkh()
	if err != nil {
		panic(err)
	}
	return pkh
}

//GetPkh coinbase交易没有pkh
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
		return pkh, errors.New("script not pkh")
	}
}

//Encode 编码脚本
func (s Script) Encode(w IWriter) error {
	return VarBytes(s).Encode(w)
}

//ForID 为计算id编码
func (s Script) ForID(w IWriter) error {
	if s.IsCoinBase() {
		return s.Encode(w)
	} else if wit, err := s.ToWitness(); err != nil {
		return err
	} else {
		return wit.ForID(w)
	}
}

//ForVerify 签名，验证写入
func (s Script) ForVerify(w IWriter) error {
	wit, err := s.ToWitness()
	if err != nil {
		return err
	}
	return wit.ForID(w)
}

//Decode 解码脚本
func (s *Script) Decode(r IReader) error {
	return (*VarBytes)(s).Decode(r)
}

//ToTxScript 如果是锁定脚本
func (s Script) ToTxScript() (TxScript, error) {
	rs := TxScript{}
	if !s.IsTxScript() {
		return rs, errors.New("script type error")
	}
	buf := NewReader(s)
	err := rs.Decode(buf)
	if err != nil {
		return rs, err
	}
	return rs, nil
}

//ToLocked 如果是锁定脚本
func (s Script) ToLocked() (*LockedScript, error) {
	rs := &LockedScript{}
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

//ToWitness 如果是隔离见证脚本
func (s Script) ToWitness() (*WitnessScript, error) {
	wit := &WitnessScript{}
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

//NewCoinbaseScript 创建coinbase脚本
func NewCoinbaseScript(h uint32, ip []byte, bs ...[]byte) (Script, error) {
	if len(ip) != net.IPv6len {
		return nil, fmt.Errorf("ip length error %d", len(ip))
	}
	s := Script{ScriptCoinbaseType}
	hb := []byte{0, 0, 0, 0}
	//当前块区块高度
	Endian.PutUint32(hb, h)
	s = append(s, hb...)
	//加入节点ip地址
	s = append(s, ip...)
	//自定义数据
	for _, v := range bs {
		s = append(s, v...)
	}
	if s.Len() > MaxCoinbaseScriptSize {
		return nil, fmt.Errorf("coinbase script too long  length = %d", s.Len())
	}
	return s, nil
}

//Data 获取coinbase中的自定义数据
func (s Script) Data() []byte {
	if !s.IsCoinBase() {
		panic(errors.New("script not coinbase type"))
	}
	return s[5+16:]
}

//IP 获取coinbase中的区块高度
func (s Script) IP() []byte {
	if !s.IsCoinBase() {
		panic(errors.New("script not coinbase type"))
	}
	return s[5 : 5+16]
}

//Height 获取coinbase中的区块高度
func (s Script) Height() uint32 {
	if !s.IsCoinBase() {
		panic(errors.New("script not coinbase type"))
	}
	return Endian.Uint32(s[1:5])
}

//LockedScript 标准锁定脚本
type LockedScript struct {
	Type uint8
	Pkh  HASH160
	Exec VarBytes
}

//Address 获取地址
func (ss LockedScript) Address() Address {
	addr, err := EncodeAddress(ss.Pkh)
	if err != nil {
		panic(err)
	}
	return addr
}

//Encode 编码
func (ss LockedScript) Encode(w IWriter) error {
	if err := w.TWrite(ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Encode(w); err != nil {
		return err
	}
	if err := ss.Exec.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码
func (ss *LockedScript) Decode(r IReader) error {
	if err := r.TRead(&ss.Type); err != nil {
		return err
	}
	if err := ss.Pkh.Decode(r); err != nil {
		return err
	}
	if err := ss.Exec.Decode(r); err != nil {
		return err
	}
	return nil
}

//ToScript 转换为脚本存储
func (ss LockedScript) ToScript() (Script, error) {
	buf := NewWriter()
	err := ss.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//NewLockedScript 创建锁定脚本
func NewLockedScript(pkh HASH160, execs ...[]byte) (*LockedScript, error) {
	std := &LockedScript{Exec: VarBytes{}}
	std.Type = ScriptLockedType
	std.Pkh = pkh
	exec, err := MergeScript(execs...)
	if err != nil {
		return nil, err
	}
	std.Exec = exec
	return std, nil
}

//WitnessScript 隔离见证脚本
type WitnessScript struct {
	Type uint8      //SCRIPT_WITNESS_TYPE
	Num  uint8      //签名数量
	Less uint8      //至少正确的数量
	Arb  uint8      //是否启用仲裁
	Pks  []PKBytes  //公钥
	Sig  []SigBytes //签名
	Exec VarBytes   //执行脚本
}

//ToAccount 转换为账户信息
func (ss WitnessScript) ToAccount() (*Account, error) {
	return NewAccountWithPks(ss.Num, ss.Less, ss.Arb != InvalidArb, ss.Pks)
}

//IsEnableArb 是否启用仲裁
func (ss WitnessScript) IsEnableArb() bool {
	return ss.Arb != InvalidArb
}

//ForID id计算
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
	if err := ss.Exec.Encode(w); err != nil {
		return err
	}
	return nil
}

//Encode 编码
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
	if err := ss.Exec.Encode(w); err != nil {
		return err
	}
	return nil
}

//Decode 解码
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
	for i := range ss.Pks {
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
	for i := range ss.Sig {
		sig := SigBytes{}
		if err := sig.Decode(r); err != nil {
			return err
		}
		ss.Sig[i] = sig
	}
	if err := ss.Exec.Decode(r); err != nil {
		return err
	}
	return nil
}

//Hash 结算hash
func (ss WitnessScript) Hash() (HASH160, error) {
	return HashPks(ss.Num, ss.Less, ss.Arb, ss.Pks)
}

//Address 获取地址
func (ss WitnessScript) Address() Address {
	pkh, err := ss.Hash()
	if err != nil {
		panic(err)
	}
	addr, err := EncodeAddress(pkh)
	if err != nil {
		panic(err)
	}
	return addr
}

//HashPks hash公钥。地址hash也将由这个方法生成
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
	if err := w.TWrite(num); err != nil {
		return id, err
	}
	if err := w.TWrite(less); err != nil {
		return id, err
	}
	if err := w.TWrite(arb); err != nil {
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

//ToScript 转换为脚本存储
func (ss WitnessScript) ToScript() (Script, error) {
	buf := NewWriter()
	err := ss.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//CheckSigs 检查指定的签名
func (ss WitnessScript) CheckSigs(sigs []SigBytes) error {
	if ss.Type != ScriptWitnessType {
		return errors.New("type errpor")
	}
	if ss.Num == 0 || ss.Num > AccountKeyMaxSize {
		return errors.New("num error")
	}
	if ss.Less == 0 || ss.Less > AccountKeyMaxSize || ss.Less > ss.Num {
		return errors.New("less error")
	}
	//启用arb的情况下num必须》=3
	if ss.IsEnableArb() && ss.Num < 3 {
		return errors.New("arb num error")
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

//Check  检查签名证书数量
func (ss WitnessScript) Check() error {
	return ss.CheckSigs(ss.Sig)
}
