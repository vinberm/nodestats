package node

import (
	cfg "github.com/nodestats/config"
	"github.com/nodestats/p2p"
)

type Node struct {
	Config *cfg.Config
	sw *p2p.Switch
}

func NewNode(config *cfg.Config) *Node {
	addrBook := p2p.NewAddrBook(cfg.DefaultConfig().AddrBook)
	sw := p2p.NewSwitch(config, addrBook)
	return &Node{
		Config: config,
		sw:     sw,
	}
}

func (n *Node) Start() error {
	n.sw.Start()
	return nil
}

