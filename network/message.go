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
	MessageTypeNewTx MessageType = iota
	MessageTypeNewBlock
	MessageTypeReqStatus
	MessageTypeResStatus
	MessageTypeReqHeaders
	MessageTypeResHeaders
	MessageTypeReqBlocks
	MessageTypeResBlocks
)

func (mt MessageType) String() string {
	switch mt {
	case MessageTypeNewTx:
		return "NewTx"
	case MessageTypeNewBlock:
		return "NewBlock"
	case MessageTypeReqStatus:
		return "ReqStatus"
	case MessageTypeResStatus:
		return "ResStatus"
	case MessageTypeReqHeaders:
		return "ReqHeaders"
	case MessageTypeResHeaders:
		return "ResHeaders"
	case MessageTypeReqBlocks:
		return "ReqBlocks"
	case MessageTypeResBlocks:
		return "ResBlocks"
	default:
		return "Unknown"
	}
}

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

type RequestHeaders struct {
	From  uint64
	Count uint64
}

type ResponseHeaders struct {
	Headers []*core.Header
}

type RequestBlocks struct {
	From  uint64
	Count uint64
}

type ResponseBlocks struct {
	Blocks []*core.Block
}
