package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
)

// Signature Method ECDSA
type Signature struct {
	R *big.Int
	S *big.Int
}

func (sig Signature) Verify(pk PublicKey, data []byte) bool {
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), pk.Key)

	key := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	return ecdsa.Verify(key, data, sig.R, sig.S)
}
