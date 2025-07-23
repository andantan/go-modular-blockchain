package core

import (
	"crypto/sha256"
	"github.com/andantan/go-modular-blockchain/types"
)

type Hasher[T any] interface {
	Hash(T) types.Hash
}

type BlockHasher struct{}

func (BlockHasher) Hash(b *Header) types.Hash {
	h := sha256.Sum256(b.Bytes())

	return types.Hash(h[:])
}

type TxHasher struct{}

func (TxHasher) Hash(tx *Transaction) types.Hash {
	return sha256.Sum256(tx.Data)
}
