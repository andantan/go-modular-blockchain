package crypto

import (
	"crypto/sha256"
	"github.com/andantan/go-modular-blockchain/types"
)

type PublicKey struct {
	Key []byte
}

func (pk PublicKey) Address() types.Address {
	h := sha256.Sum256(pk.Key)
	start := len(h) - types.AddressLength

	return types.MustAddressFromBytes(h[start:])
}
