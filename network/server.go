package network

import (
	"bytes"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/sirupsen/logrus"
	"time"
)

type ServerOpts struct {
	RPCDecodeFunc RPCDecodeFunc
	RPCProcesor   RPCProcesor
	Transports    []Transport
	BlockTime     time.Duration
	PrivateKey    *crypto.PrivateKey
}

type Server struct {
	ServerOpts

	memPool     *TxPool
	chain       *core.Blockchain
	isValidator bool
	rpcCh       chan RPC
	quitCh      chan struct{}
}

func NewServer(opts ServerOpts) (*Server, error) {
	if opts.BlockTime == time.Duration(0) {
		timerUnit := config.GetIntEnvVar("PARAMETER_BLOCK_TIME_UNIT")
		timerDuration := config.GetIntEnvVar("PARAMETER_BLOCK_TIME_DURATION")
		timer := timerUnit * timerDuration

		config.GetDefaultLogger().WithFields(logrus.Fields{
			"unit":     timerUnit,
			"duration": timerDuration,
			"timer":    timer,
		}).Info("set default block time")

		opts.BlockTime = time.Duration(timer)
	}

	if opts.RPCDecodeFunc == nil {
		opts.RPCDecodeFunc = DefaultRPCDecodeFunc
	}

	chain, err := core.NewBlockchain(core.GetGenesisBlock())

	if err != nil {
		return nil, err
	}

	s := &Server{
		ServerOpts:  opts,
		memPool:     NewTxPool(),
		chain:       chain,
		isValidator: opts.PrivateKey != nil,
		rpcCh:       make(chan RPC),
		quitCh:      make(chan struct{}, 1),
	}

	if s.RPCProcesor == nil {
		s.RPCProcesor = s
	}

	if s.isValidator {
		go s.validatorLoop()
	}

	return s, nil
}

func (s *Server) Start() {
	logger := config.GetDefaultLogger()
	s.initTransports()

free:
	for {
		select {
		case rpc := <-s.rpcCh:
			msg, err := s.RPCDecodeFunc(rpc)

			if err != nil {
				logger.Error(err)
			}

			if err := s.RPCProcesor.ProcessMessage(msg); err != nil {
				logger.Error(err)
			}
		case <-s.quitCh:
			break free
		}
	}

	logger.Info("server is shutting down")
}

func (s *Server) validatorLoop() {
	logger := config.GetDefaultLogger()
	ticker := time.NewTicker(s.BlockTime)

	logger.WithFields(logrus.Fields{
		"blockTime": s.BlockTime,
	}).Info("starting validator loop")

	for {
		<-ticker.C

		if err := s.createNewBlock(); err != nil {
			logger.Error(err)
		}
	}
}

func (s *Server) ProcessMessage(msg *DecodedMessage) error {
	switch t := msg.Data.(type) {
	case *core.Transaction:
		return s.processTransaction(t)
	}

	return nil
}

func (s *Server) broadcast(payload []byte) error {
	for _, tr := range s.Transports {
		if err := tr.Broadcast(payload); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) processTransaction(tx *core.Transaction) error {
	logger := config.GetDefaultLogger()
	hash := tx.Hash(core.TxHasher{})

	if s.memPool.Has(hash) {

		return nil
	}

	if err := tx.Verify(); err != nil {
		return err
	}

	tx.SetFirstSeen(uint64(time.Now().UnixNano()))

	logger.WithFields(logrus.Fields{
		"hash":           hash,
		"mempool-length": s.memPool.Len(),
	}).Info("adding new tx to the mempool")

	go func() {
		if err := s.broadcastTx(tx); err != nil {
			logger.Error(err)
		}
	}()

	return s.memPool.Add(tx)
}

func (s *Server) broadcastTx(tx *core.Transaction) error {
	buf := &bytes.Buffer{}

	if err := tx.Encode(core.NewGobTxEncoder(buf)); err != nil {
		return err
	}

	msg := NewMessage(MessageTypeTx, buf.Bytes())

	return s.broadcast(msg.Bytes())
}

func (s *Server) initTransports() {
	for _, tr := range s.Transports {
		go func(tr Transport) {
			for rpc := range tr.Consume() {
				s.rpcCh <- rpc
			}
		}(tr)
	}
}

func (s *Server) createNewBlock() error {
	currentHeader, err := s.chain.GetHeader(s.chain.Height())

	if err != nil {
		return err
	}

	block, err := core.NewBlockFromPrevheader(currentHeader, nil)

	if err != nil {
		return err
	}

	if err := block.Sign(*s.PrivateKey); err != nil {
		return err
	}

	if err := s.chain.AddBlock(block); err != nil {
		return err
	}

	return nil
}
