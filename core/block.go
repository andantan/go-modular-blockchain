package core

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"time"
)

type Block struct {
	*Header
	Transactions []*Transaction
	Validator    crypto.PublicKey
	Signature    *crypto.Signature

	BlockHash types.Hash
}

func NewBlock(h *Header, txx []*Transaction) *Block {
	return &Block{
		Header:       h,
		Transactions: txx,
	}
}

func NewBlockFromPrevheader(prevHeader *Header, currentTxx []*Transaction) (*Block, error) {
	currentDataHash, err := CalculateDataHash(currentTxx)

	if err != nil {
		return nil, err
	}

	header := &Header{
		Version:       1,
		Height:        prevHeader.Height + 1,
		DataHash:      currentDataHash,
		PrevBlockHash: BlockHasher{}.Hash(prevHeader),
		Timestamp:     uint64(time.Now().UnixNano()),
	}

	return NewBlock(header, currentTxx), nil
}

func (b *Block) AddTransaction(tx *Transaction) {
	b.Transactions = append(b.Transactions, tx)
}

func (b *Block) Sign(privKey crypto.PrivateKey) error {
	sig, err := privKey.Sign(b.Header.Bytes())

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

	if !b.Signature.Verify(b.Validator, b.Header.Bytes()) {
		return fmt.Errorf("invalid block signature")
	}

	for _, tx := range b.Transactions {
		if err := tx.Verify(); err != nil {
			return err
		}
	}

	dataHash, err := CalculateDataHash(b.Transactions)

	if err != nil {
		return err
	}

	if dataHash != b.DataHash {
		return fmt.Errorf("block (%s) has an invalid block hash", b.Hash(BlockHasher{}))
	}

	return nil
}

// Hash Returns Hash of Block.Header
func (b *Block) Hash(hasher Hasher[*Header]) types.Hash {
	if b.BlockHash.IsZero() {
		b.BlockHash = hasher.Hash(b.Header)
	}

	return b.BlockHash
}

func (b *Block) Decode(dec Decoder[*Block]) error {
	return dec.Decode(b)
}

func (b *Block) Encode(enc Encoder[*Block]) error {
	return enc.Encode(b)
}

func CalculateDataHash(txx []*Transaction) (hash types.Hash, err error) {
	buf := &bytes.Buffer{}

	for _, tx := range txx {
		if err = tx.Encode(NewGobTxEncoder(buf)); err != nil {
			return
		}
	}

	hash = sha256.Sum256(buf.Bytes())

	return
}

func GetGenesisBlock() *Block {
	header := &Header{
		Version:       1,
		DataHash:      types.Hash{},
		Timestamp:     uint64(0),
		Height:        uint32(0),
		PrevBlockHash: types.Hash{},
	}

	return NewBlock(header, nil)
}
