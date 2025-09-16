package network

import (
	"errors"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
)

type PbftState byte

const (
	Idle PbftState = iota
	PrePrepared
	Prepared
	Committed
	Finalized
)

type PBFTConsensusEngine interface {
	State() PbftState
	Block() *core.Block
	HandleMessage(from types.Address, msg any) error
	IsFinalized() bool
}

type DefaultPBFTConsensusEngine struct {
	logger log.Logger
	state  *types.AtomicNumber[PbftState]
	block  *core.Block

	broadcaster  Broadcaster
	processor    core.Processor
	ourValidator core.Validator
	validatorSet *types.SyncSet[types.Address]

	prepareVotes *types.SyncMap[types.Address, *PrepareMessage]
	commitVotes  *types.SyncMap[types.Address, *CommitMessage]
}

func NewDefaultPBFTConsensusEngine(
	broadcaster Broadcaster,
	processor core.Processor,
	ourValidator core.Validator,
	validatorSet []types.Address,
) *DefaultPBFTConsensusEngine {
	e := &DefaultPBFTConsensusEngine{
		logger:       config.DefaultDebugLogger("consensus"),
		state:        types.NewAtomicNumber[PbftState](Idle),
		block:        nil,
		broadcaster:  broadcaster,
		processor:    processor,
		ourValidator: ourValidator,
		validatorSet: types.NewSyncSet[types.Address](),
		prepareVotes: types.NewSyncMap[types.Address, *PrepareMessage](),
		commitVotes:  types.NewSyncMap[types.Address, *CommitMessage](),
	}

	for _, v := range validatorSet {
		e.validatorSet.Put(v)
	}

	return e
}

func (e *DefaultPBFTConsensusEngine) State() PbftState {
	return e.state.Get()
}

func (e *DefaultPBFTConsensusEngine) Block() *core.Block {
	return e.block
}

func (e *DefaultPBFTConsensusEngine) HandleMessage(from types.Address, msg any) error {
	if !e.validatorSet.Contains(from) {
		return fmt.Errorf("received prepare vote from non-validator: %s", from.ShortString(8))
	}

	switch m := msg.(type) {
	case *PrePrepareMessage:
		return e.handlePrePrepare(m)
	case *PrepareMessage:
		return e.handlePrepare(m)
	case *CommitMessage:
		return e.handleCommit(m)
	default:
		return fmt.Errorf("unknown consensus message type: %T", m)
	}
}

func (e *DefaultPBFTConsensusEngine) IsFinalized() bool {
	return e.state.Eq(Finalized)
}

func (e *DefaultPBFTConsensusEngine) handlePrePrepare(msg *PrePrepareMessage) error {
	if !e.state.Eq(Idle) {
		return nil
	}

	if err := msg.Verify(); err != nil {
		return err
	}

	if err := e.processor.ProcessBlock(msg.Block); err != nil {
		if !errors.Is(err, core.ErrBlockKnown) {
			return err
		}

		return nil
	}

	if e.block != nil {
		return nil
	}

	e.block = msg.Block

	go func() {
		if err := e.broadcaster.Broadcast(msg); err != nil {
			_ = e.logger.Log("error", "failed to gossip pre-prepare message", "err", err)
		}
	}()

	_ = e.logger.Log(
		"msg", "block validation succeeded with pre-prepared message",
		"proposer", msg.Block.Proposer.Address().ShortString(8),
		"block-hash", msg.Block.Hash(core.BlockHasher{}),
		"height", msg.Block.Height,
	)
	e.state.Set(PrePrepared)

	prepareMsg := NewPrepareMessage(msg.Block)
	if err := prepareMsg.Sign(e.ourValidator.PrivateKey()); err != nil {
		return err
	}

	return e.broadcaster.Broadcast(prepareMsg)
}

func (e *DefaultPBFTConsensusEngine) handlePrepare(msg *PrepareMessage) error {
	currentState := e.state.Get()

	if currentState < PrePrepared || currentState >= Committed {
		return nil
	}

	if err := msg.Verify(); err != nil {
		return err
	}

	if e.block == nil || e.block.Hash(core.BlockHasher{}) != msg.BlockHash {
		return fmt.Errorf("received block hash mismatch: expected %s, got %s", e.block.BlockHash.ShortString(8), msg.BlockHash.ShortString(8))
	}

	fromAddr := msg.Validator.Address()

	if e.prepareVotes.Exists(fromAddr) {
		return nil
	}

	go func() {
		if err := e.broadcaster.Broadcast(msg); err != nil {
			_ = e.logger.Log("error", "failed to gossip prepare message", "err", err)
		}
	}()

	e.prepareVotes.Put(fromAddr, msg)
	requiredVotes := (2 * e.validatorSet.Len() / 3) + 1

	if e.prepareVotes.Len() >= requiredVotes && e.state.Eq(PrePrepared) {
		e.state.Set(Prepared)
		logMsg := fmt.Sprintf("prepare quorum reached %d/%d", e.prepareVotes.Len(), requiredVotes)
		_ = e.logger.Log(
			"msg", logMsg,
			"block-hash", msg.BlockHash.ShortString(8),
			"height", msg.Height,
		)

		commitMsg := NewCommitMessage(e.block)
		if err := commitMsg.Sign(e.ourValidator.PrivateKey()); err != nil {
			return err
		}

		return e.broadcaster.Broadcast(commitMsg)
	}

	return nil
}

func (e *DefaultPBFTConsensusEngine) handleCommit(msg *CommitMessage) error {
	currentState := e.state.Get()

	if currentState < Prepared || currentState >= Finalized {
		return nil
	}

	if err := msg.Verify(); err != nil {
		return err
	}

	if e.block == nil || e.block.Hash(core.BlockHasher{}) != msg.BlockHash {
		return fmt.Errorf("received block hash mismatch: expected %s, got %s", e.block.BlockHash.ShortString(8), msg.BlockHash.ShortString(8))
	}

	fromAddr := msg.Validator.Address()

	if e.commitVotes.Exists(fromAddr) {
		return nil
	}

	go func() {
		if err := e.broadcaster.Broadcast(msg); err != nil {
			_ = e.logger.Log("error", "failed to gossip commit message", "err", err)
		}
	}()

	e.commitVotes.Put(fromAddr, msg)

	requiredVotes := (2 * e.validatorSet.Len() / 3) + 1
	if e.commitVotes.Len() >= requiredVotes && e.state.Eq(Prepared) {
		e.state.Set(Committed)
		logMsg := fmt.Sprintf("commit quorum reached %d/%d, block is commited", e.commitVotes.Len(), requiredVotes)

		_ = e.logger.Log(
			"msg", logMsg,
			"height", e.block.Height,
		)
		e.finalizeBlock()
	}

	return nil
}

func (e *DefaultPBFTConsensusEngine) finalizeBlock() {
	e.state.Set(Finalized)

	_ = e.broadcaster.Broadcast(e.block)
}
