package p2p

import (
	"net"
	cfg "github.com/nodestats/config"
	"github.com/nodestats/p2p/connection"
	cmn "github.com/tendermint/tmlibs/common"
)

// peerConn contains the raw connection and its config.
type peerConn struct {
	outbound bool
	config   *cfg.Config
	conn     net.Conn // source connection
}

// Peer represent a bytom network node
type Peer struct {
	// raw peerConn and the multiplex connection
	*peerConn
	mconn *connection.MConnection // multiplex connection

	*NodeInfo
	Key  string
	Data *cmn.CMap // User data.
}