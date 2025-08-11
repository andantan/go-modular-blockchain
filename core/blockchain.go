package core

import (
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/sirupsen/logrus"
	"sync"
)

type Blockchain struct {
	ID            string
	store         Storage
	lock          sync.RWMutex
	headers       []*Header
	validator     Validator
	contractState *State // TODO: make this an interface
}

func NewBlockchain(id string, genesis *Block) (*Blockchain, error) {
	bc := &Blockchain{
		ID:            id,
		headers:       []*Header{},
		store:         NewMemoryStorage(),
		contractState: NewState(),
	}

	bc.validator = NewBlockValidator(bc)
	err := bc.addBlockWithoutValidation(genesis)

	return bc, err
}

func (bc *Blockchain) SetValidator(validator Validator) {
	bc.validator = validator
}

func (bc *Blockchain) AddBlock(block *Block) error {
	logger := config.GetBlockchainLogger()

	if err := bc.validator.ValidateBlock(block); err != nil {
		return err
	}

	for _, tx := range block.Transactions {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"hash":   block.Hash(BlockHasher{}),
				"length": len(tx.Data),
			}).Info("executing code")
		}

		vm := NewVM(tx.Data, bc.contractState)

		if err := vm.Run(); err != nil {
			return err
		}

		// fmt.Printf("STATE: %+v\n", vm.contractState)

		// result := vm.stack.Pop()

		// fmt.Printf("VM RESULT: %+v\n", result)
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

	// test pruning
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"ID":          bc.ID,
			"height":      block.Height,
			"hash":        block.Hash(BlockHasher{}),
			"length(txx)": len(block.Transactions),
		}).Info("new block")
	}

	return bc.store.Put(block)
}
