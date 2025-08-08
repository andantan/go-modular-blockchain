package core

import (
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/andantan/go-modular-blockchain/util"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// NewRandomTransaction return a new random transaction whithout signature.
func NewRandomTransaction(size int) *Transaction {
	return NewTransaction(util.GenerateRandomBytes(size))
}

func NewRandomTransactionWithSignature(t *testing.T, privKey crypto.PrivateKey, size int) *Transaction {
	tx := NewRandomTransaction(size)
	assert.Nil(t, tx.Sign(privKey))
	return tx
}

func NewRandomBlock(t *testing.T, height uint32, prevBlockHash types.Hash) *Block {
	txSigner := crypto.GeneratePrivateKey()
	tx := NewRandomTransactionWithSignature(t, txSigner, 100)
	header := &Header{
		Version:       1,
		PrevBlockHash: prevBlockHash,
		Height:        height,
		Timestamp:     uint64(time.Now().UnixNano()),
	}
	b := NewBlock(header, []*Transaction{tx})
	dataHash, err := CalculateDataHash(b.Transactions)
	assert.Nil(t, err)
	b.Header.DataHash = dataHash

	return b
}

func NewRandomBlockWithSignature(t *testing.T, pk crypto.PrivateKey, height uint32, prevHash types.Hash) *Block {
	b := NewRandomBlock(t, height, prevHash)
	assert.Nil(t, b.Sign(pk))

	return b
}
