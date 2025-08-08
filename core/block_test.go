package core

import (
	"bytes"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBlock_Sign(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := NewRandomBlock(t, 0, types.Hash{})

	assert.Nil(t, b.Sign(privKey))
	assert.NotNil(t, b.Signature)
}

func TestBlock_Verify_Valid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := NewRandomBlockWithSignature(t, privKey, 0, types.Hash{})

	assert.Nil(t, b.Verify())
}

func TestBlock_Verify_Invalid(t *testing.T) {
	privKey := crypto.GeneratePrivateKey()

	b := NewRandomBlock(t, 0, types.Hash{})
	assert.Nil(t, b.Sign(privKey))

	temperingPrivkey := crypto.GeneratePrivateKey()
	b.Validator = temperingPrivkey.PublicKey()

	assert.NotNil(t, b.Verify())

	b.Height = 100
	assert.NotNil(t, b.Verify())
}

func TestBlock_Decode_Encode(t *testing.T) {
	b := NewRandomBlock(t, 1, types.Hash{})
	buf := &bytes.Buffer{}

	assert.Nil(t, b.Encode(NewGobBlockEncoder(buf)))

	bDecode := new(Block)
	assert.Nil(t, bDecode.Decode(NewGobBlockDecoder(buf)))
	assert.Equal(t, b, bDecode)
}
