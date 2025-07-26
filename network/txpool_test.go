package network

import (
	"fmt"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/random"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

func TestTxPool_Sort(t *testing.T) {
	p := NewTxPool()
	txLen := 1000

	for i := 0; i < txLen; i++ {
		tx := core.NewTransaction(random.GenerateRandomBytes(64))
		tx.SetFirstSeen(uint64(time.Now().UnixNano()))
		time.Sleep(time.Nanosecond)

		assert.Nil(t, p.Add(tx))
	}

	assert.Equal(t, p.Len(), txLen)

	txx := p.Transactions()

	for i := 0; i < txLen; i++ {
		fmt.Println(txx[i].FirstSeen())
	}

	for i := 0; i < len(txx)-1; i++ {
		assert.True(t, txx[i].FirstSeen() < txx[i+1].FirstSeen())
	}
}
