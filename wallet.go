package xginx

import (
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	//地址key前缀
	AddrPrefix = []byte{1}
	//加密的私钥
	//EncPKPrefix = []byte{2}
	//未加密
	StdPKPrefix = []byte{3}
	//管理员前缀
	AdminPrefix = []byte{4}
)

const (
	MinerAccountKey = "MinerAccount"
)

//钱包处理
type IWallet interface {
	//根据地址获取账号
	GetAccount(addr Address) (*Account, error)
	//关闭钱包
	Close()
	//新建账号
	NewAccount(num uint8, less uint8, arb bool) (Address, error)
	//导入账号 pw != ""添加密码
	ImportAccount(str string) error
	//获取所有账号
	ListAccount() ([]Address, error)
	//删除账号
	RemoveAccount(addr Address) error
	//设置矿工账号
	SetMiner(addr Address) error
	//获取矿工账号
	GetMiner() (*Account, error)
	//初始化管理密码
	SetAdminInfo(user string, pass string, flags uint32) error
	//获取管理员密码hash
	GetAdminInfo(user string) ([]byte, uint32, error)
}

type LevelDBWallet struct {
	dir  string
	dptr DBImp
}

//初始化管理员密码只能设置一次
func (db *LevelDBWallet) SetAdminInfo(user string, pass string, flags uint32) error {
	b4 := []byte{0, 0, 0, 0}
	Endian.PutUint32(b4, flags)
	hv := Hash256([]byte(pass))
	hv = append(hv, b4...)
	return db.dptr.Put(AdminPrefix, []byte(user), hv)
}

///获取管理员密码
func (db *LevelDBWallet) GetAdminInfo(user string) ([]byte, uint32, error) {
	b, err := db.dptr.Get(AdminPrefix, []byte(user))
	if err != nil {
		return nil, 0, err
	}
	hv := b[:32]
	fv := Endian.Uint32(b[32:])
	return hv, fv, nil
}

//列出地址
func (db *LevelDBWallet) ListAccount() ([]Address, error) {
	ds := []Address{}
	iter := db.dptr.Iterator(NewPrefix(AddrPrefix))
	defer iter.Close()
	for iter.Next() {
		key := iter.Key()
		ds = append(ds, Address(key[1:]))
	}
	return ds, nil
}

func (db *LevelDBWallet) RemoveAccount(addr Address) error {
	return db.dptr.Del(AddrPrefix, []byte(addr))
}

func (db *LevelDBWallet) NewAccount(num uint8, less uint8, arb bool) (Address, error) {
	acc, err := NewAccount(num, less, arb)
	if err != nil {
		return "", err
	}
	addr, err := acc.GetAddress()
	if err != nil {
		return "", err
	}
	dump, err := acc.Dump()
	if err != nil {
		return "", err
	}
	vbs := append([]byte{}, StdPKPrefix...)
	vbs = append(vbs, []byte(dump)...)
	err = db.dptr.Put(AddrPrefix, []byte(addr), vbs)
	if err != nil {
		return "", err
	}
	return addr, nil
}

func (db *LevelDBWallet) SetMiner(addr Address) error {
	_, err := db.GetAccount(addr)
	if err != nil {
		return err
	}
	return db.dptr.Put([]byte(MinerAccountKey), []byte(addr))
}

//获取矿工账号
func (db *LevelDBWallet) GetMiner() (*Account, error) {
	ab, err := db.dptr.Get([]byte(MinerAccountKey))
	if err != nil {
		return nil, err
	}
	return db.GetAccount(Address((ab)))
}

//导入私钥 pw != ""添加密码
func (db *LevelDBWallet) ImportAccount(str string) error {
	acc, err := LoadAccount(str)
	if err != nil {
		return err
	}
	addr, err := acc.GetAddress()
	if err != nil {
		return err
	}
	dump, err := acc.Dump()
	if err != nil {
		return err
	}
	vbs := append([]byte{}, StdPKPrefix...)
	vbs = append(vbs, []byte(dump)...)
	return db.dptr.Put(AddrPrefix, []byte(addr), vbs)
}

//根据钱包地址获取私钥
func (db *LevelDBWallet) GetAccount(addr Address) (*Account, error) {
	vbs, err := db.dptr.Get(AddrPrefix, []byte(addr))
	if err != nil {
		return nil, err
	}
	if vbs[0] != StdPKPrefix[0] {
		return nil, errors.New("addr encryption,need Decryption")
	}
	acc, err := LoadAccount(string(vbs[1:]))
	if err != nil {
		return nil, err
	}
	return acc, nil
}

//关闭钱包
func (db *LevelDBWallet) Close() {
	db.dptr.Close()
}

func NewLevelDBWallet(dir string) (IWallet, error) {
	ss := &LevelDBWallet{dir: dir}
	opts := &opt.Options{
		Filter: filter.NewBloomFilter(4),
	}
	sdb, err := leveldb.OpenFile(ss.dir, opts)
	if err != nil {
		return nil, err
	}
	ss.dptr = NewDB(sdb)
	return ss, nil
}
