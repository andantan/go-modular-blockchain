package core

import (
	"errors"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/go-kit/log"
)

var ErrBlockKnown = errors.New("block already known")
var ErrFutureBlock = errors.New("block is too high")
var ErrUnknownParent = errors.New("chain has been forked")

type Processor interface {
	ProcessBlock(*Block) error
}

type BlockProcessor struct {
	Logger log.Logger

	chain *Blockchain
}

func NewBlockProcessor(chain *Blockchain) *BlockProcessor {
	return &BlockProcessor{
		Logger: config.LoggerWithPrefixes("processor"),
		chain:  chain,
	}
}

func (p *BlockProcessor) ProcessBlock(b *Block) error {
	if b.Height == 0 {
		return b.Verify()
	}

	if p.chain.HasBlock(b.Height) {
		return ErrBlockKnown
	}

	if p.chain.Height()+1 != b.Height {
		return ErrFutureBlock
	}

	prevHeader, err := p.chain.GetHeader(b.Height - 1)

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

	//if p.chain.contract != nil {
	//	for _, tx := range b.Transactions {
	//		_ = p.Logger.Log(
	//			"msg", "executing code",
	//			"hash", b.Hash(BlockHasher{}),
	//			"length", len(tx.Data),
	//		)
	//
	//		if err = p.chain.contract.Execute(tx); err != nil {
	//			return err
	//		}
	//	}
	//}

	return nil
}
