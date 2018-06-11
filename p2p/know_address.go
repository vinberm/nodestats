package p2p

import "time"


const (
	bucketTypeNew = 0x01
	bucketTypeOld = 0x02

	numMissingDays  = 30
	numRetries              = 3

	maxFailures             = 10   // max failures we will accept without a success before considering an address bad.
	minBadDays              = 7    // days since the last success before we will consider evicting an address.

)

type knownAddress struct {
	Addr        *NetAddress
	Src         *NetAddress
	Attempts    int32
	LastAttempt time.Time
	LastSuccess time.Time
	BucketType  byte
	Buckets     []int
}

func newKnownAddress(addr, src *NetAddress) *knownAddress {
	return &knownAddress{
		Addr:        addr,
		Src:         src,
		Attempts:    0,
		LastAttempt: time.Now(),
		BucketType:  bucketTypeNew,
		Buckets:     nil,
	}
}

func (ka *knownAddress) isBad() bool {
	if ka.BucketType == bucketTypeOld {
		return false
	}
	if ka.LastAttempt.After(time.Now().Add(-1*time.Minute)) && ka.Attempts != 0 {
		return true
	}
	if ka.LastAttempt.Before(time.Now().Add(-1 * numMissingDays * time.Hour * 24)) {
		return true
	}
	if ka.LastSuccess.IsZero() && ka.Attempts >= numRetries {
		return true
	}
	if ka.LastSuccess.Before(time.Now().Add(-1*minBadDays*time.Hour*24)) && ka.Attempts >= maxFailures {
		return true
	}
	return false
}

func (ka *knownAddress) addBucketRef(bucketIdx int) int {
	for _, bucket := range ka.Buckets {
		if bucket == bucketIdx {
			return -1
		}
	}
	ka.Buckets = append(ka.Buckets, bucketIdx)
	return len(ka.Buckets)
}

func (ka *knownAddress) removeBucketRef(bucketIdx int) int {
	buckets := []int{}
	for _, bucket := range ka.Buckets {
		if bucket != bucketIdx {
			buckets = append(buckets, bucket)
		}
	}
	if len(buckets) != len(ka.Buckets)-1 {
		return -1
	}
	ka.Buckets = buckets
	return len(ka.Buckets)
}

func (ka *knownAddress) markAttempt() {
	ka.LastAttempt = time.Now()
	ka.Attempts++
}

func (ka *knownAddress) markGood() {
	now := time.Now()
	ka.LastAttempt = now
	ka.LastSuccess = now
	ka.Attempts = 0
}

func (ka *knownAddress) isOld() bool {
	return ka.BucketType == bucketTypeOld
}

func (ka *knownAddress) isNew() bool {
	return ka.BucketType == bucketTypeNew
}
