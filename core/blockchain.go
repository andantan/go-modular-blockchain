package core

import (
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/sirupsen/logrus"
	"sync"
)

type Blockchain struct {
	ID        string
	store     Storage
	lock      sync.RWMutex
	headers   []*Header
	validator Validator
}

func NewBlockchain(id string, genesis *Block) (*Blockchain, error) {
	bc := &Blockchain{
		ID:      id,
		headers: []*Header{},
		store:   NewMemoryStorage(),
	}

	bc.validator = NewBlockValidator(bc)
	err := bc.addBlockWithoutValidation(genesis)

	return bc, err
}

func (bc *Blockchain) SetValidator(validator Validator) {
	bc.validator = validator
}

func (bc *Blockchain) AddBlock(block *Block) error {
	if err := bc.validator.ValidateBlock(block); err != nil {
		return err
	}

	return bc.addBlockWithoutValidation(block)
}

func (bc *Blockchain) GetHeader(height uint32) (*Header, error) {
	if bc.Height() < height {
		return nil, fmt.Errorf("given height (%d) is too high", height)
	}

	bc.lock.Lock()
	defer bc.lock.Unlock()

	return bc.headers[height], nil
}

func (bc *Blockchain) HasBlock(height uint32) bool {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	return height <= bc.Height()
}

func (bc *Blockchain) Height() uint32 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	return uint32(len(bc.headers) - 1)
}

func (bc *Blockchain) addBlockWithoutValidation(block *Block) error {
	logger := config.GetBlockchainLogger()

	bc.lock.Lock()
	bc.headers = append(bc.headers, block.Header)
	bc.lock.Unlock()

	logger.WithFields(logrus.Fields{
		"ID":          bc.ID,
		"height":      block.Height,
		"hash":        block.Hash(BlockHasher{}),
		"length(txx)": len(block.Transactions),
	}).Info("new block")

	return bc.store.Put(block)
}
