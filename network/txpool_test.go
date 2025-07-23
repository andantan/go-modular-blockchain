package network

import (
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTxPool(t *testing.T) {
	p := NewTxPool()

	assert.Equal(t, 0, p.Len())
}

func TestTxPool_Add(t *testing.T) {
	p := NewTxPool()
	tx := core.NewTransaction([]byte("foo"))

	assert.Nil(t, p.Add(tx))
	assert.Equal(t, p.Len(), 1)

	txp := core.NewTransaction([]byte("foo"))
	assert.Nil(t, p.Add(txp))
	assert.Equal(t, p.Len(), 1)

	p.Flush()

	assert.Equal(t, p.Len(), 0)
}
