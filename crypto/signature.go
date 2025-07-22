package crypto

import (
	"crypto/ecdsa"
	"math/big"
)

// Signature Method ECDSA
type Signature struct {
	r *big.Int
	s *big.Int
}

func (sig Signature) Verify(pubKey PublicKey, data []byte) bool {
	return ecdsa.Verify(pubKey.key, data, sig.r, sig.s)
}
