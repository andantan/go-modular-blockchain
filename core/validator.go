package core

import (
	"errors"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/go-kit/log"
)

var ErrBlockKnown = errors.New("block already known")
var ErrFutureBlock = errors.New("block is too high")
var ErrUnknownParent = errors.New("chain has been forked")

type Validator interface {
	ValidateBlock(*Block) error
}

type BlockValidator struct {
	Logger log.Logger

	chain *Blockchain
}

func NewBlockValidator(chain *Blockchain) *BlockValidator {
	logger := config.LoggerWithPrefixes("VALIDATOR")

	return &BlockValidator{
		Logger: logger,
		chain:  chain,
	}
}

func (bv *BlockValidator) ValidateBlock(b *Block) error {
	if b.Height == 0 {
		return b.Verify()
	}

	if bv.chain.HasBlock(b.Height) {
		return ErrBlockKnown
	}

	if bv.chain.Height()+1 != b.Height {
		return ErrFutureBlock
	}

	prevHeader, err := bv.chain.GetHeader(b.Height - 1)

	if err != nil {
		return err
	}

	prevHeaderHash := BlockHasher{}.Hash(prevHeader)

	if prevHeaderHash != b.PrevBlockHash {
		return ErrUnknownParent
	}

	if err = b.Verify(); err != nil {
		return err
	}

	return nil
}
