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
	fileStorage BlockStorer

	headers      *types.SyncList[*Header]
	headersStore *types.SyncMap[types.Hash, *Header]
	version      uint16

	validator Validator
	contract  Contract
}

func NewBlockchain(blockDir string) (*Blockchain, error) {
	bc := &Blockchain{
		Logger:       config.LoggerWithPrefixes("blockchain"),
		headers:      types.NewSyncList[*Header](),
		headersStore: types.NewSyncMap[types.Hash, *Header](),
		version:      1,
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

func (bc *Blockchain) WithBlockStore(store BlockStorer) *Blockchain {
	bc.fileStorage = store
	return bc
}

func (bc *Blockchain) WithValidator(validator Validator) *Blockchain {
	bc.validator = validator
	return bc
}

func (bc *Blockchain) WithContract(contract Contract) *Blockchain {
	bc.contract = contract
	return bc
}

func (bc *Blockchain) IsValidator() bool {
	return bc.validator != nil
}

func (bc *Blockchain) Height() uint64 {
	return uint64(bc.headers.Len() - 1)
}

func (bc *Blockchain) Version() uint16 {
	return bc.version
}

func (bc *Blockchain) AddHeader(h *Header) error {
	bc.headers.Insert(h)
	bc.headersStore.Put(BlockHasher{}.Hash(h), h)
	bc.version = h.Version

	return nil
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

func (bc *Blockchain) HasHeader(h types.Hash) bool {
	return bc.headersStore.Exists(h)
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

func (bc *Blockchain) GetBlock(height uint64) (*Block, error) {
	return bc.fileStorage.GetBlockByHeight(height)
}

func (bc *Blockchain) Rollback(height uint64) error {
	currentHeight := bc.Height()

	if height >= currentHeight {
		return nil
	}

	_ = bc.Logger.Log("msg", "rolling back chain", "from_height", currentHeight, "to_height", height)

	for i := currentHeight; i > height; i-- {
		header, err := bc.GetHeader(i)

		if err != nil {
			return err
		}

		hash := BlockHasher{}.Hash(header)

		bc.headers.Remove(header)
		bc.headersStore.Remove(hash)

		if err = bc.fileStorage.RemoveBlock(hash); err != nil {
			_ = bc.Logger.Log(
				"error", "failed to remove block from storage",
				"height", i,
				"err", err,
			)
		}
	}

	return nil
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

		_ = bc.Logger.Log("msg", "loaded block", "height", height, "hash", block.BlockHash.String())

		height++
	}

	return nil
}

func (bc *Blockchain) commitBlock(b *Block) error {
	if err := bc.AddHeader(b.Header); err != nil {
		return err
	}

	// b.Debug(false, false)

	_ = bc.Logger.Log(
		"msg", "new block committed",
		"hash", b.Hash(BlockHasher{}).ShortString(8),
		"height", b.Height,
		"weight", b.Weight,
	)

	return bc.fileStorage.StoreBlock(b)
}
