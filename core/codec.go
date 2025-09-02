package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"github.com/andantan/go-modular-blockchain/types"
	"io"
)

type Hasher[T any] interface {
	Hash(T) types.Hash
}

type TxHasher struct{}

func (TxHasher) Hash(tx *Transaction) types.Hash {
	buf := new(bytes.Buffer)

	_ = binary.Write(buf, binary.LittleEndian, tx.Nonce)
	buf.Write(tx.Data)
	buf.Write(tx.From.Key)
	buf.Write(tx.Signature.Bytes())

	return sha256.Sum256(buf.Bytes())
}

type BlockHasher struct{}

func (BlockHasher) Hash(b *Header) types.Hash {
	h := sha256.Sum256(b.Bytes())

	return types.Hash(h[:])
}

type Encoder[T any] interface {
	Encode(T) error
}

type Decoder[T any] interface {
	Decode(T) error
}

type GobTxEncoder struct {
	w io.Writer
}

func NewGobTxEncoder(w io.Writer) *GobTxEncoder {
	return &GobTxEncoder{
		w: w,
	}
}

func (e *GobTxEncoder) Encode(tx *Transaction) error {
	return gob.NewEncoder(e.w).Encode(tx)
}

type GobTxDecoder struct {
	r io.Reader
}

func NewGobTxDecoder(r io.Reader) *GobTxDecoder {
	return &GobTxDecoder{
		r: r,
	}
}

func (e *GobTxDecoder) Decode(tx *Transaction) error {
	return gob.NewDecoder(e.r).Decode(tx)
}

type GobBlockEncoder struct {
	w io.Writer
}

func NewGobBlockEncoder(w io.Writer) *GobBlockEncoder {
	return &GobBlockEncoder{
		w: w,
	}
}

func (e *GobBlockEncoder) Encode(b *Block) error {
	return gob.NewEncoder(e.w).Encode(b)
}

type GobBlockDecoder struct {
	r io.Reader
}

func NewGobBlockDecoder(r io.Reader) *GobBlockDecoder {
	return &GobBlockDecoder{
		r: r,
	}
}

func (d *GobBlockDecoder) Decode(b *Block) error {
	return gob.NewDecoder(d.r).Decode(b)
}
