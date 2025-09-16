package network

import (
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"io"
)

type PrePrepareMessage struct {
	Block     *core.Block
	Signature crypto.Signature
}

func NewPrePrepareMessage(block *core.Block) *PrePrepareMessage {
	return &PrePrepareMessage{
		Block: block,
	}
}

func (ppm *PrePrepareMessage) Sign(privKey crypto.PrivateKey) error {
	hash := ppm.Block.Hash(core.BlockHasher{})
	sig, err := privKey.Sign(hash.ToSlice())
	if err != nil {
		return err
	}
	ppm.Signature = sig
	return nil
}

func (ppm *PrePrepareMessage) Verify() error {
	proposerKey := ppm.Block.Proposer
	hash := ppm.Block.Hash(core.BlockHasher{})
	if !ppm.Signature.Verify(proposerKey, hash.ToSlice()) {
		return fmt.Errorf("invalid pre-prepare message signature")
	}

	return nil
}

func (ppm *PrePrepareMessage) Marshall(w io.Writer) error {
	return gob.NewEncoder(w).Encode(ppm)
}

func (ppm *PrePrepareMessage) Unmarshall(r io.Reader) error {
	return gob.NewDecoder(r).Decode(ppm)
}

type PrepareMessage struct {
	BlockHash types.Hash
	Height    uint64
	Validator crypto.PublicKey
	Signature crypto.Signature
}

func NewPrepareMessage(block *core.Block) *PrepareMessage {
	hash := block.Hash(core.BlockHasher{})
	return &PrepareMessage{
		BlockHash: hash,
		Height:    block.Height,
	}
}

func (pm *PrepareMessage) Sign(privKey crypto.PrivateKey) error {
	sig, err := privKey.Sign(pm.BlockHash.ToSlice())
	if err != nil {
		return err
	}
	pm.Validator = privKey.PublicKey()
	pm.Signature = sig

	return nil
}

func (pm *PrepareMessage) Verify() error {
	if !pm.Signature.Verify(pm.Validator, pm.BlockHash.ToSlice()) {
		return fmt.Errorf("invalid prepare signature")
	}
	return nil
}

func (pm *PrepareMessage) Marshall(w io.Writer) error {
	return gob.NewEncoder(w).Encode(pm)
}

func (pm *PrepareMessage) Unmarshall(r io.Reader) error {
	return gob.NewDecoder(r).Decode(pm)
}

type CommitMessage struct {
	BlockHash types.Hash
	Height    uint64
	Validator crypto.PublicKey
	Signature crypto.Signature
}

func NewCommitMessage(block *core.Block) *CommitMessage {
	hash := block.Hash(core.BlockHasher{})
	return &CommitMessage{
		BlockHash: hash,
		Height:    block.Height,
	}
}

func (cm *CommitMessage) Sign(privKey crypto.PrivateKey) error {
	sig, err := privKey.Sign(cm.BlockHash.ToSlice())
	if err != nil {
		return err
	}
	cm.Validator = privKey.PublicKey()
	cm.Signature = sig

	return nil
}

func (cm *CommitMessage) Verify() error {
	if !cm.Signature.Verify(cm.Validator, cm.BlockHash.ToSlice()) {
		return fmt.Errorf("invalid commit signature")
	}
	return nil
}

func (cm *CommitMessage) Marshall(w io.Writer) error {
	return gob.NewEncoder(w).Encode(cm)
}

func (cm *CommitMessage) Unmarshall(r io.Reader) error {
	return gob.NewDecoder(r).Decode(cm)
}
