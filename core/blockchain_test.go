package core

import (
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNewBlockChain(t *testing.T) {
	bc := newBlockchainWithGenesis(t)

	assert.NotNil(t, bc.validator)
	assert.Equal(t, uint32(0), bc.Height())
}

func TestAddBlock(t *testing.T) {
	bc := newBlockchainWithGenesis(t)

	clocks := 50
	for i := 0; i < clocks; i++ {
		privKey := crypto.GeneratePrivateKey()
		block := NewRandomBlockWithSignature(t, privKey, uint32(i+1), getPrevBlockHash(t, bc, uint32(i+1)))
		assert.Nil(t, bc.AddBlock(block))
	}

	assert.Equal(t, uint32(clocks), bc.Height())
	assert.Equal(t, clocks+1, len(bc.headers))
	assert.NotNil(t, bc.AddBlock(NewRandomBlock(t, 88, types.Hash{})))
}

func TestHasBlock(t *testing.T) {
	bc := newBlockchainWithGenesis(t)

	assert.True(t, bc.HasBlock(0))
	assert.False(t, bc.HasBlock(1))
	assert.False(t, bc.HasBlock(100))
}

func TestGetHeader(t *testing.T) {
	bc := newBlockchainWithGenesis(t)

	clocks := 50
	for i := 0; i < clocks; i++ {
		privKey := crypto.GeneratePrivateKey()
		block := NewRandomBlockWithSignature(t, privKey, uint32(i+1), getPrevBlockHash(t, bc, uint32(i+1)))
		assert.Nil(t, bc.AddBlock(block))

		header, err := bc.GetHeader(block.Height)
		assert.Nil(t, err)
		assert.Equal(t, header, block.Header)
	}
}

func TestAddBlockToHigh(t *testing.T) {
	bc := newBlockchainWithGenesis(t)

	privKeyFirst := crypto.GeneratePrivateKey()
	assert.Nil(t, bc.AddBlock(NewRandomBlockWithSignature(t, privKeyFirst, uint32(1), getPrevBlockHash(t, bc, uint32(1)))))
	privKeySecond := crypto.GeneratePrivateKey()
	assert.NotNil(t, bc.AddBlock(NewRandomBlockWithSignature(t, privKeySecond, 3, types.Hash{})))
}

func newBlockchainWithGenesis(t *testing.T) *Blockchain {
	logger := log.NewLogfmtLogger(os.Stdout)
	bc, err := NewBlockchain(logger, NewRandomBlock(t, 0, types.Hash{}))
	assert.Nil(t, err)

	return bc
}

func getPrevBlockHash(t *testing.T, bc *Blockchain, height uint32) types.Hash {
	prevHeader, err := bc.GetHeader(height - 1)
	assert.Nil(t, err)

	return BlockHasher{}.Hash(prevHeader)
}
