package network

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConnect(t *testing.T) {
	tra := NewLocalTransport("A")
	trb := NewLocalTransport("B")

	assert.Nil(t, tra.Connect(trb))
	assert.Nil(t, trb.Connect(tra))

	assert.Equal(t, tra.peers[trb.Addr()], trb)
	assert.Equal(t, trb.peers[tra.Addr()], tra)
}

func TestSendMessage(t *testing.T) {
	tra := NewLocalTransport("A")
	trb := NewLocalTransport("B")

	err := tra.Connect(trb)
	assert.Nil(t, err)

	err = trb.Connect(tra)
	assert.Nil(t, err)

	msg := []byte("hello world")

	assert.Nil(t, tra.SendMessage(trb.Addr(), msg))

	rpc := <-trb.consumeCh

	assert.Equal(t, tra.Addr(), rpc.From)
	assert.Equal(t, msg, rpc.Payload)
}
