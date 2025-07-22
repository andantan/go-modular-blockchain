package crypto

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestKeypair_Sign_Verify_Valid(t *testing.T) {
	privKey := GeneratePrivateKey()
	pubKey := privKey.PublicKey()

	msg := []byte("Hello World")

	sig, err := privKey.Sign(msg)

	assert.Nil(t, err)
	assert.True(t, sig.Verify(pubKey, msg))
}

func TestKeypair_Sign_Verify_Invalid(t *testing.T) {
	privKey := GeneratePrivateKey()

	msg := []byte("Hello World")

	sig, err := privKey.Sign(msg)

	assert.Nil(t, err)

	temperingPrivKey := GeneratePrivateKey()
	temperingPubKey := temperingPrivKey.PublicKey()

	assert.False(t, sig.Verify(temperingPubKey, msg))
}
