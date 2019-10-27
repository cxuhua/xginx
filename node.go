package xginx

type Node struct {
	pool *CertPool
}

func NewNode() *Node {
	n := &Node{}
	n.pool = conf.NewCertPool()
	return n
}
