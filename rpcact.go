package xginx

import "net/rpc"

type IRpcAccount interface {
	//创建一个新账号，返回账号地址
	CreateAccount(num uint8, less uint8, arb bool) (Address, error)
	//列出当前所有的账号地址
	ListAccount() ([]Address, error)
	//移除一个账号
	RemoveAccount(addr Address) error
}

type CAArgs struct {
	Num  uint8
	Less uint8
	Arb  bool
}

type CAResult struct {
	Addr Address
}

type RpcAccount struct {
}

type LAResult struct {
	Addrs []Address
}

type RCArgs struct {
	Addr Address
}

func (a *RpcAccount) Remove(args *RCArgs, null *RpcNIL) error {
	chain := GetChain()
	w := chain.GetListener().GetWallet()
	return w.RemoveAccount(args.Addr)
}

func (a *RpcAccount) List(null *RpcNIL, res *LAResult) error {
	chain := GetChain()
	w := chain.GetListener().GetWallet()
	*res = LAResult{Addrs: w.ListAccount()}
	return nil
}

func (a *RpcAccount) Create(arg *CAArgs, res *CAResult) error {
	chain := GetChain()
	w := chain.GetListener().GetWallet()
	addr, err := w.NewAccount(arg.Num, arg.Less, arg.Arb)
	if err != nil {
		return err
	}
	*res = CAResult{Addr: addr}
	return nil
}

type rpcaccountimp struct {
	rc *rpc.Client
}

//创建一个新账号，返回账号地址
func (imp *rpcaccountimp) CreateAccount(num uint8, less uint8, arb bool) (Address, error) {
	a := &CAArgs{
		Num:  num,
		Less: less,
		Arb:  arb,
	}
	r := &CAResult{}
	err := imp.rc.Call("RpcAccount.Create", a, r)
	return r.Addr, err
}

//列出当前所有的账号地址
func (imp *rpcaccountimp) ListAccount() ([]Address, error) {
	r := &LAResult{}
	err := imp.rc.Call("RpcAccount.List", RpcNil, r)
	return r.Addrs, err
}

//移除一个账号
func (imp *rpcaccountimp) RemoveAccount(addr Address) error {
	a := &RCArgs{Addr: addr}
	r := &RpcNIL{}
	return imp.rc.Call("RpcAccount.Remove", a, r)
}
