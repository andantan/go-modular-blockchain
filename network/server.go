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
	ID            string
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

		opts.BlockTime = time.Duration(timer)
	}

	if opts.RPCDecodeFunc == nil {
		opts.RPCDecodeFunc = DefaultRPCDecodeFunc
	}

	txMaxLength := config.GetIntEnvVar("PARAMETER_TX_MAX_LENGTH")
	chain, err := core.NewBlockchain(opts.ID, core.GetGenesisBlock())

	if err != nil {
		return nil, err
	}

	s := &Server{
		ServerOpts:  opts,
		memPool:     NewTxPool(txMaxLength),
		chain:       chain,
		isValidator: opts.PrivateKey != nil,
		rpcCh:       make(chan RPC),
		quitCh:      make(chan struct{}, 1),
	}

	if s.RPCProcesor == nil {
		s.RPCProcesor = s
	}

	config.GetServerLogger().WithFields(logrus.Fields{
		"ID":          opts.ID,
		"isValidator": s.isValidator,
		"blockTime":   opts.BlockTime,
	}).Info("new server created")

	if s.isValidator {
		go s.validatorLoop()
	}

	return s, nil
}

func (s *Server) Start() {
	logger := config.GetServerLogger()
	s.initTransports()

free:
	for {
		select {
		case rpc := <-s.rpcCh:
			msg, err := s.RPCDecodeFunc(rpc)

			if err != nil {
				logger.WithFields(logrus.Fields{
					"ID": s.ID,
				}).Error(err)
			}

			if err := s.RPCProcesor.ProcessMessage(msg); err != nil {
				logger.WithFields(logrus.Fields{
					"ID": s.ID,
				}).Error(err)
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
		"ID": s.ID,
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
	case *core.Block:
		return s.processBlock(t)
	}

	return nil
}

func (s *Server) broadcastBlock(b *core.Block) error {
	buf := &bytes.Buffer{}

	if err := b.Encode(core.NewGobBlockEncoder(buf)); err != nil {
		return err
	}

	msg := NewMessage(MessageTypeBlock, buf.Bytes())

	return s.broadcast(msg.Bytes())
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
	logger := config.GetServerLogger()
	hash := tx.Hash(core.TxHasher{})

	if s.memPool.Contains(hash) {
		return nil
	}

	if err := tx.Verify(); err != nil {
		return err
	}

	tx.SetFirstSeen(uint64(time.Now().UnixNano()))

	//logger.WithFields(logrus.Fields{
	//	"ID":            s.ID,
	//	"hash":          hash,
	//	"mempoolLength": s.memPool.PendingCount() + 1,
	//}).Info("trying to add new tx to the mempool")

	go func() {
		if err := s.broadcastTx(tx); err != nil {
			logger.Error(err)
		}
	}()

	s.memPool.Add(tx)

	return nil
}

func (s *Server) processBlock(b *core.Block) error {
	if err := s.chain.AddBlock(b); err != nil {
		return err
	}

	// butterfly effect in gossip network
	go func() {
		_ = s.broadcastBlock(b)
	}()

	return nil
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
	prevHeader, err := s.chain.GetHeader(s.chain.Height())

	if err != nil {
		return err
	}

	txx := s.memPool.Pending()

	block, err := core.NewBlockFromPrevheader(prevHeader, txx)

	if err != nil {
		return err
	}

	if err := block.Sign(*s.PrivateKey); err != nil {
		return err
	}

	if err := s.chain.AddBlock(block); err != nil {
		return err
	}

	s.memPool.ClearPending()

	go func() {
		_ = s.broadcastBlock(block)
	}()

	return nil
}
