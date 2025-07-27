package network

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/core"
	"io"
)

type MessageType byte

const (
	MessageTypeTx MessageType = 1 << iota
	MessageTypeBlock
)

type RPC struct {
	From    NetAddr
	Payload io.Reader
}

type Message struct {
	Header MessageType
	Data   []byte
}

func NewMessage(t MessageType, data []byte) *Message {
	return &Message{
		Header: t,
		Data:   data,
	}
}

func (msg *Message) Bytes() []byte {
	buf := &bytes.Buffer{}
	_ = gob.NewEncoder(buf).Encode(msg)

	return buf.Bytes()
}

type DecodedMessage struct {
	From NetAddr
	Data any
}

type RPCDecodeFunc func(RPC) (*DecodedMessage, error)

func DefaultRPCDecodeFunc(rpc RPC) (*DecodedMessage, error) {
	// logger := config.GetNetworkLogger()
	msg := Message{}

	if err := gob.NewDecoder(rpc.Payload).Decode(&msg); err != nil {
		return nil, fmt.Errorf("failed to decode message from %s: %s", rpc.From, err)
	}

	//logger.WithFields(logrus.Fields{
	//	"type": msg.Header,
	//	"from": rpc.From,
	//}).Info("new incoming message")

	switch msg.Header {
	case MessageTypeTx:
		tx := new(core.Transaction)

		if err := tx.Decode(core.NewGobTxDecoder(bytes.NewReader(msg.Data))); err != nil {
			return nil, err
		}

		return &DecodedMessage{
			From: rpc.From,
			Data: tx,
		}, nil
	case MessageTypeBlock:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown message header: %x", msg.Header)
	}
}

type RPCProcesor interface {
	ProcessMessage(message *DecodedMessage) error
}
