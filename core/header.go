package core

import (
	"github.com/andantan/go-modular-blockchain/types"
)

type Header struct {
	Version       uint32
	MerkleRoot    types.Hash
	PrevBlockHash types.Hash
	Timestamp     uint64
	Height        uint32
}
