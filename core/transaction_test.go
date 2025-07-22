package core

import (
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTransaction_Sign(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	tx := Transaction{
		Data: []byte("foo"),
	}

	assert.Nil(t, tx.Sign(privKey))
	assert.NotNil(t, tx.Signature)
}

func TestTransaction_Verify_Valid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	tx := Transaction{
		Data: []byte("foo"),
	}

	assert.Nil(t, tx.Sign(privKey))
	assert.Nil(t, tx.Verify())
}

func TestTransaction_Verify_InValid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	tx := Transaction{
		Data: []byte("foo"),
	}

	assert.Nil(t, tx.Sign(privKey))

	temperingPrivKey := crypto.GeneratePrivateKey()
	tx.PublicKey = temperingPrivKey.PublicKey()

	assert.NotNil(t, tx.Verify())
}
