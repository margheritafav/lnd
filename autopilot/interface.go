package autopilot

import (
	"net"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/lnwire"
)

// Node node is an interface which represents n abstract vertex within the
// channel graph. All nodes should have at least a single edge to/from them
// within the graph.
//
// TODO(roasbeef): combine with routing.ChannelGraphSource
type Node interface {
	// PubKey is the identity public key of the node. This will be used to
	// attempt to target a node for channel opening by the main autopilot
	// agent. The key will be returned in serialized compressed format.
	PubKey() [33]byte

	// Addrs returns a slice of publicly reachable public TCP addresses
	// that the peer is known to be listening on.
	Addrs() []net.Addr

	// ForEachChannel is a higher-order function that will be used to
	// iterate through all edges emanating from/to the target node. For
	// each active channel, this function should be called with the
	// populated ChannelEdge that describes the active channel.
	ForEachChannel(func(ChannelEdge) error) error
}

// Channel is a simple struct which contains relevant details of a particular
// channel within the channel graph. The fields in this struct may be used a
// signals for various AttachmentHeuristic implementations.
type Channel struct {
	// ChanID is the short channel ID for this channel as defined within
	// BOLT-0007.
	ChanID lnwire.ShortChannelID

	// Capacity is the capacity of the channel expressed in satoshis.
	Capacity btcutil.Amount

	// FundedAmt is the amount the local node funded into the target
	// channel.
	//
	// TODO(roasbeef): need this?
	FundedAmt btcutil.Amount

	// Node is the peer that this channel has been established with.
	Node NodeID

	// TODO(roasbeef): also add other traits?
	//  * fee, timelock, etc
}

// ChannelEdge is a struct that holds details concerning a channel, but also
// contains a reference to the Node that this channel connects to as a directed
// edge within the graph. The existence of this reference to the connected node
// will allow callers to traverse the graph in an object-oriented manner.
type ChannelEdge struct {
	// Channel contains the attributes of this channel.
	Channel

	// Peer is the peer that this channel creates an edge to in the channel
	// graph.
	Peer Node
}

// ChannelGraph in an interface that represents a traversable channel graph.
// The autopilot agent will use this interface as its source of graph traits in
// order to make decisions concerning which channels should be opened, and to
// whom.
//
// TODO(roasbeef): abstract??
type ChannelGraph interface {
	// ForEachNode is a higher-order function that should be called once
	// for each connected node within the channel graph. If the passed
	// callback returns an error, then execution should be terminated.
	ForEachNode(func(Node) error) error
}

// NodeScore is a tuple mapping a NodeID to a score indicating the preference
// of opening a channel with it.
type NodeScore struct {
	// NodeID is the serialized compressed pubkey of the node that is being
	// scored.
	NodeID NodeID

	// Score is the score given by the heuristic for opening a channel of
	// the given size to this node.
	Score float64
}

// AttachmentDirective describes a channel attachment proscribed by an
// AttachmentHeuristic. It details to which node a channel should be created
// to, and also the parameters which should be used in the channel creation.
type AttachmentDirective struct {
	// NodeID is the serialized compressed pubkey of the target node for
	// this attachment directive. It can be identified by its public key,
	// and therefore can be used along with a ChannelOpener implementation
	// to execute the directive.
	NodeID NodeID

	// ChanAmt is the size of the channel that should be opened, expressed
	// in satoshis.
	ChanAmt btcutil.Amount

	// Addrs is a list of addresses that the target peer may be reachable
	// at.
	Addrs []net.Addr
}

// AttachmentHeuristic is one of the primary interfaces within this package.
// Implementations of this interface will be used to implement a control system
// which automatically regulates channels of a particular agent, attempting to
// optimize channels opened/closed based on various heuristics.  The purpose of
// the interface is to allow an auto-pilot agent to decide if it needs more
// channels, and if so, which exact channels should be opened.
type AttachmentHeuristic interface {
	// NodeScores is a method that given the current channel graph and
	// current set of local channels, scores the given nodes according to
	// the preference of opening a channel of the given size with them. The
	// returned channel candidates maps the NodeID to a NodeScore for the
	// node.
	//
	// The returned scores will be in the range [0, 1.0], where 0 indicates
	// no improvement in connectivity if a channel is opened to this node,
	// while 1.0 is the maximum possible improvement in connectivity. The
	// implementation of this interface must return scores in this range to
	// properly allow the autopilot agent to make a reasonable choice based
	// on the score from multiple heuristics.
	//
	// NOTE: A NodeID not found in the returned map is implicitly given a
	// score of 0.
	NodeScores(g ChannelGraph, chans []Channel,
		chanSize btcutil.Amount, nodes map[NodeID]struct{}) (
		map[NodeID]*NodeScore, error)
}

// ChannelController is a simple interface that allows an auto-pilot agent to
// open a channel within the graph to a target peer, close targeted channels,
// or add/remove funds from existing channels via a splice in/out mechanisms.
type ChannelController interface {
	// OpenChannel opens a channel to a target peer, with a capacity of the
	// specified amount. This function should un-block immediately after
	// the funding transaction that marks the channel open has been
	// broadcast.
	OpenChannel(target *btcec.PublicKey, amt btcutil.Amount) error

	// CloseChannel attempts to close out the target channel.
	//
	// TODO(roasbeef): add force option?
	CloseChannel(chanPoint *wire.OutPoint) error

	// SpliceIn attempts to add additional funds to the target channel via
	// a splice in mechanism. The new channel with an updated capacity
	// should be returned.
	SpliceIn(chanPoint *wire.OutPoint, amt btcutil.Amount) (*Channel, error)

	// SpliceOut attempts to remove funds from an existing channels using a
	// splice out mechanism. The removed funds from the channel should be
	// returned to an output under the control of the backing wallet.
	SpliceOut(chanPoint *wire.OutPoint, amt btcutil.Amount) (*Channel, error)
}
