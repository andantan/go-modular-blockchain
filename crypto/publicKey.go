package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"github.com/andantan/go-modular-blockchain/types"
)

type PublicKey struct {
	key *ecdsa.PublicKey
}

func (pk PublicKey) ToSlice() []byte {
	return elliptic.MarshalCompressed(pk.key.Curve, pk.key.X, pk.key.Y)
}

func (pk PublicKey) Address() types.Address {
	h := sha256.Sum256(pk.ToSlice())
	start := len(h) - types.AddressLength

	return types.MustAddressFromBytes(h[start:])
}
