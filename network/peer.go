package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/go-kit/log"
	"io"
	"net"
	"sync"
	"time"
)

type Peer interface {
	Addr() string
	ID() string
	Send([]byte) error
	Read()
	Close()
}

type TCPPeer struct {
	Logger      log.Logger
	Id          string
	Conn        net.Conn
	MessageCh   chan RawMessage
	TerminateCh chan Peer
	CloseCh     chan struct{}
}

func NewTCPPeer(conn net.Conn, MsgCh chan RawMessage, terminateCh chan Peer) *TCPPeer {
	return &TCPPeer{
		Logger:      config.LoggerWithPrefixes("peer", "from", conn.RemoteAddr().String()),
		Conn:        conn,
		MessageCh:   MsgCh,
		TerminateCh: terminateCh,
		CloseCh:     make(chan struct{}),
	}
}

func (p *TCPPeer) WithLogger(logger log.Logger) *TCPPeer {
	p.Logger = logger

	return p
}

func (p *TCPPeer) Addr() string {
	return p.Conn.RemoteAddr().String()
}

func (p *TCPPeer) ID() string {
	return p.Id
}

func (p *TCPPeer) Send(payload []byte) error {
	lenBuf := make([]byte, 4)
	payloadSize := uint32(len(payload))
	binary.BigEndian.PutUint32(lenBuf, payloadSize)

	if _, err := p.Conn.Write(lenBuf); err != nil {
		return err
	}

	if _, err := p.Conn.Write(payload); err != nil {
		return err
	}

	return nil
}

func (p *TCPPeer) Read() {
	defer func() {
		if p.TerminateCh != nil {
			p.TerminateCh <- p
		}
		_ = p.Conn.Close()
		_ = p.Logger.Log("event", "connection_closed_and_terminated", "peer-id", p.ID(), "peer-addr", p.Addr())
	}()

	msgCh := make(chan []byte)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go p.readMessages(ctx, wg, msgCh, errCh)

	_ = p.Logger.Log("msg", "new peer connected", "peer-id", p.ID(), "peer-addr", p.Addr())

	for {
		select {
		case <-p.CloseCh:
			cancel()
			wg.Wait()
			close(msgCh)
			close(errCh)
			return

		case <-errCh:
			cancel()
			wg.Wait()
			close(msgCh)
			close(errCh)
			return

		case msgBuf := <-msgCh:
			p.MessageCh <- RawMessage{
				From:    p.Conn.RemoteAddr(),
				Payload: bytes.NewBuffer(msgBuf),
			}
		}
	}
}

func (p *TCPPeer) readMessages(ctx context.Context, wg *sync.WaitGroup, msgCh chan<- []byte, errCh chan<- error) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(p.Conn, lenBuf); IsUnrecoverableTCPError(err) {
			errCh <- err
			return
		}

		msgLen := binary.BigEndian.Uint32(lenBuf)
		if msgLen == 0 {
			// tcp heartbeat
			continue
		}

		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(p.Conn, msgBuf); IsUnrecoverableTCPError(err) {
			errCh <- err
			return
		}

		msgCh <- msgBuf
	}
}

func (p *TCPPeer) Close() {
	select {
	case <-p.CloseCh:
		return
	default:
		close(p.CloseCh)
	}
}

func (p *TCPPeer) handshake(ourId string) (remoteID string, err error) {
	errCh := make(chan error, 1)
	idCh := make(chan string, 1)

	go func() {
		lenBuf := make([]byte, 4)
		if _, err = io.ReadFull(p.Conn, lenBuf); err != nil {
			errCh <- err
			return
		}

		idLen := binary.BigEndian.Uint32(lenBuf)
		idBuf := make([]byte, idLen)

		if _, err = io.ReadFull(p.Conn, idBuf); err != nil {
			errCh <- err
			return
		}

		idCh <- string(idBuf)
	}()

	idBytes := []byte(ourId)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(idBytes)))
	if _, err = p.Conn.Write(append(lenBuf, idBytes...)); err != nil {
		_ = p.Conn.Close()
		return "", err
	}

	select {
	case err = <-errCh:
		_ = p.Conn.Close()
		return "", err
	case remoteID = <-idCh:
		p.Id = remoteID

		return remoteID, nil
	case <-time.After(5 * time.Second):
		_ = p.Conn.Close()
		return "", fmt.Errorf("handshake timeout")
	}
}
