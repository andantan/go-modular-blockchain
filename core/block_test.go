package core

import (
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBlock_Sign(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(0, types.Hash{})

	assert.Nil(t, b.Sign(privKey))
	assert.NotNil(t, b.Signature)
}

func TestBlock_Verify_Valid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(0, types.Hash{})

	assert.NotNil(t, b.Verify())
	assert.Nil(t, b.Sign(privKey))
	assert.Nil(t, b.Verify())
}

func TestBlock_Verify_Invalid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(0, types.Hash{})
	assert.Nil(t, b.Sign(privKey))

	temperingPrivkey := crypto.GeneratePrivateKey()
	b.Validator = temperingPrivkey.PublicKey()

	assert.NotNil(t, b.Verify())

	b.Height = 100
	assert.NotNil(t, b.Verify())
}

func randomBlock(height uint32, prevBlockHash types.Hash) *Block {
	header := &Header{
		Version:       1,
		PrevBlockHash: prevBlockHash,
		Height:        height,
		Timestamp:     uint64(time.Now().UnixNano()),
	}

	return NewBlock(header, []Transaction{})
}

func randomBlockWithSignature(t *testing.T, height uint32, prevBlockHash types.Hash) *Block {
	privKey := crypto.GeneratePrivateKey()
	b := randomBlock(height, prevBlockHash)

	clocks := 100
	for range clocks {
		tx := randomTxWithSignature(t)
		b.AddTransaction(tx)
	}

	assert.Nil(t, b.Sign(privKey))

	return b
}
