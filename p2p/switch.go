package p2p

import (
	"sync"

	"github.com/tendermint/go-crypto"
	//dbm "github.com/tendermint/tmlibs/db"
	cfg "github.com/nodestats/config"

)

type Switch struct {
	config       *cfg.Config
	addrBook     *AddrBook
	nodePrivKey  crypto.PrivKeyEd25519
	nodeInfo     *NodeInfo
	mtx          sync.Mutex
	reactors     map[string]Reactor
}

func NewSwitch(config *cfg.Config, addrBook *AddrBook) *Switch {
	sw := &Switch{
		config: config,
		addrBook: addrBook,
		nodeInfo: nil,
	}
	return sw
}

func (sw *Switch) Start() error {
	// Start reactors
	for _, reactor := range sw.reactors {
		_, err := reactor.Start()
		if err != nil {
			return err
		}
	}
	// Start listeners
	//for _, listener := range sw.listeners {
	//	go sw.listenerRoutine(listener)
	//}
	return nil
}
