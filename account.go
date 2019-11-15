package xginx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	//账号最大的私钥数量
	ACCOUNT_KEY_MAX_SIZA = 16
)

//导出json结构
type AccountJson struct {
	Num  int      `json:"num"`
	Less int      `json:"less"`
	Pubs []string `json:"pubs"`
	Pris []string `json:"pris"`
}

//账号地址
type Account struct {
	num  uint8
	less uint8
	pubs []*PublicKey
	pris []*PrivateKey //私钥可能在多个地方，现在测试都统一先放这里
}

func LoadAccount(s string) (*Account, error) {
	a := &Account{}
	err := a.Load(s)
	return a, err
}

//pi public index
//hv sign hash
func (ap Account) Sign(pi int, hv []byte) (SigBytes, error) {
	sigb := SigBytes{}
	if pi == 1 {
		return sigb, errors.New("skip 1")
	}
	pri := ap.pris[pi]
	sig, err := pri.Sign(hv)
	if err != nil {
		return sigb, err
	}
	sigb.Set(sig)
	return sigb, nil
}

func (ap Account) NewWitnessScript() *WitnessScript {
	w := &WitnessScript{}
	w.Type = SCRIPT_WITNESS_TYPE
	w.Num = ap.num
	w.Less = ap.less
	w.Pks = []PKBytes{}
	for _, pub := range ap.pubs {
		w.Pks = append(w.Pks, pub.GetPks())
	}
	w.Sig = []SigBytes{}
	return w
}

func (ap Account) NewLockedScript() Script {
	return NewLockedScript(ap.GetPkh())
}

func (ap Account) String() string {
	return fmt.Sprintf("%d-%d", ap.less, ap.num)
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
	ap.pubs = []*PublicKey{}
	ap.pris = []*PrivateKey{}
	aj := &AccountJson{}
	err = json.Unmarshal(data[:dl-4], aj)
	if err != nil {
		return err
	}
	ap.num = uint8(aj.Num)
	ap.less = uint8(aj.Less)
	for _, ss := range aj.Pubs {
		pp, err := LoadPublicKey(ss)
		if err != nil {
			return err
		}
		ap.pubs = append(ap.pubs, pp)
	}
	for _, ss := range aj.Pris {
		pr, err := LoadPrivateKey(ss)
		if err != nil {
			return err
		}
		ap.pris = append(ap.pris, pr)
	}
	return ap.Check()
}

//导出账号信息
func (ap Account) Dump() (string, error) {
	aj := AccountJson{
		Num:  int(ap.num),
		Less: int(ap.less),
		Pubs: []string{},
		Pris: []string{},
	}
	for _, pub := range ap.pubs {
		aj.Pubs = append(aj.Pubs, pub.Dump())
	}
	for _, pri := range ap.pris {
		aj.Pris = append(aj.Pris, pri.Dump())
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

//获取账号地址
func (ap Account) GetPkh() HASH160 {
	if err := ap.Check(); err != nil {
		panic(err)
	}
	pks := []PKBytes{}
	for _, pub := range ap.pubs {
		pks = append(pks, pub.GetPks())
	}
	return HashPks(ap.num, ap.less, pks)
}

//获取账号地址
func (ap Account) GetAddress() string {
	addr, err := EncodeAddress(ap.GetPkh())
	if err != nil {
		panic(err)
	}
	return addr
}

//创建num个证书的账号,至少需要less个签名
func NewAccount(num uint8, less uint8) (*Account, error) {
	ap := &Account{num: num, less: less}
	ap.pubs = []*PublicKey{}
	ap.pris = []*PrivateKey{}
	for i := 0; i < int(num); i++ {
		pri, err := NewPrivateKey()
		if err != nil {
			return nil, err
		}
		ap.pris = append(ap.pris, pri)
		ap.pubs = append(ap.pubs, pri.PublicKey())
	}
	if err := ap.Check(); err != nil {
		return nil, err
	}
	return ap, nil
}

//csp=true 检查签名证书数量
func (ap Account) Check() error {
	if ap.num == 0 ||
		ap.num > ACCOUNT_KEY_MAX_SIZA ||
		ap.less == 0 ||
		ap.less > ACCOUNT_KEY_MAX_SIZA ||
		ap.less > ap.num {
		return errors.New("num less error")
	}
	if len(ap.pubs) != int(ap.num) {
		return errors.New("pubs num error")
	}
	return nil
}
