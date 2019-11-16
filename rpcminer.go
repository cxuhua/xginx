package xginx

import (
	"net/rpc"
)

type CBArgs struct {
	Ver uint32
}

type CBResult struct {
	Height uint32
}

type IRpcMiner interface {
	NewBlock(ver uint32) (*CBResult, error)
}

type RpcMiner struct {
}

func (m *RpcMiner) NewBlock(args *CBArgs, res *CBResult) error {
	chain := GetChain()
	Miner.NewBlock(args.Ver)
	last := chain.Last()
	if last != nil {
		*res = CBResult{Height: last.Height + 1}
	} else {
		*res = CBResult{}
	}
	return nil
}

type rpcminerimp struct {
	rc *rpc.Client
}

func (imp *rpcminerimp) NewBlock(ver uint32) (*CBResult, error) {
	a := &CBArgs{Ver: ver}
	r := &CBResult{}
	err := imp.rc.Call("RpcMiner.NewBlock", a, r)
	return r, err
}
