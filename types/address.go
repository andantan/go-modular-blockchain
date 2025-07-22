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

func (a Address) ToString() string {
	return hex.EncodeToString(a.ToSlice())
}

func MustAddressFromBytes(b []byte) Address {
	if len(b) != AddressLength {
		msg := fmt.Sprintf("given bytes with address-length %d should be 20 bytes", len(b))
		panic(msg)
	}

	var h Address

	copy(h[:], b[:])

	return h
}
