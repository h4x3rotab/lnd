package htlcswitch

import (
	"github.com/BTCGPU/lnd/channeldb"
	"github.com/BTCGPU/lnd/lnwire"
	"github.com/roasbeef/btcd/chaincfg/chainhash"
	"github.com/roasbeef/btcd/wire"
)

// InvoiceDatabase is an interface which represents the persistent subsystem
// which may search, lookup and settle invoices.
type InvoiceDatabase interface {
	// LookupInvoice attempts to look up an invoice according to its 32
	// byte payment hash.
	LookupInvoice(chainhash.Hash) (channeldb.Invoice, error)

	// SettleInvoice attempts to mark an invoice corresponding to the
	// passed payment hash as fully settled.
	SettleInvoice(chainhash.Hash) error
}

// ChannelLink is an interface which represents the subsystem for managing the
// incoming htlc requests, applying the changes to the channel, and also
// propagating/forwarding it to htlc switch.
//
//  abstraction level
//       ^
//       |
//       | - - - - - - - - - - - - Lightning - - - - - - - - - - - - -
//       |
//       | (Switch)		     (Switch)		       (Switch)
//       |  Alice <-- channel link --> Bob <-- channel link --> Carol
//       |
//       | - - - - - - - - - - - - - TCP - - - - - - - - - - - - - - -
//       |
//       |  (Peer) 		     (Peer)	                (Peer)
//       |  Alice <----- tcp conn --> Bob <---- tcp conn -----> Carol
//       |
//
type ChannelLink interface {
	// TODO(roasbeef): modify interface to embed mail boxes?

	// HandleSwitchPacket handles the switch packets. This packets might be
	// forwarded to us from another channel link in case the htlc update
	// came from another peer or if the update was created by user
	// initially.
	//
	// NOTE: This function MUST be non-blocking (or block as little as
	// possible).
	HandleSwitchPacket(*htlcPacket) error

	// HandleChannelUpdate handles the htlc requests as settle/add/fail
	// which sent to us from remote peer we have a channel with.
	//
	// NOTE: This function MUST be non-blocking (or block as little as
	// possible).
	HandleChannelUpdate(lnwire.Message)

	// ChanID returns the channel ID for the channel link. The channel ID
	// is a more compact representation of a channel's full outpoint.
	ChanID() lnwire.ChannelID

	// ShortChanID returns the short channel ID for the channel link. The
	// short channel ID encodes the exact location in the main chain that
	// the original funding output can be found.
	ShortChanID() lnwire.ShortChannelID

	// UpdateShortChanID updates the short channel ID for a link. This may
	// be required in the event that a link is created before the short
	// chan ID for it is known, or a re-org occurs, and the funding
	// transaction changes location within the chain.
	UpdateShortChanID() (lnwire.ShortChannelID, error)

	// UpdateForwardingPolicy updates the forwarding policy for the target
	// ChannelLink. Once updated, the link will use the new forwarding
	// policy to govern if it an incoming HTLC should be forwarded or not.
	UpdateForwardingPolicy(ForwardingPolicy)

	// HtlcSatifiesPolicy should return a nil error if the passed HTLC
	// details satisfy the current forwarding policy fo the target link.
	// Otherwise, a valid protocol failure message should be returned in
	// order to signal to the source of the HTLC, the policy consistency
	// issue.
	HtlcSatifiesPolicy(payHash [32]byte,
		incomingAmt, amtToForward lnwire.MilliSatoshi) lnwire.FailureMessage

	// Bandwidth returns the amount of milli-satoshis which current link
	// might pass through channel link. The value returned from this method
	// represents the up to date available flow through the channel. This
	// takes into account any forwarded but un-cleared HTLC's, and any
	// HTLC's which have been set to the over flow queue.
	Bandwidth() lnwire.MilliSatoshi

	// Stats return the statistics of channel link. Number of updates,
	// total sent/received milli-satoshis.
	Stats() (uint64, lnwire.MilliSatoshi, lnwire.MilliSatoshi)

	// Peer returns the representation of remote peer with which we have
	// the channel link opened.
	Peer() Peer

	// EligibleToForward returns a bool indicating if the channel is able
	// to actively accept requests to forward HTLC's. A channel may be
	// active, but not able to forward HTLC's if it hasn't yet finalized
	// the pre-channel operation protocol with the remote peer. The switch
	// will use this function in forwarding decisions accordingly.
	EligibleToForward() bool

	// AttachMailBox delivers an active MailBox to the link. The MailBox may
	// have buffered messages.
	AttachMailBox(MailBox)

	// Start/Stop are used to initiate the start/stop of the channel link
	// functioning.
	Start() error
	Stop()
}

// Peer is an interface which represents the remote lightning node inside our
// system.
type Peer interface {
	// SendMessage sends message to remote peer. The second argument
	// denotes if the method should block until the message has been sent
	// to the remote peer.
	SendMessage(msg lnwire.Message, sync bool) error

	// WipeChannel removes the channel uniquely identified by its channel
	// point from all indexes associated with the peer.
	WipeChannel(*wire.OutPoint) error

	// PubKey returns the serialize public key of the source peer.
	PubKey() [33]byte
}

// ForwardingLog is an interface that represents a time series database which
// keep track of all successfully completed payment circuits. Every few
// seconds, the switch will collate and flush out all the successful payment
// circuits during the last interval.
type ForwardingLog interface {
	// AddForwardingEvents is a method that should write out the set of
	// forwarding events in a batch to persistent storage. Outside
	// sub-systems can then query the contents of the log for analysis,
	// visualizations, etc.
	AddForwardingEvents([]channeldb.ForwardingEvent) error
}
