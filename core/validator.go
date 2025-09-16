package core

import (
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
)

type Validator interface {
	PrivateKey() crypto.PrivateKey
	PublicKey() crypto.PublicKey
	Address() types.Address
	Sign(data []byte) (crypto.Signature, error)
}

type BlockValidator struct {
	privKey crypto.PrivateKey
	pubKey  crypto.PublicKey
	address types.Address
}

func NewBlockValidator(privKey crypto.PrivateKey) *BlockValidator {
	return &BlockValidator{
		privKey: privKey,
		pubKey:  privKey.PublicKey(),
		address: privKey.PublicKey().Address(),
	}
}

func (v *BlockValidator) PrivateKey() crypto.PrivateKey {
	return v.privKey
}

func (v *BlockValidator) PublicKey() crypto.PublicKey {
	return v.pubKey
}

func (v *BlockValidator) Address() types.Address {
	return v.address
}

func (v *BlockValidator) Sign(data []byte) (crypto.Signature, error) {
	return v.privKey.Sign(data)
}
