package core

import "fmt"

type Validator interface {
	ValidateBlock(*Block) error
}

type BlockValidator struct {
	bc *Blockchain
}

func NewBlockValidator(bc *Blockchain) *BlockValidator {
	return &BlockValidator{
		bc: bc,
	}
}

func (bv *BlockValidator) ValidateBlock(b *Block) error {
	if bv.bc.HasBlock(b.Height) {
		return fmt.Errorf("chain already contains block (height=%d) with hash (%s)", b.Height, b.Hash(BlockHasher{}))
	}

	if bv.bc.Height()+1 != b.Height {
		return fmt.Errorf("block (%s) with height (%d) is too high => current height: (%d)", b.Hash(BlockHasher{}), b.Height, bv.bc.Height())
	}

	prevHeader, err := bv.bc.GetHeader(b.Height - 1)

	if err != nil {
		return err
	}

	prevHeaderHash := BlockHasher{}.Hash(prevHeader)

	if prevHeaderHash != b.PrevBlockHash {
		return fmt.Errorf("the hash of the previous block (%s) is invalid", b.PrevBlockHash)
	}

	if err = b.Verify(); err != nil {
		return err
	}

	return nil
}
