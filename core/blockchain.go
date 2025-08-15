package core

import (
	"fmt"
	"github.com/go-kit/log"
	"sync"
)

type Blockchain struct {
	Logger        log.Logger
	store         Storage
	lock          sync.RWMutex
	headers       []*Header
	blocks        []*Block
	validator     Validator
	contractState *State // TODO: make this an interface
}

func NewBlockchain(logger log.Logger, genesis *Block) (*Blockchain, error) {
	bc := &Blockchain{
		Logger:        logger,
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
	if err := bc.validator.ValidateBlock(block); err != nil {
		return err
	}

	for _, tx := range block.Transactions {
		_ = bc.Logger.Log(
			"msg", "executing code",
			"hash", block.Hash(BlockHasher{}),
			"length", len(tx.Data),
		)

		vm := NewVM(tx.Data, bc.contractState)

		if err := vm.Run(); err != nil {
			return err
		}

		// _ = bc.Logger.Log("state", fmt.Sprintf("%+v", bc.contractState))
	}

	return bc.addBlockWithoutValidation(block)
}

func (bc *Blockchain) GetBlock(height uint32) (*Block, error) {
	if bc.Height() < height {
		return nil, fmt.Errorf("given height (%d) is too high", height)
	}

	bc.lock.Lock()
	defer bc.lock.Unlock()

	return bc.blocks[height], nil
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

func (bc *Blockchain) addBlockWithoutValidation(b *Block) error {
	bc.lock.Lock()
	bc.headers = append(bc.headers, b.Header)
	bc.blocks = append(bc.blocks, b)
	bc.lock.Unlock()

	// test pruning
	_ = bc.Logger.Log(
		"msg", "new block",
		"height", b.Height,
		"hash", b.Hash(BlockHasher{}),
		"length(txx)", len(b.Transactions),
	)

	return bc.store.Put(b)
}
