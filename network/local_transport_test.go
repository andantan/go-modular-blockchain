package network

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSendMessage(t *testing.T) {
	tra := NewLocalTransport("A")
	trb := NewLocalTransport("B")

	err := tra.Connect(trb)
	assert.Nil(t, err)

	err = trb.Connect(tra)
	assert.Nil(t, err)

	msg := []byte("hello world")

	assert.Nil(t, tra.SendMessage(trb.Addr(), msg))

	rpc := <-trb.Consume()

	assert.Equal(t, tra.Addr(), rpc.From)
	assert.Equal(t, msg, rpc.Payload)
}
