package core

import (
	"bytes"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/util"
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
	tx.From = temperingPrivKey.PublicKey()

	assert.NotNil(t, tx.Verify())
}

func TestTransaction_Encode_Decode(t *testing.T) {
	tx := randomTxWithSignature(t)
	buf := &bytes.Buffer{}

	assert.Nil(t, tx.Encode(NewGobTxEncoder(buf)))

	txDecoded := new(Transaction)

	assert.Nil(t, txDecoded.Decode(NewGobTxDecoder(buf)))
	assert.Equal(t, tx, txDecoded)
}

func randomTxWithSignature(t *testing.T) *Transaction {
	privKey := crypto.GeneratePrivateKey()

	tx := &Transaction{
		Data: util.GenerateRandomBytes(128),
	}

	assert.Nil(t, tx.Sign(privKey))
	assert.NotNil(t, tx.From)
	assert.NotNil(t, tx.Signature)

	return tx
}
