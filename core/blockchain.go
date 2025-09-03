package core

import (
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
)

type BlockSyncingError struct {
	Height  uint64
	Message string
}

func (e *BlockSyncingError) Error() string {
	return fmt.Sprintf("block syncing error at %d: %v", e.Height, e.Message)
}

type Blockchain struct {
	Logger      log.Logger
	fileStorage BlockStorage

	headers *types.SyncList[*Header]
	version uint16

	validator Validator
	proposer  Proposer
	contract  Contract
}

func NewBlockchain(blockDir string) (*Blockchain, error) {
	bc := &Blockchain{
		Logger:  config.LoggerWithPrefixes("BLOCKCHAIN"),
		headers: types.NewSyncList[*Header](),
		version: 1,
	}

	if bc.fileStorage == nil {
		fs, err := NewDefaultBlockStorage(blockDir)

		if err != nil {
			return nil, err
		}

		bc.fileStorage = fs
	}

	if bc.validator == nil {
		bc.validator = NewBlockValidator(bc)
	}

	if err := bc.loadBlocks(); err != nil {
		return nil, err
	}

	return bc, nil
}

func (bc *Blockchain) WithLogger(logger log.Logger) *Blockchain {
	bc.Logger = logger

	return bc
}

func (bc *Blockchain) WithBlockStore(store BlockStorage) *Blockchain {
	bc.fileStorage = store
	return bc
}

func (bc *Blockchain) WithValidator(validator Validator) *Blockchain {
	bc.validator = validator
	return bc
}

func (bc *Blockchain) WithProposer(proposer Proposer) *Blockchain {
	bc.proposer = proposer
	return bc
}

func (bc *Blockchain) WithContract(contract Contract) *Blockchain {
	bc.contract = contract
	return bc
}

func (bc *Blockchain) Height() uint64 {
	return uint64(bc.headers.Len() - 1)
}

func (bc *Blockchain) Version() uint16 {
	return bc.version
}

func (bc *Blockchain) IsValidator() bool {
	return bc.validator != nil
}

func (bc *Blockchain) IsProposer() bool {
	return bc.proposer != nil
}

func (bc *Blockchain) ValidateBlock(b *Block) error {
	return bc.validator.ValidateBlock(b)
}

func (bc *Blockchain) CreateBlock() (*Block, error) {
	return bc.proposer.CreateBlock()
}

func (bc *Blockchain) AddBlock(b *Block) error {
	if err := bc.validator.ValidateBlock(b); err != nil {
		return err
	}

	if bc.contract != nil {
		for _, tx := range b.Transactions {
			_ = bc.Logger.Log(
				"msg", "executing code",
				"hash", b.Hash(BlockHasher{}),
				"length", len(tx.Data),
			)

			if err := bc.contract.Execute(tx); err != nil {
				return err
			}
		}
	}

	return bc.commitBlock(b)
}

func (bc *Blockchain) HasBlock(height uint64) bool {
	return height <= bc.Height()
}

func (bc *Blockchain) ClearHeader() {
	bc.headers.Clear()
}

func (bc *Blockchain) ClearStorage() error {
	return bc.fileStorage.ClearStorage()
}

func (bc *Blockchain) GetHeader(height uint64) (*Header, error) {
	if bc.Height() < height {
		return nil, fmt.Errorf("given height (%d) is too high", height)
	}

	h, err := bc.headers.Get(int(height))

	if err != nil {
		return nil, err
	}

	return h, nil
}

func (bc *Blockchain) GetBlockByHeight(height uint64) (*Block, error) {
	return bc.fileStorage.GetBlockByHeight(height)
}

func (bc *Blockchain) loadBlocks() error {
	storedHeight := bc.fileStorage.CurrentHeight()

	if storedHeight == 0 {
		if err := bc.commitBlock(GetGenesisBlock()); err != nil {
			fmt.Printf("error adding genesis block: %v\n", err)
		}

		return nil
	}

	height := uint64(0)

	for height <= storedHeight {
		block, err := bc.fileStorage.GetBlockByHeight(height)

		if err = bc.validator.ValidateBlock(block); err != nil {
			return &BlockSyncingError{
				Height:  height,
				Message: err.Error(),
			}
		}

		bc.headers.Insert(block.Header)

		_ = bc.Logger.Log(
			"msg", "loaded block",
			"height", height,
			"hash", block.BlockHash.String(),
		)

		height++
	}

	return nil
}

func (bc *Blockchain) commitBlock(b *Block) error {
	bc.headers.Insert(b.Header)
	bc.version = b.Version

	b.Debug(false, false)

	return bc.fileStorage.StoreBlock(b)
}
