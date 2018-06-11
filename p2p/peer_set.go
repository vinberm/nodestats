package p2p

import "sync"

// PeerSet is a special structure for keeping a table of peers.
// Iteration over the peers is super fast and thread-safe.
type PeerSet struct {
	mtx    sync.Mutex
	lookup map[string]*peerSetItem
	list   []*Peer
}

type peerSetItem struct {
	peer  *Peer
	index int
}

// Remove discards peer if the peer was previously memoized.
func (ps *PeerSet) Remove(peer *Peer) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item := ps.lookup[peer.Key]
	if item == nil {
		return
	}

	index := item.index
	// Copy the list but without the last element.
	// (we must copy because we're mutating the list)
	newList := make([]*Peer, len(ps.list)-1)
	copy(newList, ps.list)
	// If it's the last peer, that's an easy special case.
	if index == len(ps.list)-1 {
		ps.list = newList
		delete(ps.lookup, peer.Key)
		return
	}

	// Move the last item from ps.list to "index" in list.
	lastPeer := ps.list[len(ps.list)-1]
	lastPeerKey := lastPeer.Key
	lastPeerItem := ps.lookup[lastPeerKey]
	newList[index] = lastPeer
	lastPeerItem.index = index
	ps.list = newList
	delete(ps.lookup, peer.Key)
}

// Add adds the peer to the PeerSet.
// Returns false if peer with key (PubKeyEd25519) is already set
func (ps *PeerSet) Add(peer *Peer) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.lookup[peer.Key] != nil {
		return ErrDuplicatePeer
	}

	ps.lookup[peer.Key] = &peerSetItem{peer, len(ps.list)}
	ps.list = append(ps.list, peer)
	return nil
}
