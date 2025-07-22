package core

import (
	"bytes"
	"crypto/sha256"
	"github.com/andantan/go-modular-blockchain/types"
	"io"
)

type Block struct {
	Header
	Transactions []Transaction

	hash types.Hash
}

func (b *Block) Hash() types.Hash {
	buf := &bytes.Buffer{}

	_ = b.Header.EncodeBinary(buf)

	if b.hash.IsZero() {
		b.hash = sha256.Sum256(buf.Bytes())
	}

	return b.hash
}

func (b *Block) EncodeBinary(w io.Writer) error {
	if err := b.Header.EncodeBinary(w); err != nil {
		return err
	}

	for _, tx := range b.Transactions {
		if err := tx.EncodeBinary(w); err != nil {
			return err
		}
	}

	return nil
}

func (b *Block) DecodeBinary(r io.Reader) error {
	if err := b.Header.DecodeBinary(r); err != nil {
		return err
	}

	for _, tx := range b.Transactions {
		if err := tx.DecodeBinary(r); err != nil {
			return err
		}
	}

	return nil
}
