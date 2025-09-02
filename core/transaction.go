package core

import (
	"encoding/hex"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"io"
	"math/rand"
	"time"
)

type Transaction struct {
	Data      []byte
	From      crypto.PublicKey
	Signature crypto.Signature

	Nonce uint64

	hash      types.Hash
	firstSeen uint64
}

func NewTransaction(data []byte) *Transaction {
	return &Transaction{
		Data:  data,
		Nonce: rand.Uint64(),
	}
}

func (tx *Transaction) Debug(verbose bool) {
	shortLength := 8
	debugName := fmt.Sprintf("Transaction(%s)", tx.Hash(TxHasher{}).ShortString(shortLength))
	logger := config.DefaultDebugLogger(debugName)

	if verbose {
		_ = logger.Log(
			"data", hex.EncodeToString(tx.Data),
			"from", tx.From.Address().ShortString(shortLength),
			"sig", tx.Signature.String(),
			"nonce", tx.Nonce,
		)
	} else {
		dataFormat := fmt.Sprintf("[%d]byte", len(tx.Data))
		_ = logger.Log(
			"data", dataFormat,
			"from", tx.From.Address().ShortString(shortLength),
			"sig", tx.Signature.String()[:shortLength]+"...",
			"nonce", tx.Nonce,
		)
	}
}

func (tx *Transaction) Hash(hasher Hasher[*Transaction]) types.Hash {
	if tx.hash.IsZero() {
		tx.hash = hasher.Hash(tx)
	}

	return tx.hash
}

func (tx *Transaction) Sign(privKey crypto.PrivateKey) error {
	sig, err := privKey.Sign(tx.Data)

	if err != nil {
		return err
	}

	tx.From = privKey.PublicKey()
	tx.Signature = sig

	return nil
}

func (tx *Transaction) Verify() error {
	if tx.Signature.IsNil() {
		return fmt.Errorf("transaction has no signature")
	}

	if !tx.Signature.Verify(tx.From, tx.Data) {
		return fmt.Errorf("invalid transaction signature")
	}

	return nil
}

func (tx *Transaction) Encode(enc Encoder[*Transaction]) error {
	return enc.Encode(tx)
}

func (tx *Transaction) Marshall(w io.Writer) error {
	return tx.Encode(NewGobTxEncoder(w))
}

func (tx *Transaction) Decode(dec Decoder[*Transaction]) error {
	return dec.Decode(tx)
}

func (tx *Transaction) UnMarshall(r io.Reader) error {
	return tx.Decode(NewGobTxDecoder(r))
}

func (tx *Transaction) SetFirstSeen() {
	if tx.firstSeen != 0 {
		tx.firstSeen = uint64(time.Now().UnixNano())
	}
}

func (tx *Transaction) FirstSeen() uint64 {
	return tx.firstSeen
}
