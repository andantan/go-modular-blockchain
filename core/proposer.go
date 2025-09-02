package core

import (
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/go-kit/log"
)

type Proposer interface {
	CreateBlock() (*Block, error)
}

type BlockProposer struct {
	Logger log.Logger

	chain   *Blockchain
	mempool *Mempool

	privKey crypto.PrivateKey
}

func NewBlockPropoesr(chain *Blockchain, mempool *Mempool, privKey crypto.PrivateKey) *BlockProposer {
	logger := config.LoggerWithPrefixes("PROPOSER")

	return &BlockProposer{
		Logger:  logger,
		chain:   chain,
		mempool: mempool,
		privKey: privKey,
	}
}

func (bp *BlockProposer) CreateBlock() (*Block, error) {
	prevHeader, err := bp.chain.GetHeader(bp.chain.Height())

	if err != nil {
		return nil, err
	}

	txx := bp.mempool.Pending()
	block, err := NewBlockFromPrevHeader(prevHeader, txx)

	if err != nil {
		return nil, err
	}

	if err := block.Sign(bp.privKey); err != nil {
		return nil, err
	}

	return block, nil
}
