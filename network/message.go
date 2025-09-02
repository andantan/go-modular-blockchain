package network

import (
	"bytes"
	"encoding/gob"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/types"
	"io"
	"net"
)

type MessageType byte

const (
	MessageTypeNewTx     MessageType = 0x01
	MessageTypeNewBlock  MessageType = 0x02
	MessageTypeReqStatus MessageType = 0x03
	MessageTypeResStatus MessageType = 0x04
	MessageTypeReqBlock  MessageType = 0x05
	MessageTypeResBlock  MessageType = 0x06
)

type RawMessage struct {
	From    net.Addr
	Payload io.Reader
}

type Message struct {
	Type MessageType
	Data []byte
}

func NewMessage(t MessageType, data []byte) *Message {
	return &Message{
		Type: t,
		Data: data,
	}
}

func (m *Message) Bytes() []byte {
	buf := &bytes.Buffer{}
	_ = gob.NewEncoder(buf).Encode(m)

	return buf.Bytes()
}

type DecodedMessage struct {
	From net.Addr
	Data any
}

type RawMessageDecodeFunc func(RawMessage) (*DecodedMessage, error)
type MessageProcessFunc func(*DecodedMessage) error

type RequestStatus struct{}

type ResponseStatus struct {
	ID               string
	Version          uint16
	Height           uint64
	Status           BlockchainStatusType
	GenesisBlockHash types.Hash
	CurrentBlockHash types.Hash
}

type RequestBlocks struct {
	From      uint64
	To        uint64
	BatchSize uint64
}

type ResponseBlocks struct {
	Blocks []*core.Block
}
