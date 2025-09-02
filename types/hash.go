package types

import (
	"encoding/hex"
	"fmt"
)

const (
	HashLength = 32
)

type Hash [HashLength]uint8

func (h Hash) Bytes() []byte {
	b := make([]byte, HashLength)

	copy(b[:], h[:])

	return b
}

func (h Hash) IsZero() bool {
	return h == Hash{}
}

func (h Hash) String() string {
	return hex.EncodeToString(h.Bytes())
}

func (h Hash) ShortString(n int) string {
	fullHash := h.String()

	if n > len(fullHash) {
		return fullHash
	}

	return fullHash[:n]
}

func HashFromBytes(b []byte) (Hash, error) {
	if len(b) != HashLength {
		return Hash{}, fmt.Errorf("given bytes with hash-length %d should be 32 bytes", len(b))
	}

	var h Hash

	copy(h[:], b[:])

	return h, nil
}
