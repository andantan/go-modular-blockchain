package types

import (
	"encoding/hex"
	"fmt"
)

const (
	AddressLength = 20
)

type Address [AddressLength]uint8

func (a Address) ToSlice() []byte {
	b := make([]byte, AddressLength)

	copy(b[:], a[:])

	return b
}

func (a Address) String() string {
	return hex.EncodeToString(a.ToSlice())
}

func (a Address) ShortString(n int) string {
	fullHash := a.String()

	if n > len(fullHash) {
		return fullHash
	}

	return fullHash[:n]
}

func AddressFromBytes(b []byte) (Address, error) {
	if len(b) != AddressLength {
		return Address{}, fmt.Errorf("given bytes with address-length %d should be 20 bytes", len(b))
	}

	var h Address

	copy(h[:], b[:])

	return h, nil
}
