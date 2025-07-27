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

	b := randomBlock(t, 0, types.Hash{})

	assert.Nil(t, b.Sign(privKey))
	assert.NotNil(t, b.Signature)
}

func TestBlock_Verify_Valid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(t, 0, types.Hash{})

	assert.Nil(t, b.Verify())
	assert.Nil(t, b.Sign(privKey))
	assert.Nil(t, b.Verify())
}

func TestBlock_Verify_Invalid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(t, 0, types.Hash{})
	assert.Nil(t, b.Sign(privKey))

	temperingPrivkey := crypto.GeneratePrivateKey()
	b.Validator = temperingPrivkey.PublicKey()

	assert.NotNil(t, b.Verify())

	b.Height = 100
	assert.NotNil(t, b.Verify())
}

func randomBlock(t *testing.T, height uint32, prevBlockHash types.Hash) *Block {
	privKey := crypto.GeneratePrivateKey()
	tx := randomTxWithSignature(t)

	header := &Header{
		Version:       1,
		PrevBlockHash: prevBlockHash,
		Height:        height,
		Timestamp:     uint64(time.Now().UnixNano()),
	}

	b := NewBlock(header, []*Transaction{tx})
	dataHash, err := CalculateDataHash(b.Transactions)
	assert.Nil(t, err)
	b.DataHash = dataHash
	assert.Nil(t, b.Sign(privKey))

	return b
}
