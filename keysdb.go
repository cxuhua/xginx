package xginx

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	//金额账户
	CoinAccountType = 1
	//临时账户
	TempAccountType = 2
)

//密钥证书存储库
//AccountInfo 地址账户,值使用公钥hash256生成地址
//无需加密,只是用来保存地址生成数据,是否有控制权需要另外检测
type AccountInfo struct {
	Num  int      `json:"num"`  //密钥总数
	Less int      `json:"less"` //需要通过的签名数量
	Arb  bool     `json:"arb"`  //是否启用仲裁 != InvalidArb 表示启用
	Pks  []string `json:"pks"`  //公钥ID
	Desc string   `json:"desc"` //描述
	Type int      `json:"type"` //类型 1-金额账户,2-临时账户,可能会被删除
}

func (ka AccountInfo) GetArb() uint8 {
	if ka.Arb {
		return uint8(ka.Num - 1)
	}
	return InvalidArb
}

func (ka AccountInfo) ID() (Address, error) {
	pkcs := []HASH256{}
	for _, addr := range ka.Pks {
		pkh, err := DecodePublicHash(addr)
		if err != nil {
			return "", err
		}
		pkcs = append(pkcs, pkh)
	}
	arb := InvalidArb
	if ka.Arb {
		arb = uint8(len(ka.Pks) - 1)
	}
	hv, err := HashPkh(uint8(ka.Num), uint8(ka.Less), arb, pkcs)
	if err != nil {
		return "", err
	}
	return EncodeAddress(hv)
}

//转换为账户,必须拥有私钥控制权
func (ka AccountInfo) ToAccount(db IKeysDB) (*Account, error) {
	kaddr, err := ka.ID()
	if err != nil {
		return nil, err
	}
	acc := &Account{
		Num:  uint8(ka.Num),
		Less: uint8(ka.Less),
		Arb:  ka.GetArb(),
		Pubs: PublicArray{},
		Pris: PrivatesMap{},
	}
	for _, pid := range ka.Pks {
		pri, err := db.LoadPrivateKey(pid)
		if err != nil {
			return nil, err
		}
		pub := pri.PublicKey()
		acc.Pubs = append(acc.Pubs, pub)
		acc.Pris[pub.Hash()] = pri
	}
	aaddr, err := acc.GetAddress()
	if err != nil {
		return nil, err
	}
	if kaddr != aaddr {
		return nil, fmt.Errorf("id not match")
	}
	return acc, nil
}

func (ka AccountInfo) Encode() ([]byte, error) {
	return json.Marshal(ka)
}

func (ka AccountInfo) Check() error {
	//检测私钥id格式
	for idx, kid := range ka.Pks {
		_, err := DecodePublicHash(kid)
		if err != nil {
			return fmt.Errorf("private id %d error %w", idx, err)
		}
	}
	return CheckAccountArgs(uint8(ka.Num), uint8(ka.Less), ka.Arb, len(ka.Pks))
}

func (ka *AccountInfo) Decode(bb []byte) error {
	return json.Unmarshal(bb, ka)
}

//CtrlPrivateKeyReq 私钥控制权证明请求参数
type CtrlPrivateKeyReq struct {
	ID      string //私钥ID
	RandStr string //给定一个随机字符串
}

//CtrlPrivateKeyRes 私钥控制权结果
type CtrlPrivateKeyRes struct {
	RandStr string   //给定的随机字符串
	Pks     PKBytes  //返回私钥对应的公钥
	Sig     SigBytes //随机字符串签名
}

//检测是否签名正确
func (res CtrlPrivateKeyRes) Check(req *CtrlPrivateKeyReq) error {
	//是否是同一个字符串
	if res.RandStr != req.RandStr {
		return fmt.Errorf("rand str error")
	}
	//公钥是否正确
	pub, err := NewPublicKey(res.Pks[:])
	if err != nil {
		return err
	}
	//id是否一致
	pid, err := pub.ID()
	if err != nil {
		return err
	}
	if pid != req.ID {
		return fmt.Errorf("id error")
	}
	sig, err := NewSigValue(res.Sig[:])
	if err != nil {
		return err
	}
	hv := Hash256From([]byte(res.RandStr))
	//验证签名
	if !pub.Verify(hv[:], sig) {
		return fmt.Errorf("sig verify error")
	}
	return nil
}

type IKeysDB interface {
	Sync()
	//关闭密钥数据库
	Close()
	//创建一个1-1账号返回描述信息
	NewAccountInfo(typ int, desc string) (*AccountInfo, error)
	//创建一个新的私钥
	NewPrivateKey() (string, error)
	//获取一个私钥
	LoadPrivateKey(id string) (*PrivateKey, error)
	//保存账户地址描述
	SaveAccountInfo(ka *AccountInfo) (Address, error)
	//加载账户地址描述
	LoadAccountInfo(id Address) (*AccountInfo, error)
	//是否有私钥控制权
	HasKeyPrivileges(req *CtrlPrivateKeyReq) (*CtrlPrivateKeyRes, error)
	//创建待签名脚本
	NewWitnessScript(id Address, execs ...[]byte) (*WitnessScript, error)
	//创建锁定脚本
	NewLockedScript(id Address, meta string, exec ...[]byte) (*LockedScript, error)
	//签名并填充脚本数据
	Sign(id Address, data []byte, wits *WitnessScript) error
	//删除私钥
	DeletePrivateKey(id string) error
	//删除账户描述
	DeleteAccountInfo(id Address) error
	//设置密钥ttl为过期时间
	SetKey(key string, ttl time.Duration)
	//列出地址
	ListAddress(limit int, skey ...[]byte) ([]Address, []byte)
	//列出私钥id
	ListPrivate(limit int, skey ...[]byte) ([]string, []byte)
	//添加配置
	PutConfig(id string, v []byte) error
	//获取配置
	GetConfig(id string) ([]byte, error)
	//是否存在配置
	HasConfig(id string) (bool, error)
}

var (
	priprefix = []byte{1} //私钥前缀,使用公钥hash256作为id保存
	accprefix = []byte{2} //账号前缀
	conprefox = []byte{3} //配置信息key前缀
)

type levelkeysdb struct {
	key []string
	db  DBImp
	exp time.Time //密钥过期时间
	ttl time.Duration
}

func (kd levelkeysdb) keyexpire() bool {
	if kd.ttl == 0 {
		return false
	}
	if time.Now().Sub(kd.exp) >= 0 {
		return true
	}
	return false
}

//添加配置
func (kd *levelkeysdb) PutConfig(id string, v []byte) error {
	return kd.db.Put(conprefox, []byte(id), v)
}

//是否存在配置
func (kd *levelkeysdb) HasConfig(id string) (bool, error) {
	return kd.db.Has(conprefox, []byte(id))
}

//获取配置
func (kd *levelkeysdb) GetConfig(id string) ([]byte, error) {
	return kd.db.Get(conprefox, []byte(id))
}

func (kd *levelkeysdb) NewAccountInfo(typ int, desc string) (*AccountInfo, error) {
	id, err := kd.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	ka := &AccountInfo{
		Num:  1,
		Less: 1,
		Arb:  false,
		Pks:  []string{id},
		Type: CoinAccountType,
		Desc: desc,
	}
	_, err = kd.SaveAccountInfo(ka)
	return ka, err
}

func (kd *levelkeysdb) SetKey(key string, ttl time.Duration) {
	if ttl > 0 {
		kd.exp = time.Now().Add(ttl)
		kd.ttl = ttl
	} else {
		kd.ttl = 0
	}
	kd.key = []string{key}
}

func (kd *levelkeysdb) Close() {
	kd.db.Close()
}

//列出地址 返回地址列表和最后一个key,下一页从返回的key开始
func (kd *levelkeysdb) ListAddress(limit int, skey ...[]byte) ([]Address, []byte) {
	var lkey []byte
	res := []Address{}
	iter := kd.db.Iterator(NewPrefix(accprefix))
	defer iter.Close()
	//如果设置了起始key跳过起始key,并且移动到下一个
	if len(skey) > 0 && skey[0] != nil && !iter.Seek(skey[0]) {
		return res, lkey
	}
	for iter.Next() {
		lkey = iter.Key()
		res = append(res, Address(lkey[len(accprefix):]))
		if limit > 0 && len(res) >= limit {
			break
		}
	}
	return res, lkey
}

//列出私钥id
func (kd *levelkeysdb) ListPrivate(limit int, skey ...[]byte) ([]string, []byte) {
	var lkey []byte
	res := []string{}
	iter := kd.db.Iterator(NewPrefix(priprefix))
	defer iter.Close()
	//如果设置了起始key跳过起始key,并且移动到下一个
	if len(skey) > 0 && skey[0] != nil && !iter.Seek(skey[0]) {
		return res, lkey
	}
	for iter.Next() {
		lkey = iter.Key()
		res = append(res, string(lkey[len(accprefix):]))
		if limit > 0 && len(res) >= limit {
			break
		}
	}
	return res, lkey
}

func (kd *levelkeysdb) HasKeyPrivileges(req *CtrlPrivateKeyReq) (*CtrlPrivateKeyRes, error) {
	pri, err := kd.LoadPrivateKey(req.ID)
	if err != nil {
		return nil, err
	}
	hv := Hash256From([]byte(req.RandStr))
	pub := pri.PublicKey()
	res := &CtrlPrivateKeyRes{}
	res.RandStr = req.RandStr
	res.Pks = pub.GetPks()
	sig, err := pri.Sign(hv[:])
	if err != nil {
		return nil, err
	}
	res.Sig = sig.GetSigs()
	return res, nil
}

func (kd *levelkeysdb) DeletePrivateKey(id string) error {
	return kd.db.Del(priprefix, []byte(id))
}

func (kd *levelkeysdb) DeleteAccountInfo(id Address) error {
	return kd.db.Del(accprefix, []byte(id))
}

//NewLockedScript 生成锁定脚本
func (kd *levelkeysdb) NewLockedScript(id Address, meta string, exec ...[]byte) (*LockedScript, error) {
	_, err := kd.LoadAccountInfo(id)
	if err != nil {
		return nil, err
	}
	pkh, err := id.GetPkh()
	if err != nil {
		return nil, err
	}
	return NewLockedScript(pkh, meta, exec...)
}

//使用我的数据签名data并填充脚本脚本
func (kd *levelkeysdb) Sign(id Address, data []byte, wits *WitnessScript) error {
	ka, err := kd.LoadAccountInfo(id)
	if err != nil {
		return err
	}
	//检测数量是否匹配
	if len(ka.Pks) != len(wits.Pks) {
		return fmt.Errorf("pks num error")
	}
	if len(ka.Pks) != len(wits.Sig) {
		return fmt.Errorf("Sig num error")
	}
	//注意签名的顺序应该和公钥保存顺序一致
	for i, id := range ka.Pks {
		pri, err := kd.LoadPrivateKey(id)
		if err != nil {
			//忽略我没控制权的私钥
			continue
		}
		pub := pri.PublicKey()
		sig, err := pri.Sign(data)
		if err != nil {
			//签名错误返回
			return err
		}
		//先对号入座,在调用WitnessScript.Final时过滤不需要的签名,并检查公钥是否完整
		wits.Pks[i] = pub.GetPks()
		if !wits.Pks[i].IsValid() {
			return fmt.Errorf("public bytes error")
		}
		wits.Sig[i] = sig.GetSigs()
		if !wits.Sig[i].IsValid() {
			return fmt.Errorf("signure bytes error")
		}
	}
	return nil
}

//NewWitnessScript 生成未带有签名的脚本对象
func (kd *levelkeysdb) NewWitnessScript(id Address, execs ...[]byte) (*WitnessScript, error) {
	ka, err := kd.LoadAccountInfo(id)
	if err != nil {
		return nil, err
	}
	w := &WitnessScript{}
	w.Type = ScriptWitnessType
	w.Num = uint8(ka.Num)
	w.Less = uint8(ka.Less)
	w.Arb = ka.GetArb()
	//使用空的数据预填充
	w.Pks = make([]PKBytes, w.Num)
	w.Sig = make([]SigBytes, w.Num)
	exec, err := MergeScript(execs...)
	if err != nil {
		return nil, err
	}
	w.Exec = exec
	return w, nil
}

//获取账户信息
func (kd *levelkeysdb) LoadAccountInfo(id Address) (*AccountInfo, error) {
	ka := &AccountInfo{}
	bb, err := kd.db.Get(accprefix, []byte(id))
	if err != nil {
		return nil, err
	}
	err = ka.Decode(bb)
	if err != nil {
		return nil, err
	}
	err = ka.Check()
	if err != nil {
		return nil, err
	}
	return ka, nil
}

//使用私钥id创建一个账号,私钥id可能是自己的,也可能是其他人的,注意控制权
//pkids 账户包含这些公钥ID,对应的私钥和公钥内容使用LoadPrivateKey获取
func (kd *levelkeysdb) SaveAccountInfo(ka *AccountInfo) (Address, error) {
	if err := ka.Check(); err != nil {
		return "", err
	}
	id, err := ka.ID()
	if err != nil {
		return "", err
	}
	if _, err := kd.LoadAccountInfo(id); err == nil {
		return "", fmt.Errorf("address %s exists", id)
	}
	bb, err := ka.Encode()
	if err != nil {
		return "", err
	}
	err = kd.db.Put(accprefix, []byte(id), bb)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (kd *levelkeysdb) LoadPrivateKey(id string) (*PrivateKey, error) {
	if kd.keyexpire() {
		return nil, fmt.Errorf("key expire")
	}
	bb, err := kd.db.Get(priprefix, []byte(id))
	if err != nil {
		return nil, err
	}
	return LoadPrivateKey(string(bb), kd.key...)
}

func (kd *levelkeysdb) Sync() {
	kd.db.Sync()
}

//创建私钥,返回公钥hash256
func (kd *levelkeysdb) NewPrivateKey() (string, error) {
	if kd.keyexpire() {
		return "", fmt.Errorf("key expire")
	}
	pri, err := NewPrivateKey()
	if err != nil {
		return "", err
	}
	id, err := pri.PublicKey().ID()
	if err != nil {
		return "", err
	}
	str, err := pri.Dump(kd.key...)
	if err != nil {
		return "", err
	}
	err = kd.db.Put(priprefix, []byte(id), []byte(str))
	if err != nil {
		return "", err
	}
	return id, nil
}

func OpenKeysDB(dir string, key ...string) (IKeysDB, error) {
	db, err := NewDBImp(dir)
	if err != nil {
		return nil, err
	}
	ptr := &levelkeysdb{
		db:  db,
		key: []string{},
	}
	if len(key) > 0 && key[0] != "" {
		ptr.key = []string{key[0]}
	}
	return ptr, nil
}
