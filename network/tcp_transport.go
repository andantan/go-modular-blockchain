package network

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

type TCPPeer struct {
	conn     net.Conn
	Outgoing bool
}

func NewTCPPeer(conn net.Conn) *TCPPeer {
	return &TCPPeer{
		conn: conn,
	}
}

func (p *TCPPeer) Send(b []byte) error {
	_, err := p.conn.Write(b)

	return err
}

func (p *TCPPeer) readLoop(rpcCh chan RPC) {
	// 4K bytes
	buf := make([]byte, 1<<12)

	for {
		n, err := p.conn.Read(buf)

		if err == io.EOF {
			continue
		}

		if err != nil {
			fmt.Printf("read error: %s\n", err)
			continue
		}

		msg := buf[:n]
		rpcCh <- RPC{
			From:    p.conn.RemoteAddr(),
			Payload: bytes.NewBuffer(msg),
		}
	}
}

type TCPTransport struct {
	peerCh     chan *TCPPeer
	listenAddr string
	listener   net.Listener
}

func NewTCPTransport(addr string, peerCh chan *TCPPeer) *TCPTransport {
	return &TCPTransport{
		peerCh:     peerCh,
		listenAddr: addr,
	}
}

func (t *TCPTransport) Start() error {
	ln, err := net.Listen("tcp", t.listenAddr)

	if err != nil {
		return err
	}

	t.listener = ln

	go t.acceptLoop()

	fmt.Printf("TCP transport listening to %s\n", t.listenAddr)

	return nil
}

func (t *TCPTransport) readLoop(peer *TCPPeer) {
	// 2048 bytes
	buf := make([]byte, 1<<11)

	for {
		n, err := peer.conn.Read(buf)

		if err != nil {
			fmt.Printf("read error: %s\n", err)
			continue
		}

		msg := buf[:n]

		fmt.Println(string(msg))

		// TODO: handleMessage

	}
}

func (t *TCPTransport) acceptLoop() {
	for {
		conn, err := t.listener.Accept()

		if err != nil {
			fmt.Printf("accept error from %+v\n", conn)
			continue
		}

		peer := NewTCPPeer(conn)
		t.peerCh <- peer
	}
}
