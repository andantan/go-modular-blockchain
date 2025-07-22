package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

type PrivateKey struct {
	key *ecdsa.PrivateKey
}

func GeneratePrivateKey() PrivateKey {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		panic(err)
	}

	return PrivateKey{key: k}
}

func (pk PrivateKey) PublicKey() PublicKey {
	return PublicKey{
		key: &pk.key.PublicKey,
	}
}

func (pk PrivateKey) Sign(data []byte) (*Signature, error) {
	r, s, err := ecdsa.Sign(rand.Reader, pk.key, data)

	if err != nil {
		return nil, err
	}

	return &Signature{
		r: r,
		s: s,
	}, nil
}
