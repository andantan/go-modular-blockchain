package network

import (
	"github.com/stretchr/testify/assert"
	"io"
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
	b, err := io.ReadAll(rpc.Payload)

	assert.Nil(t, err)
	assert.Equal(t, rpc.From, tra.Addr())
	assert.Equal(t, msg, b)
}

func TestLocalTransport_Broadcast(t *testing.T) {
	tra := NewLocalTransport("A")
	trb := NewLocalTransport("B")
	trc := NewLocalTransport("C")

	_ = tra.Connect(trb)
	_ = tra.Connect(trc)

	msg := []byte("foo bar baz")

	assert.Nil(t, tra.Broadcast(msg))

	rpcb := <-trb.Consume()
	b, err := io.ReadAll(rpcb.Payload)
	assert.Nil(t, err)
	assert.Equal(t, b, msg)

	rpcc := <-trc.Consume()
	c, err := io.ReadAll(rpcc.Payload)
	assert.Nil(t, err)
	assert.Equal(t, c, msg)
}
