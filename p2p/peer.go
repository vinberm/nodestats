package p2p

import (
	"net"

	"github.com/pkg/errors"
	cfg "github.com/nodestats/config"
	log "github.com/sirupsen/logrus"

	"github.com/nodestats/p2p/connection"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/go-crypto"
	"time"
	"github.com/tendermint/go-wire"
)

// peerConn contains the raw connection and its config.
type peerConn struct {
	outbound bool
	config   *PeerConfig
	conn     net.Conn // source connection
}

// PeerConfig is a Peer configuration.
type PeerConfig struct {
	HandshakeTimeout time.Duration           `mapstructure:"handshake_timeout"` // times are in seconds
	DialTimeout      time.Duration           `mapstructure:"dial_timeout"`
	MConfig          *connection.MConnConfig `mapstructure:"connection"`
}

// DefaultPeerConfig returns the default config.
func DefaultPeerConfig(config *cfg.P2PConfig) *PeerConfig {
	return &PeerConfig{
		HandshakeTimeout: time.Duration(config.HandshakeTimeout), // * time.Second,
		DialTimeout:      time.Duration(config.DialTimeout),      // * time.Second,
		MConfig:          connection.DefaultMConnConfig(),
	}
}

// Peer represent a bytom network node
type Peer struct {
	// raw peerConn and the multiplex connection
	*peerConn
	mconn *connection.MConnection // multiplex connection

	*NodeInfo
	Key  string
	//Data *cmn.CMap // User data.
}

// OnStart implements BaseService.
func (p *Peer) Start() error {
	_, err := p.mconn.Start()
	return err
}


// OnStop implements BaseService.
func (p *Peer) Stop() {
	//p.BaseService.OnStop()
	p.mconn.Stop()
}

func newPeer(pc *peerConn, nodeInfo *NodeInfo, reactorsByCh map[byte]Reactor, chDescs []*connection.ChannelDescriptor, onPeerError func(*Peer, interface{})) *Peer {
	// Key and NodeInfo are set after Handshake
	p := &Peer{
		peerConn: pc,
		NodeInfo: nodeInfo,
		Key:      nodeInfo.PubKey.KeyString(),
	}
	p.mconn = createMConnection(pc.conn, p, reactorsByCh, chDescs, onPeerError, pc.config.MConfig)
	//p.BaseService = *cmn.NewBaseService(nil, "Peer", p)
	return p
}

func newPeerConn(rawConn net.Conn, outbound bool, ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig) (*peerConn, error) {
	rawConn.SetDeadline(time.Now().Add(config.HandshakeTimeout * time.Second))
	conn, err := connection.MakeSecretConnection(rawConn, ourNodePrivKey)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating peer")
	}

	return &peerConn{
		config:   config,
		outbound: outbound,
		conn:     conn,
	}, nil
}

func newOutboundPeerConn(addr *NetAddress, ourNodePrivKey crypto.PrivKeyEd25519, config *PeerConfig) (*peerConn, error) {
	conn, err := dial(addr, config)
	if err != nil {
		return nil, errors.Wrap(err, "Error dial peer")
	}

	pc, err := newPeerConn(conn, true, ourNodePrivKey, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return pc, nil
}

func dial(addr *NetAddress, config *PeerConfig) (net.Conn, error) {
	conn, err := addr.DialTimeout(config.DialTimeout * time.Second)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// HandshakeTimeout performs a handshake between a given node and the peer.
// NOTE: blocking
func (pc *peerConn) HandshakeTimeout(ourNodeInfo *NodeInfo, timeout time.Duration) (*NodeInfo, error) {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	pc.conn.SetDeadline(time.Now().Add(timeout))

	var peerNodeInfo = new(NodeInfo)
	var err1, err2 error
	cmn.Parallel(
		func() {
			var n int
			wire.WriteBinary(ourNodeInfo, pc.conn, &n, &err1)
		},
		func() {
			var n int
			wire.ReadBinary(peerNodeInfo, pc.conn, maxNodeInfoSize, &n, &err2)
			log.WithField("peerNodeInfo", peerNodeInfo).Info("Peer handshake")
		})
	if err1 != nil {
		return peerNodeInfo, errors.Wrap(err1, "Error during handshake/write")
	}
	if err2 != nil {
		return peerNodeInfo, errors.Wrap(err2, "Error during handshake/read")
	}

	// Remove deadline
	pc.conn.SetDeadline(time.Time{})
	peerNodeInfo.RemoteAddr = pc.conn.RemoteAddr().String()
	return peerNodeInfo, nil
}

// CloseConn should be used when the peer was created, but never started.
func (pc *peerConn) CloseConn() {
	pc.conn.Close()
}

// TrySend msg to the channel identified by chID byte. Immediately returns
// false if the send queue is full.
func (p *Peer) TrySend(chID byte, msg interface{}) bool {

	return p.mconn.TrySend(chID, msg)
}

func createMConnection(conn net.Conn, p *Peer, reactorsByCh map[byte]Reactor, chDescs []*connection.ChannelDescriptor, onPeerError func(*Peer, interface{}), config *connection.MConnConfig) *connection.MConnection {
	onReceive := func(chID byte, msgBytes []byte) {
		reactor := reactorsByCh[chID]
		if reactor == nil {
			cmn.PanicSanity(cmn.Fmt("Unknown channel %X", chID))
		}
		reactor.Receive(chID, p, msgBytes)
	}

	onError := func(r interface{}) {
		onPeerError(p, r)
	}
	return connection.NewMConnectionWithConfig(conn, chDescs, onReceive, onError, config)
}
