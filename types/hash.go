package types

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	HashLength = 32
)

type Hash [HashLength]uint8

func (h Hash) ToSlice() []byte {
	b := make([]byte, HashLength)

	copy(b[:], h[:])

	return b
}

func (h Hash) IsZero() bool {
	return h == Hash{}
}

func (h Hash) String() string {
	return "0x" + hex.EncodeToString(h.ToSlice())
}

func (h Hash) ShortString(length int) string {
	hashStr := hex.EncodeToString(h.ToSlice())

	if length > len(hashStr) {
		length = len(hashStr)
	}

	return "0x" + hashStr[:length]
}

func HashFromBytes(b []byte) (Hash, error) {
	if len(b) != HashLength {
		return Hash{}, fmt.Errorf("given bytes with hash-length %d should be 32 bytes", len(b))
	}

	var h Hash

	copy(h[:], b[:])

	return h, nil
}

func HashFromHexString(s string) (Hash, error) {
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}

	if len(s) != HashLength*2 {
		return Hash{}, fmt.Errorf("invalid hex string length (%d), must be %d", len(s), HashLength*2)
	}

	hashBytes, err := hex.DecodeString(s)
	if err != nil {
		return Hash{}, err
	}

	var h Hash
	copy(h[:], hashBytes)

	return h, nil
}
