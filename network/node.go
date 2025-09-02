package network

import (
	"errors"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/go-kit/log"
	"net"
)

type Node interface {
	Listen() error
	Connect(addr string) error
	Stop() error
}

type TCPNode struct {
	PeerCh     chan Peer
	Logger     log.Logger
	ListenAddr string
	Listener   net.Listener
}

func NewTCPNode(addr string, ch chan Peer) *TCPNode {
	logger := config.LoggerWithPrefixes("TCPNODE", "LISTEN-ADDRESS", addr)

	return &TCPNode{
		PeerCh:     ch,
		Logger:     logger,
		ListenAddr: addr,
	}
}

func (t *TCPNode) WithLogger(logger log.Logger) *TCPNode {
	t.Logger = logger

	return t
}

func (t *TCPNode) Listen() error {
	_ = t.Logger.Log("msg", "bootstrap TCP node", "listenAddr", t.ListenAddr)

	ln, err := net.Listen("tcp", t.ListenAddr)

	if err != nil {
		return err
	}

	t.Listener = ln

	go t.acceptLoop()

	return nil
}

func (t *TCPNode) acceptLoop() {
	_ = t.Logger.Log("msg", "accepting TCP connection on", "listenAddr", t.ListenAddr)

	for {
		conn, err := t.Listener.Accept()

		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				_ = t.Logger.Log("msg", "listener closed, terminating accept loop")
				return
			}

			_ = t.Logger.Log("error", "accept error", "msg", err)
			continue
		}

		peer := NewTCPPeer(conn)

		_ = t.Logger.Log("msg", "accept new peer connection", "from", peer.Who())

		t.PeerCh <- peer
	}
}

func (t *TCPNode) Connect(addr string) error {
	_ = t.Logger.Log("msg", "attempting to connect to seed", "to", addr)

	conn, err := net.Dial("tcp", addr)

	if err != nil {
		return err
	}

	peer := NewTCPPeer(conn)

	t.PeerCh <- peer

	return nil
}

func (t *TCPNode) Stop() error {
	_ = t.Logger.Log("msg", "shutting down TCP node")

	return t.Listener.Close()
}
