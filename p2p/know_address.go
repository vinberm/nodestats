package p2p

import "time"


const (
	bucketTypeNew = 0x01
	bucketTypeOld = 0x02
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
