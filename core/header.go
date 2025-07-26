package core

import (
	"bytes"
	"encoding/gob"
	"github.com/andantan/go-modular-blockchain/types"
)

type Header struct {
	Version       uint32
	DataHash      types.Hash
	PrevBlockHash types.Hash
	Timestamp     uint64
	Height        uint32
}

func (h *Header) Bytes() []byte {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)

	_ = enc.Encode(h)

	return buf.Bytes()
}
