package core

import (
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/random"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func randomBlock(height uint32) *Block {
	header := &Header{
		Version:       1,
		PrevBlockHash: types.RandomHash(),
		Height:        height,
		Timestamp:     uint64(time.Now().UnixNano()),
	}

	txx := make([]Transaction, 100)

	for i := 0; i < 100; i++ {
		txx = append(txx, Transaction{
			Data: random.GenerateRandomBytes(128),
		})
	}

	return NewBlock(header, txx)
}

func TestBlock_Sign(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(0)

	assert.Nil(t, b.Sign(privKey))
	assert.NotNil(t, b.Signature)
}

func TestBlock_Verify_Valid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(0)

	assert.NotNil(t, b.Verify())
	assert.Nil(t, b.Sign(privKey))
	assert.Nil(t, b.Verify())
}

func TestBlock_Verify_Invalid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := randomBlock(0)
	assert.Nil(t, b.Sign(privKey))

	temperingPrivkey := crypto.GeneratePrivateKey()
	b.Validator = temperingPrivkey.PublicKey()

	assert.NotNil(t, b.Verify())

	b.Height = 100
	assert.NotNil(t, b.Verify())
}
