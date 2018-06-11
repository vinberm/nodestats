package p2p

import (
	"errors"
	"sync"

	"github.com/tendermint/go-crypto"
	//dbm "github.com/tendermint/tmlibs/db"
	log "github.com/sirupsen/logrus"
	cmn "github.com/tendermint/tmlibs/common"
	cfg "github.com/nodestats/config"

	"github.com/nodestats/p2p/connection"

	"time"
)

//pre-define errors for connecting fail
var (
	ErrDuplicatePeer     = errors.New("Duplicate peer")
	ErrConnectSelf       = errors.New("Connect self")
	ErrConnectBannedPeer = errors.New("Connect banned peer")
)

type Switch struct {
	Config       *cfg.Config
	peerConfig   *PeerConfig

	chDescs      []*connection.ChannelDescriptor
	reactorsByCh map[byte]Reactor
	addrBook     *AddrBook
	nodePrivKey  crypto.PrivKeyEd25519
	peers        *PeerSet
	dialing		 *cmn.CMap
	nodeInfo     *NodeInfo
	mtx          sync.Mutex
	reactors     map[string]Reactor
}

func NewSwitch(config *cfg.Config, addrBook *AddrBook) *Switch {
	sw := &Switch{
		onfig: config,
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

// List threadsafe list of peers.
func (ps *PeerSet) List() []*Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.list
}

//Peers return switch peerset
func (sw *Switch) Peers() *PeerSet {
	return sw.peers
}

// NumPeers Returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

//IsDialing prevent duplicate dialing
func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.IP.String())
}

// AddPeer performs the P2P handshake with a peer
// that already has a SecretConnection. If all goes well,
// it starts the peer and adds it to the switch.
// NOTE: This performs a blocking handshake before the peer is added.
// CONTRACT: If error is returned, peer is nil, and conn is immediately closed.
func (sw *Switch) AddPeer(pc *peerConn) error {
	peerNodeInfo, err := pc.HandshakeTimeout(sw.nodeInfo, time.Duration(sw.peerConfig.HandshakeTimeout*time.Second))
	if err != nil {
		return err
	}

	if err := sw.nodeInfo.CompatibleWith(peerNodeInfo); err != nil {
		return err
	}

	peer := newPeer(pc, peerNodeInfo, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)
	//if err := sw.filterConnByPeer(peer); err != nil {
	//	return err
	//}

	if err := sw.startInitPeer(peer); err != nil {
		return err
	}

	return sw.peers.Add(peer)
}

//DialPeerWithAddress dial node from net address
func (sw *Switch) DialPeerWithAddress(addr *NetAddress) error {
	log.Debug("Dialing peer address:", addr)
	sw.dialing.Set(addr.IP.String(), addr)
	defer sw.dialing.Delete(addr.IP.String())
	//if err := sw.filterConnByIP(addr.IP.String()); err != nil {
	//	return err
	//}

	pc, err := newOutboundPeerConn(addr, sw.nodePrivKey, sw.peerConfig)
	if err != nil {
		log.WithFields(log.Fields{"address": addr, " err": err}).Debug("DialPeer fail on newOutboundPeerConn")
		return err
	}

	if err = sw.AddPeer(pc); err != nil {
		log.WithFields(log.Fields{"address": addr, " err": err}).Debug("DialPeer fail on switch AddPeer")
		pc.CloseConn()
		return err
	}
	log.Debug("DialPeer added peer:", addr)
	return nil
}

// StopPeerForError disconnects from a peer due to external error.
func (sw *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	log.WithFields(log.Fields{"peer": peer, " err": reason}).Debug("stopping peer for error")
	sw.stopAndRemovePeer(peer, reason)
}

func (sw *Switch) startInitPeer(peer *Peer) error {
	peer.Start() // spawn send/recv routines
	for _, reactor := range sw.reactors {
		if err := reactor.AddPeer(peer); err != nil {
			return err
		}
	}
	return nil
}

func (sw *Switch) stopAndRemovePeer(peer *Peer, reason interface{}) {
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
	sw.peers.Remove(peer)
	peer.Stop()
}

// StopPeerGracefully disconnect from a peer gracefully.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	sw.stopAndRemovePeer(peer, nil)
}

// NodeInfo returns the switch's NodeInfo.
// NOTE: Not goroutine safe.
func (sw *Switch) NodeInfo() *NodeInfo {
	return sw.nodeInfo
}

