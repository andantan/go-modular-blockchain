package network

import (
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/sirupsen/logrus"
	"time"
)

type ServerOpts struct {
	Transports []Transport
	BlockTime  time.Duration
	PrivateKey *crypto.PrivateKey
}

type Server struct {
	ServerOpts

	blockTime   time.Duration
	memPool     *TxPool
	isValidator bool
	messageCh   chan Message
	quitCh      chan struct{}
}

func NewServer(opts ServerOpts) *Server {
	if opts.BlockTime == time.Duration(0) {
		timerUnit := config.GetIntEnvVar("PARAMETER_BLOCK_TIME_UNIT")
		timerDuration := config.GetIntEnvVar("PARAMETER_BLOCK_TIME_DURATION")
		timer := timerUnit * timerDuration

		logrus.WithFields(logrus.Fields{
			"unit":     timerUnit,
			"duration": timerDuration,
			"timer":    timer,
		}).Info("set default block time")

		opts.BlockTime = time.Duration(timer)
	}

	return &Server{
		ServerOpts:  opts,
		blockTime:   opts.BlockTime,
		memPool:     NewTxPool(),
		isValidator: opts.PrivateKey != nil,
		messageCh:   make(chan Message),
		quitCh:      make(chan struct{}, 1),
	}
}

func (s *Server) Start() {
	s.initTransports()

	ticker := time.NewTicker(s.blockTime)

free:
	for {
		select {
		case msg := <-s.messageCh:
			fmt.Printf("%+v\n", msg)
		case <-s.quitCh:
			break free
		case <-ticker.C:
			if s.isValidator {
				_ = s.createNewBlock()
			}
		}
	}

	fmt.Println("Server shutdown")
}

func (s *Server) handleTransaction(tx *core.Transaction) error {
	if err := tx.Verify(); err != nil {
		return err
	}

	hash := tx.Hash(core.TxHasher{})

	if s.memPool.Has(hash) {
		logrus.WithFields(logrus.Fields{
			"hash": hash,
		}).Info("transaction already in mempool")

		return nil
	}

	logrus.WithFields(logrus.Fields{
		"hash": hash,
	}).Info("adding new tx to the mempool")

	return s.memPool.Add(tx)
}

func (s *Server) createNewBlock() error {
	fmt.Println("creating a new block")

	return nil
}

func (s *Server) initTransports() {
	for _, tr := range s.Transports {
		go func(tr Transport) {
			for rpc := range tr.Consume() {
				s.messageCh <- rpc
			}
		}(tr)
	}
}
