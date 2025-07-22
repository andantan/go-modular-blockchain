package core

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"io"
)

type Block struct {
	*Header
	Transactions []Transaction
	Validator    crypto.PublicKey
	Signature    *crypto.Signature

	BlockHash types.Hash
}

func NewBlock(h *Header, txx []Transaction) *Block {
	return &Block{
		Header:       h,
		Transactions: txx,
	}
}

func (b *Block) Sign(privKey crypto.PrivateKey) error {
	sig, err := privKey.Sign(b.Bytes())

	if err != nil {
		return err
	}

	b.Validator = privKey.PublicKey()
	b.Signature = sig

	return nil
}

func (b *Block) Verify() error {
	if b.Signature == nil {
		return fmt.Errorf("block has no signature")
	}

	if !b.Signature.Verify(b.Validator, b.Bytes()) {
		return fmt.Errorf("invalid block signature")
	}

	return nil
}

// Hash Returns Hash of Block.Header
func (b *Block) Hash(hasher Hasher[*Block]) types.Hash {
	if b.BlockHash.IsZero() {
		b.BlockHash = hasher.Hash(b)
	}

	return b.BlockHash
}

func (b *Block) Decode(r io.Reader, dec Decoder[*Block]) error {
	return dec.Decode(r, b)
}

func (b *Block) Encode(w io.Writer, enc Encoder[*Block]) error {
	return enc.Encode(w, b)
}

// Bytes Returns byte slice of Block.Header
func (b *Block) Bytes() []byte {
	buf := &bytes.Buffer{}
	enc := gob.NewEncoder(buf)

	_ = enc.Encode(b.Header)

	return buf.Bytes()
}
