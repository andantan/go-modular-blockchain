package network

import (
	"bytes"
	"encoding/binary"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/go-kit/log"
	"io"
	"net"
)

type Peer interface {
	Send([]byte) error
	Read(chan RawMessage) error
	Close() error
	Who() net.Addr
}

type TCPPeer struct {
	Conn   net.Conn
	Logger log.Logger
}

func NewTCPPeer(conn net.Conn) *TCPPeer {
	logger := config.LoggerWithPrefixes("TCPPEER", "PEER-ADDRESS", conn.RemoteAddr().String())

	return &TCPPeer{
		Conn:   conn,
		Logger: logger,
	}
}

func (p *TCPPeer) WithLogger(logger log.Logger) *TCPPeer {
	p.Logger = logger

	return p
}

func (p *TCPPeer) Send(payload []byte) error {
	lenBuf := make([]byte, 4)
	payloadSize := uint32(len(payload))
	binary.BigEndian.PutUint32(lenBuf, payloadSize)

	if _, err := p.Conn.Write(lenBuf); err != nil {
		return err
	}

	_, err := p.Conn.Write(payload)

	return err
}

func (p *TCPPeer) Read(ch chan RawMessage) error {
	defer func() {
		_ = p.Close()
		_ = p.Logger.Log("msg", "terminating read loop", "from", p.Who())
	}()

	_ = p.Logger.Log("msg", "entering read loop mode", "from", p.Who())

	for {
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(p.Conn, lenBuf); err != nil {
			return err
		}

		msgLen := binary.BigEndian.Uint32(lenBuf)

		if msgLen == 0 {
			continue
		}

		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(p.Conn, msgBuf); err != nil {
			return err
		}

		ch <- RawMessage{
			From:    p.Conn.RemoteAddr(),
			Payload: bytes.NewBuffer(msgBuf),
		}
	}
}

func (p *TCPPeer) Close() error {
	return p.Conn.Close()
}

func (p *TCPPeer) Who() net.Addr {
	return p.Conn.RemoteAddr()
}
