package types

import (
	"encoding/hex"
	"fmt"
	"github.com/andantan/go-modular-blockchain/random"
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

func MustHashFromBytes(b []byte) Hash {
	if len(b) != HashLength {
		msg := fmt.Sprintf("given bytes with hash-length %d should be 32 bytes", len(b))
		panic(msg)
	}

	var h Hash

	copy(h[:], b[:])

	return h
}

func RandomHash() Hash {
	return MustHashFromBytes(random.GenerateRandomBytes(HashLength))
}
