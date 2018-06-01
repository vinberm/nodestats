package p2p

import ("sync"
		tcrypto "github.com/tendermint/go-crypto"
)
type AddrBook struct {
	key      string
	filePath string
	mtx sync.RWMutex
	ourAddrs   map[string]*NetAddress
}

func NewAddrBook(filePath string) *AddrBook {
	a := &AddrBook{
		key:      tcrypto.CRandHex(24),
		filePath: filePath,
		ourAddrs: make(map[string]*NetAddress),
	}
	return a
}


