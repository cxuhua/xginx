package xginx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	//账号最大的私钥数量
	ACCOUNT_KEY_MAX_SIZE = 16
)

//导出json结构
type AccountJson struct {
	Num  uint8    `json:"num"`
	Less uint8    `json:"less"`
	Arb  uint8    `json:"arb"`
	Pubs []string `json:"pubs"`
	Pris []string `json:"pris"`
}

type PrivatesMap map[HASH160]*PrivateKey

//账号地址
type Account struct {
	Num  uint8                   //总的密钥数量
	Less uint8                   //至少需要签名的数量
	Arb  uint8                   //仲裁，当less  < num时可启用，必须是最后一个公钥
	Pubs []*PublicKey            //所有的密钥公钥
	Pris PrivatesMap //公钥对应的私钥
}

func LoadAccount(s string) (*Account, error) {
	a := &Account{}
	err := a.Load(s)
	return a, err
}

//是否包含私钥
func (ap Account) HasPrivate() bool {
	return len(ap.Pris) >= int(ap.Less)
}

//根据公钥索引获取私钥
func (ap Account) GetPrivateKey(pi int) *PrivateKey {
	if pi < 0 || pi >= len(ap.Pubs) {
		return nil
	}
	pkh := ap.Pubs[pi].Hash()
	return ap.Pris[pkh]
}

//是否启用仲裁
func (ap Account) IsEnableArb() bool {
	return ap.Arb != InvalidArb
}

//指定的私钥签名hash
//返回账户对应的索引和签名
func (ap Account) SignHash(hash []byte, pri *PrivateKey) (int, SigBytes, error) {
	pub := pri.PublicKey()
	sigb := SigBytes{}
	i := -1
	for idx, p := range ap.Pubs {
		if p.Equal(pub.Encode()) {
			i = idx
			break
		}
	}
	if i < 0 {
		return i, sigb, errors.New("private not belong to account")
	}
	sig, err := pri.Sign(hash)
	if err != nil {
		return i, sigb, err
	}
	sigb.Set(sig)
	return i, sigb, nil
}

//pi public index
//hv sign hash
func (ap Account) Sign(pi int, hv []byte) (SigBytes, error) {
	sigb := SigBytes{}
	pri := ap.GetPrivateKey(pi)
	if pri == nil {
		return sigb, errors.New("private key miss")
	}
	sig, err := pri.Sign(hv)
	if err != nil {
		return sigb, err
	}
	sigb.Set(sig)
	return sigb, nil
}

//生成未带有签名的脚本对象
func (ap Account) NewWitnessScript() *WitnessScript {
	w := &WitnessScript{}
	w.Type = SCRIPT_WITNESS_TYPE
	w.Num = ap.Num
	w.Less = ap.Less
	w.Arb = ap.Arb
	w.Pks = []PKBytes{}
	for _, pub := range ap.Pubs {
		w.Pks = append(w.Pks, pub.GetPks())
	}
	w.Sig = []SigBytes{}
	return w
}

//生成锁定脚本
func (ap Account) NewLockedScript(vbs ...[]byte) (Script, error) {
	if pkh, err := ap.GetPkh(); err != nil {
		return nil, err
	} else {
		return NewLockedScript(pkh, vbs...)
	}
}

func (ap Account) String() string {
	if ap.IsEnableArb() {
		return fmt.Sprintf("%d-%d+arb", ap.Less, ap.Num)
	} else {
		return fmt.Sprintf("%d-%d", ap.Less, ap.Num)
	}
}

func (ap *Account) hasPub(pub *PublicKey) bool {
	for _, v := range ap.Pubs {
		if v.Equal(pub.Encode()) {
			return true
		}
	}
	return false
}

//加载账号信息
func (ap *Account) Load(s string) error {
	data, err := B58Decode(s, BitcoinAlphabet)
	if err != nil {
		return err
	}
	dl := len(data)
	hash := Hash256(data[:dl-4])
	if !bytes.Equal(hash[:4], data[dl-4:]) {
		return errors.New("checksum error")
	}
	ap.Pubs = []*PublicKey{}
	ap.Pris = PrivatesMap{}
	aj := &AccountJson{}
	err = json.Unmarshal(data[:dl-4], aj)
	if err != nil {
		return err
	}
	ap.Num = aj.Num
	ap.Less = aj.Less
	ap.Arb = aj.Arb
	for _, ss := range aj.Pubs {
		pp, err := LoadPublicKey(ss)
		if err != nil {
			return err
		}
		ap.Pubs = append(ap.Pubs, pp)
	}
	for _, ss := range aj.Pris {
		pri, err := LoadPrivateKey(ss)
		if err != nil {
			return err
		}
		pub := pri.PublicKey()
		if !ap.hasPub(pub) {
			return errors.New("pri'pubkey not in account")
		}
		ap.Pris[pri.PublicKey().Hash()] = pri
	}
	return ap.Check()
}

//导出账号信息
func (ap Account) Dump(ispri bool) (string, error) {
	aj := AccountJson{
		Num:  ap.Num,
		Less: ap.Less,
		Arb:  ap.Arb,
		Pubs: []string{},
		Pris: []string{},
	}
	for _, pub := range ap.Pubs {
		aj.Pubs = append(aj.Pubs, pub.Dump())
	}
	if ispri && ap.HasPrivate() {
		for _, pri := range ap.Pris {
			aj.Pris = append(aj.Pris, pri.Dump())
		}
	}
	data, err := json.Marshal(aj)
	if err != nil {
		return "", err
	}
	hash := Hash256(data)
	data = append(data, hash[:4]...)
	str := B58Encode(data, BitcoinAlphabet)
	return str, nil
}

func (ap Account) GetPks() []PKBytes {
	pks := []PKBytes{}
	for _, pub := range ap.Pubs {
		pks = append(pks, pub.GetPks())
	}
	return pks
}

//获取公钥hash数组
func (ap Account) GetPkhs() []HASH160 {
	pkhs := []HASH160{}
	for _,v := range ap.GetPks() {
		pkhs = append(pkhs,v.Hash())
	}
	return pkhs
}

//获取账号地址
func (ap Account) GetPkh() (HASH160, error) {
	if err := ap.Check(); err != nil {
		return ZERO160, err
	}
	return HashPks(ap.Num, ap.Less, ap.Arb, ap.GetPks())
}

//获取账号地址
func (ap Account) GetAddress() (Address, error) {
	if pkh, err := ap.GetPkh(); err != nil {
		return "", err
	} else if addr, err := EncodeAddress(pkh); err != nil {
		return "", err
	} else {
		return addr, nil
	}
}

//创建无私钥账号
//不能用来签名
func NewAccountWithPks(num uint8, less uint8, arb bool, pkss []PKBytes) (*Account, error) {
	if len(pkss) != int(num) {
		return nil, errors.New("pubs num error")
	}
	ap := &Account{
		Num:  num,
		Less: less,
		Arb:  InvalidArb,
	}
	if arb && num == less {
		return nil, errors.New("can't use arb")
	}
	ap.Pubs = []*PublicKey{}
	ap.Pris = PrivatesMap{}
	for _, pks := range pkss {
		pub, err := NewPublicKey(pks.Bytes())
		if err != nil {
			return nil, err
		}
		ap.Pubs = append(ap.Pubs, pub)
	}
	//如果启用arb，最后后一个为仲裁公钥
	if num > 0 && less > 0 && arb && less < num {
		ap.Arb = ap.Num - 1
	}
	if err := ap.Check(); err != nil {
		return nil, err
	}
	return ap, nil
}

//创建num个证书的账号,至少需要less个签名
//arb是否启用仲裁
//有pkss将不包含私钥
func NewAccount(num uint8, less uint8, arb bool, pkss ...PKBytes) (*Account, error) {
	ap := &Account{
		Num:  num,
		Less: less,
		Arb:  InvalidArb,
	}
	if arb && num == less {
		return nil, errors.New("can't use arb")
	}
	ap.Pubs = []*PublicKey{}
	ap.Pris = PrivatesMap{}
	if len(pkss) > 0 {
		if len(pkss) != int(num) {
			return nil,errors.New("pkss count error")
		}
		for _, pks := range pkss {
			pub, err := NewPublicKey(pks.Bytes())
			if err != nil {
				return nil, err
			}
			ap.Pubs = append(ap.Pubs, pub)
		}
	}else {
		//自动创建公钥私钥
		for i := 0; i < int(num); i++ {
			pri, err := NewPrivateKey()
			if err != nil {
				return nil, err
			}
			pub := pri.PublicKey()
			ap.Pris[pub.Hash()] = pri
			ap.Pubs = append(ap.Pubs, pub)
		}
	}
	//如果启用arb，最后后一个为仲裁公钥
	if num > 0 && less > 0 && arb && less < num {
		ap.Arb = ap.Num - 1
	}
	if err := ap.Check(); err != nil {
		return nil, err
	}
	return ap, nil
}

//csp=true 检查签名证书数量
func (ap Account) Check() error {
	if ap.Num == 0 ||
		ap.Num > ACCOUNT_KEY_MAX_SIZE ||
		ap.Less == 0 ||
		ap.Less > ACCOUNT_KEY_MAX_SIZE ||
		ap.Less > ap.Num {
		return errors.New("num less error")
	}
	if ap.Less == ap.Num && ap.Arb != InvalidArb {
		return errors.New("arb error")
	}
	if len(ap.Pubs) != int(ap.Num) {
		return errors.New("pubs num error")
	}
	return nil
}
