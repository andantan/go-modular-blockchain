package network

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/go-kit/log"
	"os"
	"time"
)

type ServerOpts struct {
	ID            string
	Logger        log.Logger
	Transport     Transport
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

	if opts.Logger == nil {
		opts.Logger = log.NewLogfmtLogger(os.Stdout)
		opts.Logger = log.With(opts.Logger, "addr", opts.Transport.Addr())
	}

	txMaxLength := config.GetIntEnvVar("PARAMETER_TX_MAX_LENGTH")
	chain, err := core.NewBlockchain(opts.Logger, core.GetGenesisBlock())

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

	_ = s.Logger.Log(
		"msg", "new server created",
		"isValidator", s.isValidator,
		"blockTime", opts.BlockTime,
	)

	if s.isValidator {
		go s.validatorLoop()
	}

	s.bootstrapNodes()

	return s, nil
}

func (s *Server) Start() {
	s.initTransports()

free:
	for {
		select {
		case rpc := <-s.rpcCh:
			msg, err := s.RPCDecodeFunc(rpc)

			if err != nil {
				_ = s.Logger.Log(err)
			}

			if err := s.RPCProcesor.ProcessMessage(msg); err != nil {
				if err != core.ErrBlockKnown {
					// fmt.Printf("%+v\n", err)
					_ = s.Logger.Log("err", err)
				}
			}
		case <-s.quitCh:
			break free
		}
	}

	_ = s.Logger.Log("msg", "server is shutting down")
}

func (s *Server) bootstrapNodes() {
	for _, tr := range s.Transports {
		if s.Transport.Addr() != tr.Addr() {
			if err := s.Transport.Connect(tr); err != nil {
				_ = s.Logger.Log("msg", "cound not connect to remote node", "err", err)
			}

			_ = s.Logger.Log("msg", "connected to remote node", "we", s.Transport.Addr(), "addr", tr.Addr())
			_ = s.Logger.Log("msg", "sending message", "we", s.Transport.Addr(), "addr", tr.Addr())

			if err := s.sendGetStatusMessage(tr); err != nil {
				_ = s.Logger.Log("msg", "cound not send status message", "err", err)
			}
		}
	}
}

func (s *Server) validatorLoop() {
	ticker := time.NewTicker(s.BlockTime)

	_ = s.Logger.Log("msg", "starting validator loop")

	for {
		<-ticker.C

		if err := s.createNewBlock(); err != nil {
			_ = s.Logger.Log("msg", "error creating new block", "err", err)
		}
	}
}

func (s *Server) ProcessMessage(msg *DecodedMessage) error {
	switch t := msg.Data.(type) {
	case *core.Transaction:
		return s.processTransaction(t)

	case *core.Block:
		//_ = s.Logger.Log(
		//	"msg", "received block",
		//	"ourHeight", s.chain.Height(),
		//	"receivedHeight", t.Height,
		//	"from", msg.From,
		//)

		return s.processBlock(t)

	case *GetBlocksMessage:
		return s.processGetBlocksMessage(msg.From, t)

	case *StatusMessage:
		return s.processStatusMessage(msg.From, t)

	case *GetStatusMessage:
		return s.processGetStatusMessage(msg.From, t)
	}

	return nil
}

func (s *Server) processGetBlocksMessage(from NetAddr, data *GetBlocksMessage) error {
	panic("HERE !!!!!!!")

	fmt.Printf("got get blocks message => %+v\n", data)

	return nil
}

func (s *Server) sendGetStatusMessage(tr Transport) error {
	var (
		getStatusMsg = new(GetStatusMessage)
		buf          = new(bytes.Buffer)
	)

	if err := gob.NewEncoder(buf).Encode(getStatusMsg); err != nil {
		return err
	}

	msg := NewMessage(MessageTypeGetStatus, buf.Bytes())

	if err := s.Transport.SendMessage(tr.Addr(), msg.Bytes()); err != nil {
		return err
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
	hash := tx.Hash(core.TxHasher{})

	if s.memPool.Contains(hash) {
		return nil
	}

	if err := tx.Verify(); err != nil {
		return err
	}

	tx.SetFirstSeen(uint64(time.Now().UnixNano()))

	go func() {
		if err := s.broadcastTx(tx); err != nil {
			_ = s.Logger.Log(err)
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

func (s *Server) processStatusMessage(from NetAddr, data *StatusMessage) error {
	if data.CurrentHeight <= s.chain.Height() {
		_ = s.Logger.Log(
			"msg", "cannot sync blockHeight to low",
			"addr", from,
			"theirHeight", data.CurrentHeight,
			"ourHeight", s.chain.Height(),
		)

		return nil
	}

	getBlockMessage := &GetBlocksMessage{
		From: s.chain.Height(),
		To:   0,
	}
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(getBlockMessage); err != nil {
		return nil
	}

	msg := NewMessage(MessageTypeGetBlocks, buf.Bytes())

	return s.Transport.SendMessage(from, msg.Bytes())
}

func (s *Server) processGetStatusMessage(from NetAddr, data *GetStatusMessage) error {
	_ = s.Logger.Log(
		"msg", "received GetStatus msg",
		"from", from,
		"data", fmt.Sprintf("%+v", data),
	)

	statusMessage := &StatusMessage{
		ID:            s.ID,
		CurrentHeight: s.chain.Height(),
	}

	buf := &bytes.Buffer{}

	if err := gob.NewEncoder(buf).Encode(statusMessage); err != nil {
		return err
	}

	msg := NewMessage(MessageTypeStatus, buf.Bytes())

	return s.Transport.SendMessage(from, msg.Bytes())
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
		if s.Transport.Addr() != tr.Addr() {
			go func(tr Transport) {
				for rpc := range tr.Consume() {
					s.rpcCh <- rpc
				}
			}(tr)
		}
		//go func(tr Transport) {
		//	for rpc := range tr.Consume() {
		//		s.rpcCh <- rpc
		//	}
		//}(tr)
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
