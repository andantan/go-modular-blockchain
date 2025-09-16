package network

import (
	"errors"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
	"net"
	"time"
)

type Node interface {
	Listen() error
	Connect(addr string) error
	ConsumePeer() (chan Peer, chan Peer)
	ConsumeMessage() chan RawMessage
	Remove(peer Peer)
	Stop() error
}

type TCPNode struct {
	logger log.Logger

	listenAddr string
	listener   net.Listener
	domain     string

	privKey crypto.PrivateKey
	pubKey  crypto.PublicKey
	address types.Address

	messageCh  chan RawMessage
	newPeerCh  chan Peer
	delPeerCh  chan Peer
	quitCh     chan struct{}
	knownPeers *types.SyncMap[types.Address, struct{}]
}

func NewTCPNode(privKey crypto.PrivateKey, listenAddr string, domain string, quitCh chan struct{}) *TCPNode {
	t := &TCPNode{
		listenAddr: listenAddr,
		domain:     domain,
		privKey:    privKey,
		pubKey:     privKey.PublicKey(),
		address:    privKey.PublicKey().Address(),
		messageCh:  make(chan RawMessage),
		newPeerCh:  make(chan Peer),
		delPeerCh:  make(chan Peer),
		quitCh:     quitCh,
		knownPeers: types.NewSyncMap[types.Address, struct{}](),
	}

	t.logger = config.LoggerWithPrefixes("node", "address", t.address.ShortString(8), "listen-addr", t.listenAddr, "domain", domain)

	return t
}

func (t *TCPNode) WithLogger(logger log.Logger) *TCPNode {
	t.logger = logger

	return t
}

func (t *TCPNode) Listen() error {
	ln, err := net.Listen("tcp", t.listenAddr)

	if err != nil {
		return err
	}
	_ = t.logger.Log("event", "TCP_listening", "")

	t.listener = ln

	go t.acceptLoop()

	return nil
}

func (t *TCPNode) acceptLoop() {
	defer close(t.newPeerCh)
	defer close(t.delPeerCh)
	defer close(t.messageCh)

	_ = t.logger.Log("event", "accept_loop_started")

	acceptCh := make(chan net.Conn)
	errCh := make(chan error, 1)

	go func() {
		for {
			conn, err := t.listener.Accept()
			if err != nil {
				errCh <- err
				return
			}
			acceptCh <- conn
		}
	}()

	for {
		select {
		case conn := <-acceptCh:
			go t.handshakeAndValidate(conn)

		case err := <-errCh:
			if errors.Is(err, net.ErrClosed) {
				_ = t.logger.Log("msg", "accept loop terminated due to closed listener")
				return
			}
			_ = t.logger.Log("error", "accept error", "err", err)

		case <-t.quitCh:
			_ = t.logger.Log("msg", "server shutdown signal received, terminating accept loop")
			return
		}
	}
}

func (t *TCPNode) Connect(addr string) error {
	_ = t.logger.Log("event", "attempting_connect", "to", addr)

	conn, err := net.Dial("tcp", addr)

	if err != nil {
		return err
	}

	go t.handshakeAndValidate(conn)

	return nil
}

func (t *TCPNode) ConsumePeer() (chan Peer, chan Peer) {
	return t.newPeerCh, t.delPeerCh
}

func (t *TCPNode) ConsumeMessage() chan RawMessage {
	return t.messageCh
}

func (t *TCPNode) Remove(peer Peer) {
	info := peer.Identity()
	t.knownPeers.Remove(info.Address)
}

func (t *TCPNode) Stop() error {
	_ = t.logger.Log("event", "shutdown")

	return t.listener.Close()
}

func (t *TCPNode) handshakeAndValidate(conn net.Conn) {
	ourIdentity := &PeerIdentity{
		Address:       t.address,
		NetAddr:       t.listenAddr,
		Domain:        t.domain,
		ConnectedTime: time.Now().UnixNano(),
		PublicKey:     t.pubKey,
		IsValidator:   false,
	}

	if err := ourIdentity.Sign(t.privKey); err != nil {
		_ = conn.Close()
		fmt.Printf("failed to sign our identity: %v\n", err)
		return
	}

	if err := ourIdentity.Verify(); err != nil {
		_ = conn.Close()
		fmt.Printf("failed to verify our identity: %v\n", err)
		return
	}

	peer := NewTCPPeer(conn, t.messageCh, t.delPeerCh)
	remoteIdentity, err := peer.handshake(ourIdentity)

	if err != nil {
		_ = t.logger.Log("event", "handshake_fail", "error", err)
		return
	}

	if !t.knownPeers.PutIfNotExists(remoteIdentity.Address, struct{}{}) {
		// tie-breaking
		isInbound := conn.LocalAddr() == t.listener.Addr()
		if isInbound && t.address.String() > remoteIdentity.Address.String() {
			_ = t.logger.Log("msg", "tie-breaking: dropping inbound from lower ID peer")
			_ = peer.conn.Close()
		}
		return
	}

	t.newPeerCh <- peer
}
