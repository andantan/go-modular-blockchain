package types

import (
	"encoding/hex"
	"fmt"
	"strings"
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

func (a Address) Bytes() []byte {
	b := make([]byte, AddressLength)

	copy(b[:], a[:])

	return b
}

func (a Address) String() string {
	return "0x" + hex.EncodeToString(a.ToSlice())
}

func (a Address) ShortString(length int) string {
	addressStr := hex.EncodeToString(a.ToSlice())

	if length > len(addressStr) {
		length = len(addressStr)
	}

	return "0x" + addressStr[:length]
}

func AddressFromBytes(b []byte) (Address, error) {
	if len(b) != AddressLength {
		return Address{}, fmt.Errorf("given bytes with address-length %d should be 20 bytes", len(b))
	}

	var h Address

	copy(h[:], b[:])

	return h, nil
}

func AddressFromHexString(s string) (Address, error) {
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}

	if len(s) != AddressLength*2 {
		return Address{}, fmt.Errorf("invalid hex string length (%d), must be %d", len(s), AddressLength*2)
	}

	addrBytes, err := hex.DecodeString(s)
	if err != nil {
		return Address{}, err
	}

	var addr Address
	copy(addr[:], addrBytes)

	return addr, nil
}
