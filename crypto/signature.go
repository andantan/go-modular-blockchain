package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"math/big"
)

// Signature By ECDSA
type Signature struct {
	R *big.Int
	S *big.Int
}

func (sig Signature) IsNil() bool {
	return sig.R == nil || sig.S == nil
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

func (sig Signature) Bytes() []byte {
	rBytes := make([]byte, 32)
	sBytes := make([]byte, 32)

	sig.R.FillBytes(rBytes)
	sig.S.FillBytes(sBytes)

	return append(rBytes, sBytes...)
}

func (sig Signature) String() string {
	b := append(sig.R.Bytes(), sig.S.Bytes()...)

	return hex.EncodeToString(b)
}
