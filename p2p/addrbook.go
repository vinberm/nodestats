package p2p

import (
	"errors"

	"sync"
	tcrypto "github.com/tendermint/go-crypto"
	log "github.com/sirupsen/logrus"

	"github.com/nodestats/crypto"

	"math"
	"math/rand"
	"encoding/binary"
	"net"
)

const (
	needAddressThreshold = 1000
	maxNewBucketsPerAddress = 4
	newBucketsPerGroup = 32

	newBucketCount     = 256
	newBucketSize      = 64
)


type AddrBook struct {
	key      string
	filePath string
	routabilityStrict bool

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

// AddAddress add address to address book
func (a *AddrBook) AddAddress(addr, src *NetAddress) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.addAddress(addr, src)
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

func (a *AddrBook) getBucket(bucketType byte, bucketIdx int) map[string]*knownAddress {
	switch bucketType {
	case bucketTypeNew:
		return a.bucketsNew[bucketIdx]
	case bucketTypeOld:
		return a.bucketsOld[bucketIdx]
	default:
		log.Error("try to access an unknow address book bucket type")
		return nil
	}
}

func (a *AddrBook) addToNewBucket(ka *knownAddress, bucketIdx int) error {
	if ka.isOld() {
		return errors.New("cant add old address to new bucket")
	}

	addrStr := ka.Addr.String()
	bucket := a.getBucket(bucketTypeNew, bucketIdx)
	if _, ok := bucket[addrStr]; ok {
		return nil
	}

	if len(bucket) > newBucketSize {
		a.expireNew(bucketIdx)
	}

	bucket[addrStr] = ka
	a.addrLookup[addrStr] = ka
	if ka.addBucketRef(bucketIdx) == 1 {
		a.nNew++
	}
	return nil
}

func (a *AddrBook) addAddress(addr, src *NetAddress) error {
	if addr == nil || src == nil {
		return errors.New("can't add nil to address book")
	}
	if _, ok := a.ourAddrs[addr.String()]; ok {
		return errors.New("add ourselves to address book")
	}
	//if a.routabilityStrict && !addr.Routable() {
	//	return errors.New("cannot add non-routable address")
	//}

	ka := a.addrLookup[addr.String()]
	if ka != nil {
		if ka.isOld() {
			return nil
		}
		if len(ka.Buckets) == maxNewBucketsPerAddress {
			return nil
		}
		if factor := int32(2 * len(ka.Buckets)); a.rand.Int31n(factor) != 0 {
			return nil
		}
	} else {
		ka = newKnownAddress(addr, src)
	}

	bucket := a.calcNewBucket(addr, src)
	return a.addToNewBucket(ka, bucket)
}

func (a *AddrBook) calcNewBucket(addr, src *NetAddress) int {
	data1 := []byte{}
	data1 = append(data1, []byte(a.key)...)
	data1 = append(data1, []byte(a.groupKey(addr))...)
	data1 = append(data1, []byte(a.groupKey(src))...)
	hash1 := crypto.DoubleSha256(data1)
	hash64 := binary.BigEndian.Uint64(hash1)
	hash64 %= newBucketsPerGroup
	var hashbuf [8]byte
	binary.BigEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, []byte(a.key)...)
	data2 = append(data2, a.groupKey(src)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := crypto.DoubleSha256(data2)
	return int(binary.BigEndian.Uint64(hash2) % newBucketCount)
}

func (a *AddrBook) groupKey(na *NetAddress) string {
	if a.routabilityStrict && na.Local() {
		return "local"
	}
	if a.routabilityStrict && !na.Routable() {
		return "unroutable"
	}
	if ipv4 := na.IP.To4(); ipv4 != nil {
		return (&net.IPNet{IP: na.IP, Mask: net.CIDRMask(16, 32)}).String()
	}
	if na.RFC6145() || na.RFC6052() {
		// last four bytes are the ip address
		ip := net.IP(na.IP[12:16])
		return (&net.IPNet{IP: ip, Mask: net.CIDRMask(16, 32)}).String()
	}
	if na.RFC3964() {
		ip := net.IP(na.IP[2:7])
		return (&net.IPNet{IP: ip, Mask: net.CIDRMask(16, 32)}).String()

	}
	if na.RFC4380() {
		// teredo tunnels have the last 4 bytes as the v4 address XOR 0xff.
		ip := net.IP(make([]byte, 4))
		for i, byte := range na.IP[12:16] {
			ip[i] = byte ^ 0xff
		}
		return (&net.IPNet{IP: ip, Mask: net.CIDRMask(16, 32)}).String()
	}

	bits := 32
	heNet := &net.IPNet{IP: net.ParseIP("2001:470::"), Mask: net.CIDRMask(32, 128)}
	if heNet.Contains(na.IP) {
		bits = 36
	}
	return (&net.IPNet{IP: na.IP, Mask: net.CIDRMask(bits, 128)}).String()
}

func (a *AddrBook) pickOldest(bucketType byte, bucketIdx int) *knownAddress {
	bucket := a.getBucket(bucketType, bucketIdx)
	var oldest *knownAddress
	for _, ka := range bucket {
		if oldest == nil || ka.LastAttempt.Before(oldest.LastAttempt) {
			oldest = ka
		}
	}
	return oldest
}

func (a *AddrBook) expireNew(bucketIdx int) {
	for _, ka := range a.bucketsNew[bucketIdx] {
		if ka.isBad() {
			a.removeFromBucket(ka, bucketIdx)
			return
		}
	}

	oldest := a.pickOldest(bucketTypeNew, bucketIdx)
	a.removeFromBucket(oldest, bucketIdx)
}


func (a *AddrBook) removeFromBucket(ka *knownAddress, bucketIdx int) {
	bucket := a.getBucket(ka.BucketType, bucketIdx)
	delete(bucket, ka.Addr.String())
	if ka.removeBucketRef(bucketIdx) == 0 {
		delete(a.addrLookup, ka.Addr.String())
		if ka.BucketType == bucketTypeNew {
			a.nNew--
		} else {
			a.nOld--
		}
	}
}
