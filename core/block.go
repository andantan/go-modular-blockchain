package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"io"
	"sort"
	"time"
)

type Header struct {
	Version       uint16
	MerkleRoot    types.Hash
	PrevBlockHash types.Hash
	Timestamp     uint64
	Height        uint64
	Weight        uint64
}

func (h *Header) Bytes() []byte {
	buf := new(bytes.Buffer)

	_ = gob.NewEncoder(buf).Encode(h)

	return buf.Bytes()
}

type Block struct {
	*Header

	Transactions []*Transaction
	Proposer     crypto.PublicKey
	Signature    crypto.Signature

	BlockHash types.Hash
}

func NewBlock(h *Header, txx []*Transaction) *Block {
	return &Block{
		Header:       h,
		Transactions: txx,
		BlockHash:    BlockHasher{}.Hash(h),
	}
}

func NewBlockFromPrevHeader(prevHeader *Header, currentTxx []*Transaction) (*Block, error) {
	currentMerkleRoot, err := CalculateMerkleRoot(currentTxx)

	if err != nil {
		return nil, err
	}

	header := &Header{
		Version:       1,
		MerkleRoot:    currentMerkleRoot,
		PrevBlockHash: BlockHasher{}.Hash(prevHeader),
		Timestamp:     uint64(time.Now().UnixNano()),
		Height:        prevHeader.Height + 1,
		Weight:        uint64(len(currentTxx)),
	}

	return NewBlock(header, currentTxx), nil
}

func (b *Block) Debug(blockVerbose bool, txxVerbose bool) {
	shortLength := 8
	debugName := fmt.Sprintf("Block(%s)", b.Hash(BlockHasher{}).ShortString(shortLength))
	logger := config.DefaultDebugLogger(debugName)

	if blockVerbose {
		_ = logger.Log(
			"version", b.Header.Version,
			"merkleroot", b.MerkleRoot.String(),
			"height", b.Header.Height,
			"weight", b.Header.Weight,
			"prevBlockHash", b.Header.PrevBlockHash.String(),
			"timestamp", b.Header.Timestamp,
			"validator", b.Proposer.Address().String(),
			"signature", b.Signature.String(),
		)
	} else {
		_ = logger.Log(
			"height", b.Height,
			"merkleroot", b.MerkleRoot.ShortString(shortLength),
			"prevBlockHash", b.Header.PrevBlockHash.ShortString(shortLength),
			"timestamp", b.Header.Timestamp,
			"weight", b.Header.Weight,
		)
	}

	if txxVerbose {
		for _, tx := range b.Transactions {
			tx.Debug(false)
		}
	}
}

func (b *Block) AddTransaction(tx *Transaction) {
	b.Transactions = append(b.Transactions, tx)
}

func (b *Block) Sign(privKey crypto.PrivateKey) error {
	sig, err := privKey.Sign(b.Bytes())

	if err != nil {
		return err
	}

	b.Proposer = privKey.PublicKey()
	b.Signature = sig

	return nil
}

func (b *Block) Verify() error {
	if b.Signature.IsNil() {
		return fmt.Errorf("block has no signature")
	}

	if !b.Signature.Verify(b.Proposer, b.Header.Bytes()) {
		return fmt.Errorf("invalid block signature")
	}

	for _, tx := range b.Transactions {
		if err := tx.Verify(); err != nil {
			return err
		}
	}

	merkleRoot, err := CalculateMerkleRoot(b.Transactions)

	if err != nil {
		return err
	}

	if merkleRoot != b.MerkleRoot {
		return fmt.Errorf("block (%s) has an invalid block hash", b.Hash(BlockHasher{}))
	}

	return nil
}

func (b *Block) Hash(hasher Hasher[*Header]) types.Hash {
	if b.BlockHash.IsZero() {
		b.BlockHash = hasher.Hash(b.Header)
	}

	return b.BlockHash
}

func (b *Block) Encode(enc Encoder[*Block]) error {
	return enc.Encode(b)
}

func (b *Block) Marshall(w io.Writer) error {
	return b.Encode(NewGobBlockEncoder(w))
}

func (b *Block) Decode(dec Decoder[*Block]) error {
	return dec.Decode(b)
}

func (b *Block) UnMarshall(r io.Reader) error {
	return b.Decode(NewGobBlockDecoder(r))
}

func CalculateMerkleRoot(txx []*Transaction) (types.Hash, error) {
	if txx == nil {
		return types.Hash{}, nil
	}

	sort.Slice(txx, func(i, j int) bool {
		txHashA := txx[i].Hash(TxHasher{})
		txHashB := txx[j].Hash(TxHasher{})

		return bytes.Compare(txHashA.ToSlice(), txHashB.ToSlice()) < 0
	})

	var hashes []types.Hash
	for _, tx := range txx {
		hashes = append(hashes, tx.Hash(TxHasher{}))
	}

	for len(hashes) > 1 {
		if len(hashes)%2 != 0 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		var nextLevelHashes []types.Hash

		for i := 0; i < len(hashes); i += 2 {
			left := hashes[i]
			right := hashes[i+1]

			combinedHashData := append(left.ToSlice(), right.ToSlice()...)
			parentHash := types.Hash(sha256.Sum256(combinedHashData))

			nextLevelHashes = append(nextLevelHashes, parentHash)
		}

		hashes = nextLevelHashes
	}

	return hashes[0], nil
}

func GetGenesisBlock() *Block {
	header := &Header{
		Version:       uint16(1),
		MerkleRoot:    types.Hash{},
		Timestamp:     uint64(0),
		Height:        uint64(0),
		PrevBlockHash: types.Hash{},
	}

	b := NewBlock(header, nil)
	privKey, err := crypto.GeneratePrivateKey()

	if err != nil {
		panic(err)
	}

	if err = b.Sign(privKey); err != nil {
		panic(err)
	}

	return b
}
