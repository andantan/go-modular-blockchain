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

	headers     *types.SyncList[*Header]
	headerStore *types.SyncMap[types.Hash, *Header]
	version     *types.AtomicNumber[uint16]

	processor Processor
	contract  Contract
}

func NewBlockchain(blockDir string) (*Blockchain, error) {
	bc := &Blockchain{
		Logger:      config.LoggerWithPrefixes("chain"),
		headers:     types.NewSyncList[*Header](),
		headerStore: types.NewSyncMap[types.Hash, *Header](),
		version:     types.NewAtomicNumber[uint16](1),
		contract:    nil,
	}

	if bc.fileStorage == nil {
		fs, err := NewDefaultBlockStorage(blockDir)

		if err != nil {
			return nil, err
		}

		bc.fileStorage = fs
	}

	if bc.processor == nil {
		bc.processor = NewBlockProcessor(bc)
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

func (bc *Blockchain) WithBlockStore(s BlockStorer) *Blockchain {
	bc.fileStorage = s
	return bc
}

func (bc *Blockchain) WithContract(contract Contract) *Blockchain {
	bc.contract = contract
	return bc
}

func (bc *Blockchain) Processor() Processor {
	return bc.processor
}

func (bc *Blockchain) Height() uint64 {
	if bc.headers.Len() == 0 {
		return uint64(0)
	}

	return uint64(bc.headers.Len() - 1)
}

func (bc *Blockchain) Version() uint16 {
	return bc.version.Get()
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

func (bc *Blockchain) HasHeader(h types.Hash) bool {
	return bc.headerStore.Exists(h)
}

func (bc *Blockchain) HasBlock(height uint64) bool {
	return height <= bc.Height()
}

func (bc *Blockchain) GetBlock(height uint64) (*Block, error) {
	return bc.fileStorage.GetBlockByHeight(height)
}

func (bc *Blockchain) ClearHeader() {
	bc.headers.Clear()
}

func (bc *Blockchain) ClearStorage() error {
	return bc.fileStorage.ClearStorage()
}

func (bc *Blockchain) Rollback(height uint64) error {
	currentHeight := bc.Height()

	if height > currentHeight {
		return nil
	}

	_ = bc.Logger.Log("msg", "rolling back chain", "from_height", currentHeight, "to_height", height)

	for i := currentHeight; i >= height; i-- {
		header, err := bc.GetHeader(i)

		if err != nil {
			return err
		}

		hash := BlockHasher{}.Hash(header)

		bc.headers.Remove(header)
		bc.headerStore.Remove(hash)

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

func (bc *Blockchain) AddBlock(b *Block) error {
	if err := bc.verifyAndAddInMemory(b); err != nil {
		return err
	}

	_ = bc.Logger.Log(
		"msg", "new block added",
		"hash", b.Hash(BlockHasher{}).ShortString(8),
		"height", b.Height,
		"weight", b.Weight,
	)

	return bc.fileStorage.StoreBlock(b)
}

func (bc *Blockchain) loadBlocks() error {
	storedHeight := bc.fileStorage.CurrentHeight()

	if storedHeight == 0 {
		if err := bc.AddBlock(GetGenesisBlock()); err != nil {
			panic("error adding genesis block: " + err.Error())
		}

		return nil
	}

	for height := uint64(0); height <= storedHeight; height++ {
		block, err := bc.fileStorage.GetBlockByHeight(height)

		if err != nil {
			return err
		}

		if err = bc.verifyAndAddInMemory(block); err != nil {
			return &BlockSyncingError{
				Height:  height,
				Message: err.Error(),
			}
		}

		_ = bc.Logger.Log("msg", "loaded block", "height", height, "hash", block.BlockHash.String())
	}

	return nil
}

func (bc *Blockchain) verifyAndAddInMemory(b *Block) error {
	if err := bc.processor.ProcessBlock(b); err != nil {
		return err
	}
	bc.headers.Insert(b.Header)
	bc.headerStore.Put(b.Hash(BlockHasher{}), b.Header)
	bc.version.Set(b.Version)

	return nil
}
