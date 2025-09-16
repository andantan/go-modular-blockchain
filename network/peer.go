package network

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
	"io"
	"net"
	"sync"
	"time"
)

//type PeerIdentity struct {
//	ID            string
//	Addr          string
//	ConnectedTime int64
//}

type PeerIdentity struct {
	PublicKey     crypto.PublicKey
	Address       types.Address
	NetAddr       string
	Domain        string
	ConnectedTime int64
	IsValidator   bool
	Signature     crypto.Signature
}

func (pi *PeerIdentity) Hash() (types.Hash, error) {
	buf := new(bytes.Buffer)
	buf.Write(pi.PublicKey.Key)
	buf.Write(pi.Address.Bytes())
	buf.Write([]byte(pi.NetAddr))
	buf.Write([]byte(pi.Domain))
	hash := sha256.Sum256(buf.Bytes())

	return hash, nil
}

func (pi *PeerIdentity) Sign(privKey crypto.PrivateKey) error {
	hash, err := pi.Hash()

	if err != nil {
		return err
	}

	sig, err := privKey.Sign(hash.ToSlice())

	if err != nil {
		return err
	}

	pi.Signature = sig
	return nil
}

func (pi *PeerIdentity) Verify() error {
	hash, err := pi.Hash()

	if err != nil {
		return err
	}

	if pi.Signature.IsNil() {
		return fmt.Errorf("peerIdentity has no signature")
	}

	if !pi.Signature.Verify(pi.PublicKey, hash.ToSlice()) {
		return fmt.Errorf("invalid peerIdentity signature")
	}

	return nil
}

type Peer interface {
	Identity() PeerIdentity
	Send([]byte) error
	Read()
	Close()
}

type TCPPeer struct {
	PeerIdentity
	logger log.Logger

	conn      net.Conn
	messageCh chan RawMessage

	closeCh     chan struct{}
	terminateCh chan Peer
	closeOnce   sync.Once
}

func NewTCPPeer(conn net.Conn, MsgCh chan RawMessage, terminateCh chan Peer) *TCPPeer {
	return &TCPPeer{
		logger:      config.LoggerWithPrefixes("peer", "from", conn.RemoteAddr().String()),
		conn:        conn,
		messageCh:   MsgCh,
		terminateCh: terminateCh,
		closeCh:     make(chan struct{}),
	}
}

func (p *TCPPeer) Identity() PeerIdentity {
	return p.PeerIdentity
}

func (p *TCPPeer) Send(payload []byte) error {
	lenBuf := make([]byte, 4)
	payloadSize := uint32(len(payload))
	binary.BigEndian.PutUint32(lenBuf, payloadSize)

	if _, err := p.conn.Write(append(lenBuf, payload...)); IsUnrecoverableTCPError(err) {
		p.Close()
		return err
	}

	return nil
}

func (p *TCPPeer) Read() {
	defer p.Close()

	msgCh := make(chan []byte)
	defer close(msgCh)
	errCh := make(chan error, 1)
	defer close(errCh)

	ctx, cancel := context.WithCancel(context.Background())

	wg := new(sync.WaitGroup)
	wg.Add(1)

	defer func() {
		cancel()
		wg.Wait()
	}()

	go p.readMessages(ctx, wg, msgCh, errCh)

	for {
		select {
		case <-p.closeCh:
			return

		case <-errCh:
			return

		case msgBuf := <-msgCh:
			p.messageCh <- RawMessage{
				From:    p.Address,
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
			panic("CTX DONE HERE!")
			return
		default:
		}

		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(p.conn, lenBuf); IsUnrecoverableTCPError(err) {
			errCh <- err
			return
		}

		msgLen := binary.BigEndian.Uint32(lenBuf)
		if msgLen == 0 {
			// tcp heartbeat
			continue
		}

		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(p.conn, msgBuf); IsUnrecoverableTCPError(err) {
			errCh <- err
			return
		}

		msgCh <- msgBuf
	}
}

func (p *TCPPeer) Close() {
	p.closeOnce.Do(func() {
		_ = p.logger.Log("msg", "closing connection", "address", p.Address.ShortString(8), "net-addr", p.NetAddr)

		close(p.closeCh)
		_ = p.conn.Close()
		if p.terminateCh != nil {
			p.terminateCh <- p
		}
	})
}

func (p *TCPPeer) handshake(ourIdentity *PeerIdentity) (*PeerIdentity, error) {
	identityCh := make(chan PeerIdentity, 1)
	defer close(identityCh)
	errCh := make(chan error, 1)
	defer close(errCh)

	go p.readHandshakeMessage(identityCh, errCh)

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(ourIdentity); err != nil {
		_ = p.conn.Close()
		return nil, err
	}
	payload := buf.Bytes()
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(buf.Len()))

	if _, err := p.conn.Write(append(lenBuf, payload...)); err != nil {
		_ = p.conn.Close()
		return nil, err
	}

	select {
	case err := <-errCh:
		_ = p.conn.Close()
		return nil, err
	case remoteIdentity := <-identityCh:
		p.PeerIdentity = remoteIdentity
		return &remoteIdentity, nil
	case <-time.After(5 * time.Second):
		_ = p.conn.Close()
		return nil, fmt.Errorf("handshake timeout")
	}
}

func (p *TCPPeer) readHandshakeMessage(identityCh chan<- PeerIdentity, errCh chan<- error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(p.conn, lenBuf); IsUnrecoverableTCPError(err) {
		errCh <- err
		return
	}

	payloadLen := binary.BigEndian.Uint32(lenBuf)

	// sanity check threshold = 1KB
	if payloadLen > 1<<10 {
		errCh <- fmt.Errorf("handshake payload too large: %d bytes", payloadLen)
		return
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(p.conn, payload); IsUnrecoverableTCPError(err) {
		errCh <- err
		return
	}

	var remoteIdentity PeerIdentity
	if err := gob.NewDecoder(bytes.NewBuffer(payload)).Decode(&remoteIdentity); err != nil {
		errCh <- err
		return
	}

	if err := remoteIdentity.Verify(); err != nil {
		errCh <- err
		return
	}

	identityCh <- remoteIdentity
}
