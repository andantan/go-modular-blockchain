package core

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/andantan/go-modular-blockchain/types"
	"os"
	"path/filepath"
)

type BlockStorage interface {
	StoreBlock(*Block) error
	GetBlockByHash(types.Hash) (*Block, error)
	GetBlockByHeight(uint64) (*Block, error)
	CurrentHeight() uint64
	ClearStorage() error
}

type DefaultBlockStorage struct {
	blocksDir      string
	marker         uint64
	BlockHashCache *types.SyncMap[uint64, types.Hash]
}

func NewDefaultBlockStorage(blocksDir string) (*DefaultBlockStorage, error) {
	fs := &DefaultBlockStorage{
		blocksDir:      blocksDir,
		marker:         0,
		BlockHashCache: types.NewSyncMap[uint64, types.Hash](),
	}

	if err := os.MkdirAll(blocksDir, os.ModePerm); err != nil {
		return nil, err
	}

	if err := fs.bootstrap(); err != nil {
		return nil, err
	}

	return fs, nil
}

func (fs *DefaultBlockStorage) bootstrap() error {
	files, err := os.ReadDir(fs.blocksDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		hashBytes, err := hex.DecodeString(file.Name())

		if err != nil {
			continue
		}

		hash, err := types.HashFromBytes(hashBytes)

		if err != nil {
			continue
		}

		block, err := fs.GetBlockByHash(hash)

		if err != nil {
			continue
		}

		if fs.BlockHashCache.Exists(block.Height) {
			fs.marker = block.Height

			return fmt.Errorf("duplicated block height %d", block.Height)
		}

		fs.BlockHashCache.Put(block.Height, block.BlockHash)
	}

	return nil
}

func (fs *DefaultBlockStorage) StoreBlock(block *Block) error {
	buf := new(bytes.Buffer)
	if err := block.Encode(NewGobBlockEncoder(buf)); err != nil {
		return err
	}

	blockHash := block.Hash(BlockHasher{})
	fileName := hex.EncodeToString(blockHash.Bytes())
	filePath := filepath.Join(fs.blocksDir, fileName)

	if err := os.WriteFile(filePath, buf.Bytes(), os.ModePerm); err != nil {
		return err
	}

	fs.BlockHashCache.Put(block.Header.Height, blockHash)

	return nil
}

func (fs *DefaultBlockStorage) GetBlockByHash(hash types.Hash) (*Block, error) {
	fileName := hex.EncodeToString(hash.Bytes())
	filePath := filepath.Join(fs.blocksDir, fileName)

	data, err := os.ReadFile(filePath)

	if err != nil {
		return nil, err
	}

	block := new(Block)

	if err := block.UnMarshall(bytes.NewReader(data)); err != nil {
		return nil, err
	}

	return block, nil
}

func (fs *DefaultBlockStorage) GetBlockByHeight(height uint64) (*Block, error) {
	hash, ok := fs.BlockHashCache.Get(height)

	if !ok {
		return nil, fmt.Errorf("block with height %d not found", height)
	}

	return fs.GetBlockByHash(hash)
}

func (fs *DefaultBlockStorage) CurrentHeight() uint64 {
	count := uint64(fs.BlockHashCache.Len())

	if count == 0 {
		return 0
	}

	return count - 1
}

func (fs *DefaultBlockStorage) ClearStorage() error {
	if err := os.RemoveAll(fs.blocksDir); err != nil {
		return err
	}

	fs.BlockHashCache = types.NewSyncMap[uint64, types.Hash]()

	return os.MkdirAll(fs.blocksDir, os.ModePerm)
}
