package xginx

import (
	"encoding/json"
	"errors"
	"fmt"
)

// 账号最大的私钥数量
const (
	AccountKeyMaxSize = 16
)

//AccountJSON 账户导出结构
type AccountJSON struct {
	Num  uint8    `json:"num"`
	Less uint8    `json:"less"`
	Arb  uint8    `json:"arb"`
	Pubs []string `json:"pubs"`
	Pris []string `json:"pris"`
}

//PrivatesMap 私钥存储结构
type PrivatesMap map[HASH160]*PrivateKey

//Account 账号地址
type Account struct {
	Num  uint8        //总的密钥数量
	Less uint8        //至少需要签名的数量
	Arb  uint8        //仲裁，当less  < num时可启用，必须是最后一个公钥
	Pubs []*PublicKey //所有的密钥公钥
	Pris PrivatesMap  //公钥对应的私钥
}

//LoadAccount 从导出的数据加载账号
func LoadAccount(s string, pass ...string) (*Account, error) {
	a := &Account{}
	err := a.Load(s, pass...)
	return a, err
}

//HasPrivate 是否包含私钥
func (ap Account) HasPrivate() bool {
	return len(ap.Pris) >= int(ap.Less)
}

//GetPrivateKey 根据公钥索引获取私钥
func (ap Account) GetPrivateKey(pi int) *PrivateKey {
	if pi < 0 || pi >= len(ap.Pubs) {
		return nil
	}
	pkh := ap.Pubs[pi].Hash()
	return ap.Pris[pkh]
}

//IsEnableArb 是否启用仲裁
func (ap Account) IsEnableArb() bool {
	return ap.Arb != InvalidArb
}

//SignHash 指定的私钥签名hash
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

//VerifyAll 验证签名
func (ap Account) VerifyAll(hv []byte, sigs [][]byte) error {
	less := int(ap.Less)
	num := int(ap.Num)
	if len(ap.Pubs) != num {
		return errors.New("pub num error")
	}
	if num < less {
		return errors.New("pub num error,num must >= less")
	}
	for i, k := 0, 0; i < len(sigs) && k < len(ap.Pubs); {
		sig, err := NewSigValue(sigs[i])
		if err != nil {
			return err
		}
		vok := ap.Pubs[i].Verify(hv, sig)
		if vok {
			less--
			i++
		}
		//如果启用仲裁，并且当前仲裁验证成功立即返回
		if vok && ap.IsEnableArb() && ap.Arb == uint8(k) {
			less = 0
		}
		if less == 0 {
			break
		}
		k++
	}
	if less > 0 {
		return errors.New("sig verify error")
	}
	return nil
}

//SignAll 获取账号所有签名
func (ap Account) SignAll(hv []byte) ([][]byte, error) {
	rets := [][]byte{}
	for idx := range ap.Pubs {
		sig, err := ap.Sign(idx, hv)
		if err != nil {
			continue
		}
		rets = append(rets, sig.Bytes())
	}
	if len(rets) == 0 {
		return nil, errors.New("miss sigs")
	}
	return rets, nil
}

//Sign 签名指定公钥
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

//NewWitnessScript 生成未带有签名的脚本对象
func (ap Account) NewWitnessScript() *WitnessScript {
	w := &WitnessScript{}
	w.Type = ScriptWitnessType
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

//NewLockedScript 生成锁定脚本
func (ap Account) NewLockedScript(vbs ...[]byte) (Script, error) {
	pkh, err := ap.GetPkh()
	if err != nil {
		return nil, err
	}
	return NewLockedScript(pkh, vbs...)
}

//
func (ap Account) String() string {
	if ap.IsEnableArb() {
		return fmt.Sprintf("%d-%d+arb", ap.Less, ap.Num)
	}
	return fmt.Sprintf("%d-%d", ap.Less, ap.Num)
}

func (ap *Account) hasPub(pub *PublicKey) bool {
	for _, v := range ap.Pubs {
		if v.Equal(pub.Encode()) {
			return true
		}
	}
	return false
}

//Load 加载账号信息
func (ap *Account) Load(s string, pass ...string) error {
	data, err := HashLoad(s, pass...)
	if err != nil {
		return err
	}
	ap.Pubs = []*PublicKey{}
	ap.Pris = PrivatesMap{}
	aj := &AccountJSON{}
	err = json.Unmarshal(data, aj)
	if err != nil {
		return err
	}
	ap.Num = aj.Num
	ap.Less = aj.Less
	ap.Arb = aj.Arb
	for _, ss := range aj.Pubs {
		pp, err := LoadPublicKey(ss, pass...)
		if err != nil {
			return err
		}
		ap.Pubs = append(ap.Pubs, pp)
	}
	for _, ss := range aj.Pris {
		pri, err := LoadPrivateKey(ss, pass...)
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

//Dump 导出账号信息
func (ap Account) Dump(ispri bool, pass ...string) (string, error) {
	aj := AccountJSON{
		Num:  ap.Num,
		Less: ap.Less,
		Arb:  ap.Arb,
		Pubs: []string{},
		Pris: []string{},
	}
	for _, pub := range ap.Pubs {
		ds, err := pub.Dump(pass...)
		if err != nil {
			return "", err
		}
		aj.Pubs = append(aj.Pubs, ds)
	}
	if ispri && ap.HasPrivate() {
		for _, pub := range ap.Pubs {
			pkh := pub.GetPks().Hash()
			pri := ap.Pris[pkh]
			ds, err := pri.Dump(pass...)
			if err != nil {
				return "", err
			}
			aj.Pris = append(aj.Pris, ds)
		}
	}
	data, err := json.Marshal(aj)
	if err != nil {
		return "", err
	}
	return HashDump(data, pass...)
}

//GetPks 获取所有的公钥
func (ap Account) GetPks() []PKBytes {
	pks := []PKBytes{}
	for _, pub := range ap.Pubs {
		pks = append(pks, pub.GetPks())
	}
	return pks
}

//GetPkhs 获取公钥hash数组
func (ap Account) GetPkhs() []HASH160 {
	pkhs := []HASH160{}
	for _, v := range ap.GetPks() {
		pkhs = append(pkhs, v.Hash())
	}
	return pkhs
}

//GetPkh 获取账号地址
func (ap Account) GetPkh() (HASH160, error) {
	if err := ap.Check(); err != nil {
		return ZERO160, err
	}
	return HashPks(ap.Num, ap.Less, ap.Arb, ap.GetPks())
}

//GetAddress 获取账号地址
func (ap Account) GetAddress() (Address, error) {
	if pkh, err := ap.GetPkh(); err != nil {
		return "", err
	} else if addr, err := EncodeAddress(pkh); err != nil {
		return "", err
	} else {
		return addr, nil
	}
}

//NewAccountWithPks 创建无私钥账号
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

//NewAccount 创建num个证书的账号,至少需要less个签名
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
			return nil, errors.New("pkss count error")
		}
		for _, pks := range pkss {
			pub, err := NewPublicKey(pks.Bytes())
			if err != nil {
				return nil, err
			}
			ap.Pubs = append(ap.Pubs, pub)
		}
	} else {
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

//Check 检查签名证书数量
func (ap Account) Check() error {
	if ap.Num == 0 ||
		ap.Num > AccountKeyMaxSize ||
		ap.Less == 0 ||
		ap.Less > AccountKeyMaxSize ||
		ap.Less > ap.Num {
		return errors.New("num less error")
	}
	if ap.Arb != InvalidArb && ap.Num < 3 {
		return errors.New("use arb,num must >= 3")
	}
	if ap.Less == ap.Num && ap.Arb != InvalidArb {
		return errors.New("arb error")
	}
	if len(ap.Pubs) != int(ap.Num) {
		return errors.New("pubs num error")
	}
	return nil
}
