package p2p

import (
	"encoding/json"
	cmn "github.com/tendermint/tmlibs/common"
)

type addrBookJSON struct {
	Key   string
	Addrs []*knownAddress
}

// SaveToFile will save the address book to a json file in disk
func (a *AddrBook) SaveToFile() error {
	a.mtx.RLock()
	defer a.mtx.RUnlock()

	aJSON := &addrBookJSON{Key: a.key, Addrs: []*knownAddress{}}
	for _, ka := range a.addrLookup {
		aJSON.Addrs = append(aJSON.Addrs, ka)
	}

	rawDats, err := json.MarshalIndent(aJSON, "", "\t")
	if err != nil {
		return err
	}
	return cmn.WriteFileAtomic(a.filePath, rawDats, 0644)
}
