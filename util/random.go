package util

import (
	"crypto/rand"
	"github.com/andantan/go-modular-blockchain/types"
)

func GenerateRandomBytes(size int) []byte {
	token := make([]byte, size)
	_, _ = rand.Read(token)

	return token
}

func RandomHash() types.Hash {
	return types.MustHashFromBytes(GenerateRandomBytes(32))
}
