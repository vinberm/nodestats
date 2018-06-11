package reactor

import (
	"math/rand"

	"github.com/nodestats/p2p"

	log "github.com/sirupsen/logrus"
	cmn "github.com/tendermint/tmlibs/common"
	"time"
	"sync"
)

const (
	// PexChannel is a channel for PEX messages
	PexChannel = byte(0x00)

	minNumOutboundPeers      = 5
	maxPexMessageSize        = 1048576 // 1MB
	defaultMaxMsgCountByPeer = uint16(1000)
)

// PEXReactor handles peer exchange and ensures that an adequate number of peers are connected to the switch.
type PEXReactor struct {
	p2p.BaseReactor
	book           *p2p.AddrBook
	msgCountByPeer *cmn.CMap
}

// NewPEXReactor creates new PEX reactor.
func NewPEXReactor(b *p2p.AddrBook) *PEXReactor {
	r := &PEXReactor{
		book:           b,
		msgCountByPeer: cmn.NewCMap(),
	}
	r.BaseReactor = *p2p.NewBaseReactor("PEXReactor", r)
	return r
}

// OnStart implements BaseService
func (r *PEXReactor) OnStart() error {
	r.BaseReactor.OnStart()


	go r.ensurePeersRoutine()
	go r.flushMsgCountByPeer()
	return nil
}

// OnStop implements BaseService
func (r *PEXReactor) OnStop() {
	r.BaseReactor.OnStop()
	r.book.Stop()
}

func (r *PEXReactor) dialPeerWorker(a *p2p.NetAddress, wg *sync.WaitGroup) {
	if err := r.Switch.DialPeerWithAddress(a); err != nil {
		r.book.MarkAttempt(a)
	} else {
		r.book.MarkGood(a)
	}
	wg.Done()
}

//
func (r *PEXReactor) ensurePeers() {
	numOutPeers, _, numDialing := r.Switch.NumPeers()
	numToDial := (minNumOutboundPeers - (numOutPeers + numDialing)) * 3
	log.WithFields(log.Fields{
		"numOutPeers": numOutPeers,
		"numDialing":  numDialing,
		"numToDial":   numToDial,
	}).Debug("ensure peers")
	if numToDial <= 0 {
		return
	}

	newBias := cmn.MinInt(numOutPeers, 8)*10 + 10
	toDial := make(map[string]*p2p.NetAddress)
	maxAttempts := numToDial * 3

	connectedPeers := make(map[string]struct{})
	for _, peer := range r.Switch.Peers().List() {
		connectedPeers[peer.RemoteAddrHost()] = struct{}{}
	}

	for i := 0; i < maxAttempts && len(toDial) < numToDial; i++ {
		try := r.book.PickAddress(newBias)
		if try == nil {
			continue
		}
		if _, selected := toDial[try.IP.String()]; selected {
			continue
		}
		if dialling := r.Switch.IsDialing(try); dialling {
			continue
		}
		if _, ok := connectedPeers[try.IP.String()]; ok {
			continue
		}

		log.Debug("Will dial address addr:", try)
		toDial[try.IP.String()] = try
	}

	var wg sync.WaitGroup
	for _, item := range toDial {
		wg.Add(1)
		go r.dialPeerWorker(item, &wg)
	}
	wg.Wait()

	if r.book.NeedMoreAddrs() {
		if peers := r.Switch.Peers().List(); len(peers) > 0 {
			peer := peers[rand.Int()%len(peers)]
			r.RequestAddrs(peer)
		}
	}
}

func (r *PEXReactor) ensurePeersRoutine() {
	r.ensurePeers()
	if r.Switch.Peers().Size() < 3 {
		r.dialSeeds()
	}

	ticker := time.NewTicker(120 * time.Second)
	quickTicker := time.NewTicker(3 * time.Second)

	for {
		select {
		case <-ticker.C:
			r.ensurePeers()
		case <-quickTicker.C:
			if r.Switch.Peers().Size() < 3 {
				r.ensurePeers()
			}
		case <-r.Quit:
			return
		}
	}
}

// RequestAddrs asks peer for more addresses.
func (r *PEXReactor) RequestAddrs(p *p2p.Peer) bool {
	ok := p.TrySend(PexChannel, struct{ PexMessage }{&pexRequestMessage{}})
	if !ok {
		r.Switch.StopPeerGracefully(p)
	}
	return ok
}


