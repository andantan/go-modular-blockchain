package types

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const (
	HashLength = 32
)

type Hash [32]uint8

func (h Hash) IsZero() bool {
	return h == Hash{}
}

func (h Hash) ToSlice() []byte {
	b := make([]byte, HashLength)

	copy(b[:], h[:])

	return b
}

func (h Hash) String() string {
	return hex.EncodeToString(h.ToSlice())
}

func HashFromBytes(b []byte) Hash {
	if len(b) != HashLength {
		msg := fmt.Sprintf("given bytes with length %d should be 32 bytes", len(b))
		panic(msg)
	}

	var h Hash

	copy(h[:], b[:])

	return h
}

func RandomBytes(size int) []byte {
	token := make([]byte, size)
	_, _ = rand.Read(token)

	return token
}

func RandomHash() Hash {
	return HashFromBytes(RandomBytes(HashLength))
}
