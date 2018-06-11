package p2p

import (
	"sync"
	tcrypto "github.com/tendermint/go-crypto"

	"math"
	"math/rand"
)

const (
	needAddressThreshold = 1000
)


type AddrBook struct {
	key      string
	filePath string
	mtx sync.RWMutex
	rand       *rand.Rand
	ourAddrs   map[string]*NetAddress
	addrLookup map[string]*knownAddress // new & old

	bucketsNew []map[string]*knownAddress
	bucketsOld []map[string]*knownAddress

	nOld       int
	nNew       int
}

func NewAddrBook(filePath string) *AddrBook {
	a := &AddrBook{
		key:      tcrypto.CRandHex(24),
		filePath: filePath,
		ourAddrs: make(map[string]*NetAddress),
	}
	return a
}

// PickAddress picks a random address from random bucket
func (a *AddrBook) PickAddress(bias int) *NetAddress {
	a.mtx.RLock()
	defer a.mtx.RUnlock()

	if a.size() == 0 {
		return nil
	}

	// make sure bias is in the range [0, 100]
	if bias > 100 {
		bias = 100
	} else if bias < 0 {
		bias = 0
	}

	oldCorrelation := math.Sqrt(float64(a.nOld)) * (100.0 - float64(bias))
	newCorrelation := math.Sqrt(float64(a.nNew)) * float64(bias)
	pickFromOldBucket := (newCorrelation+oldCorrelation)*a.rand.Float64() < oldCorrelation
	if (pickFromOldBucket && a.nOld == 0) || (!pickFromOldBucket && a.nNew == 0) {
		return nil
	}

	var bucket map[string]*knownAddress
	for len(bucket) == 0 {
		if pickFromOldBucket {
			bucket = a.bucketsOld[a.rand.Intn(len(a.bucketsOld))]
		} else {
			bucket = a.bucketsNew[a.rand.Intn(len(a.bucketsNew))]
		}
	}

	randIndex := a.rand.Intn(len(bucket))
	for _, ka := range bucket {
		if randIndex == 0 {
			return ka.Addr
		}
		randIndex--
	}
	return nil
}

// Size count the number of know address
func (a *AddrBook) Size() int {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.size()
}

func (a *AddrBook) size() int {
	return a.nNew + a.nOld
}

// MarkGood marks the peer as good and moves it into an "old" bucket.
func (a *AddrBook) MarkGood(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	ka := a.addrLookup[addr.String()]
	if ka == nil {
		return
	}

	ka.markGood()
	//if ka.isNew() {
	//	if err := a.moveToOld(ka); err != nil {
	//		log.WithField("err", err).Error("fail on move to old bucket")
	//	}
	//}
}

// MarkAttempt marks that an attempt was made to connect to the address.
func (a *AddrBook) MarkAttempt(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if ka := a.addrLookup[addr.String()]; ka != nil {
		ka.markAttempt()
		ka.markAttempt()
	}
}

// NeedMoreAddrs check does the address number meet the threshold
func (a *AddrBook) NeedMoreAddrs() bool {
	return a.Size() < needAddressThreshold
}
