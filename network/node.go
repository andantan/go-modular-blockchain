package network

import (
	"errors"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
	"net"
)

type Node interface {
	Listen() error
	Connect(addr string) error
	Remove(peer Peer)
	Stop() error
}

type TCPNode struct {
	Logger log.Logger

	ID         string
	ListenAddr string
	Listener   net.Listener

	MessageCh  chan RawMessage
	NewPeerCh  chan Peer
	DelPeerCh  chan Peer
	KnownPeers *types.SyncMap[string, struct{}]
}

func NewTCPNode(
	id string,
	addr string,
	msgCh chan RawMessage,
	newPeerCh chan Peer,
	delPeerCh chan Peer,
) *TCPNode {
	return &TCPNode{
		Logger:     config.LoggerWithPrefixes("node", "id", id, "listen-addr", addr),
		ID:         id,
		ListenAddr: addr,
		MessageCh:  msgCh,
		NewPeerCh:  newPeerCh,
		DelPeerCh:  delPeerCh,
		KnownPeers: types.NewSyncMap[string, struct{}](),
	}
}

func (t *TCPNode) WithLogger(logger log.Logger) *TCPNode {
	t.Logger = logger

	return t
}

func (t *TCPNode) Listen() error {
	ln, err := net.Listen("tcp", t.ListenAddr)
	_ = t.Logger.Log("event", "TCP_listening")

	if err != nil {
		return err
	}

	t.Listener = ln

	go t.acceptLoop()

	return nil
}

func (t *TCPNode) acceptLoop() {
	_ = t.Logger.Log("event", "accepting_TCP_connection")

	for {
		conn, err := t.Listener.Accept()

		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				_ = t.Logger.Log("event", "closed_listener", "peer", conn.RemoteAddr().String())
				return
			}

			_ = t.Logger.Log("event", "accept_error", "error", err)
			continue
		}

		go t.handshakeAndValidate(conn)
	}
}

func (t *TCPNode) Connect(addr string) error {
	_ = t.Logger.Log("event", "attempting_connect", "to", addr)

	conn, err := net.Dial("tcp", addr)

	if err != nil {
		return err
	}

	go t.handshakeAndValidate(conn)

	return nil
}

func (t *TCPNode) Remove(peer Peer) {
	t.KnownPeers.Remove(peer.ID())
}

func (t *TCPNode) Stop() error {
	_ = t.Logger.Log("event", "shutdown")

	return t.Listener.Close()
}

func (t *TCPNode) handshakeAndValidate(conn net.Conn) {
	peer := NewTCPPeer(conn, t.MessageCh, t.DelPeerCh)
	remoteID, err := peer.handshake(t.ID)

	if err != nil {
		_ = t.Logger.Log("event", "handshake_fail", "error", err)
		return
	}

	if !t.KnownPeers.PutIfNotExists(remoteID, struct{}{}) {
		// tie-breaking
		isInbound := conn.LocalAddr() == t.Listener.Addr()
		if isInbound && t.ID > remoteID {
			_ = t.Logger.Log("msg", "tie-breaking: dropping inbound from lower ID peer")
			_ = peer.Conn.Close()
		}
		return
	}

	_ = t.Logger.Log("msg", "new peer connection sending to channel", "peer-id", remoteID, "peer-addr", peer.Addr())

	t.NewPeerCh <- peer
}
