package core

import (
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
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

func NewBlockProposer(bc *Blockchain, m *Mempool, pk crypto.PrivateKey) *BlockProposer {
	return &BlockProposer{
		Logger:  config.LoggerWithPrefixes("proposer"),
		chain:   bc,
		mempool: m,
		privKey: pk,
	}
}

func (bp *BlockProposer) PublicKey() crypto.PublicKey {
	return bp.privKey.PublicKey()
}

func (bp *BlockProposer) Address() types.Address {
	return bp.privKey.PublicKey().Address()
}

func (bp *BlockProposer) CreateBlock() (b *Block, err error) {
	var prevHeader *Header

	if prevHeader, err = bp.chain.GetHeader(bp.chain.Height()); err != nil {
		return
	}

	txx := bp.mempool.Pending()

	if b, err = NewBlockFromPrevHeader(prevHeader, txx); err != nil {
		return
	}

	if err = b.Sign(bp.privKey); err != nil {
		return
	}

	return
}
