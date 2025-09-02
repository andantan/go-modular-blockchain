package network

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type ChainParameter struct {
	BlockTime   time.Duration
	MaxPoolSize int
}

func NewChainParameter(BlockTime time.Duration, MaxPoolSize int) *ChainParameter {
	if MaxPoolSize <= 0 {
		panic("MaxPoolSize must be greater than 0")
	}

	return &ChainParameter{
		BlockTime:   BlockTime,
		MaxPoolSize: MaxPoolSize,
	}
}

type BlockchainStatusType byte

const (
	BlockchainStatusTypeBootstrap   BlockchainStatusType = 0x00
	BlockchainStatusTypeSyncing     BlockchainStatusType = 0x01
	BlockchainStatusTypeOnline      BlockchainStatusType = 0x02
	BlockchainStatusTypeInConsensus BlockchainStatusType = 0x03
	BlockchainStatusTypeOffline     BlockchainStatusType = 0x04
)

func (s BlockchainStatusType) String() string {
	switch s {
	case BlockchainStatusTypeBootstrap:
		return "BOOTSTRAPPING"
	case BlockchainStatusTypeSyncing:
		return "SYNCING"
	case BlockchainStatusTypeOnline:
		return "ONLINE"
	case BlockchainStatusTypeInConsensus:
		return "IN_CONSENSUS"
	case BlockchainStatusTypeOffline:
		return "OFFLINE"
	default:
		return "UNKNOWN"
	}
}

type ServerStatus struct {
	ResponseStatus

	Addr net.Addr
}

type ServerOpts struct {
	Logger log.Logger

	ID           string
	ListenAddr   string
	MaxPeers     int
	Flooding     bool
	GossipFactor int
	RemoteSeeds  []string

	Tester   bool
	BlockDir string

	RawMessageDecodeFunc RawMessageDecodeFunc
	MessageProcessFunc   MessageProcessFunc
}

func NewServerOpts(ID string, ListenAddr string) *ServerOpts {
	return &ServerOpts{
		ID:         ID,
		ListenAddr: ListenAddr,
	}
}

func (opts *ServerOpts) WithLogger(logger log.Logger) *ServerOpts {
	opts.Logger = logger
	return opts
}

func (opts *ServerOpts) WithSeeds(seeds []string) *ServerOpts {
	opts.RemoteSeeds = seeds
	return opts
}

func (opts *ServerOpts) WithMaxPeers(maxPeers int) *ServerOpts {
	if maxPeers <= 0 {
		panic("MaxPeers must be greater than 0")
	}

	opts.MaxPeers = maxPeers
	return opts
}

func (opts *ServerOpts) WithFlooding() *ServerOpts {
	opts.Flooding = true
	return opts
}

func (opts *ServerOpts) WithGossipFactor(factor int) *ServerOpts {
	if factor <= 0 {
		panic("factor must be greater than 0")
	}

	opts.GossipFactor = factor
	return opts
}

func (opts *ServerOpts) WithTester() *ServerOpts {
	opts.Tester = true
	return opts
}

func (opts *ServerOpts) WithBlockDir(blockDir string) *ServerOpts {
	if blockDir == "" {
		panic("BlockDir must not be equal \"\"")
	}

	opts.BlockDir = blockDir
	return opts
}

func (opts *ServerOpts) WithRawMessageDecodeFunc(f RawMessageDecodeFunc) *ServerOpts {
	if opts.RawMessageDecodeFunc == nil {
		panic("nil RawMessageDecodeFunc")
	}

	opts.RawMessageDecodeFunc = f

	return opts
}

func (opts *ServerOpts) WithDecodedMessageProcessFunc(p MessageProcessFunc) *ServerOpts {
	if p == nil {
		panic("nil DecodedMessageProcessor")
	}

	opts.MessageProcessFunc = p

	return opts
}

type Server struct {
	ServerOpts
	ChainParameter

	node    Node
	peerMap *types.SyncMap[net.Addr, Peer]

	chain   *core.Blockchain
	memPool *core.Mempool

	statusLock sync.RWMutex
	status     BlockchainStatusType

	rawMessageCh chan RawMessage
	peerCh       chan Peer
	statCh       chan *ServerStatus
	quitCh       chan struct{}
}

func NewServer(opts ServerOpts, chainParams ChainParameter) (*Server, error) {
	if chainParams.MaxPoolSize <= 0 {
		panic("MaxPoolSize must be greater than zero")
	}

	if chainParams.BlockTime == time.Duration(0) {
		panic("BlockTime must be greater than zero")
	}

	if opts.Logger == nil {
		opts.Logger = config.LoggerWithPrefixes("SERVER", "ID", opts.ID)
	}

	if opts.MaxPeers == 0 {
		opts.MaxPeers = 1 << 3
	}

	if !opts.Flooding && opts.GossipFactor == 0 {
		opts.GossipFactor = (opts.MaxPeers * 2) / 3

		if opts.GossipFactor == 0 && opts.MaxPeers > 1 {
			opts.GossipFactor = 1
		}
	}

	chain, err := core.NewBlockchain(opts.BlockDir)

	if err != nil {
		var syncErr *core.BlockSyncingError

		if errors.As(err, &syncErr) {
			fmt.Printf("%+v\n", syncErr)
			panic("TODO: syncing blocks")
		}

		return nil, err
	}

	peerCh := make(chan Peer)

	s := &Server{
		ServerOpts:     opts,
		ChainParameter: chainParams,
		node:           NewTCPNode(opts.ListenAddr, peerCh),
		peerMap:        types.NewSyncMap[net.Addr, Peer](),
		chain:          chain,
		memPool:        core.NewMemPool(chainParams.MaxPoolSize),
		rawMessageCh:   make(chan RawMessage),
		peerCh:         peerCh,
		statCh:         make(chan *ServerStatus),
		quitCh:         make(chan struct{}),
	}

	if opts.RawMessageDecodeFunc == nil {
		s.RawMessageDecodeFunc = s.DefaultRawMessageDecodeFunc
	}

	if opts.MessageProcessFunc == nil {
		s.MessageProcessFunc = s.DefaultMessageProcessFunc
	}

	_ = s.Logger.Log("addr", s.ListenAddr, "msg", "Server initialized")

	return s, nil
}

func (s *Server) UpgradeToProposer(privKey crypto.PrivateKey) (*Server, error) {
	proposer := core.NewBlockPropoesr(s.chain, s.memPool, privKey)
	_ = s.chain.WithProposer(proposer)

	return s, nil
}

func (s *Server) Start() error {
	s.setStatus(BlockchainStatusTypeBootstrap)

	if err := s.node.Listen(); err != nil {
		return err
	}

	_ = s.Logger.Log("msg", "server started, listening on", "addr", s.ListenAddr)

	s.setStatus(BlockchainStatusTypeSyncing)

	go s.syncChain()
	go s.bootstrapNetwork()

	if s.chain.IsProposer() {
		go s.proposerLoop()
	}

	s.loop()

	return nil
}

func (s *Server) Shutdown(wg *sync.WaitGroup) {
	defer wg.Done()
	close(s.quitCh)
}

func (s *Server) setStatus(status BlockchainStatusType) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()

	s.status = status
	_ = s.Logger.Log("msg", "status updated", "status", status.String())
}

func (s *Server) DefaultRawMessageDecodeFunc(rm RawMessage) (*DecodedMessage, error) {
	msg := new(Message)

	if err := gob.NewDecoder(rm.Payload).Decode(&msg); err != nil {
		return nil, fmt.Errorf("failed to decode message from %s: %s", rm.From, err)
	}

	switch msg.Type {
	case MessageTypeNewTx:
		tx := new(core.Transaction)

		if err := tx.UnMarshall(bytes.NewReader(msg.Data)); err != nil {
			return nil, err
		}

		return &DecodedMessage{
			From: rm.From,
			Data: tx,
		}, nil

	case MessageTypeNewBlock:
		block := new(core.Block)

		if err := block.UnMarshall(bytes.NewReader(msg.Data)); err != nil {
			return nil, err
		}

		return &DecodedMessage{
			From: rm.From,
			Data: block,
		}, nil

	case MessageTypeReqStatus:
		req := new(RequestStatus)

		if err := gob.NewDecoder(bytes.NewReader(msg.Data)).Decode(req); err != nil {
			return nil, err
		}

		return &DecodedMessage{
			From: rm.From,
			Data: req,
		}, nil

	case MessageTypeResStatus:
		res := new(ResponseStatus)

		if err := gob.NewDecoder(bytes.NewReader(msg.Data)).Decode(res); err != nil {
			return nil, err
		}

		return &DecodedMessage{
			From: rm.From,
			Data: res,
		}, nil

	case MessageTypeReqBlock:
		req := new(RequestBlocks)

		if err := gob.NewDecoder(bytes.NewReader(msg.Data)).Decode(req); err != nil {
			return nil, err
		}

		return &DecodedMessage{
			From: rm.From,
			Data: req,
		}, nil

	case MessageTypeResBlock:
		res := new(ResponseBlocks)

		if err := gob.NewDecoder(bytes.NewReader(msg.Data)).Decode(res); err != nil {
			return nil, err
		}

		return &DecodedMessage{
			From: rm.From,
			Data: res,
		}, nil
	}

	return nil, nil
}

func (s *Server) DefaultMessageProcessFunc(msg *DecodedMessage) error {
	switch t := msg.Data.(type) {
	case *core.Transaction:
		return s.processTransaction(msg.From, t)

	case *core.Block:
		return s.processBlock(msg.From, t)

	case *RequestStatus:
		return s.processRequestStatus(msg.From, t)

	case *ResponseStatus:
		return s.processResponseStatus(msg.From, t)

	case *RequestBlocks:
		return s.processRequestBlocks(msg.From, t)

	case *ResponseBlocks:
		return s.processResponseBlocks(msg.From, t)

	default:
		return fmt.Errorf("unknown message type in process func: %T", t)
	}
}

func (s *Server) bootstrapNetwork() {
	if len(s.RemoteSeeds) == 0 {
		return
	}

	_ = s.Logger.Log("msg", "connecting to seed nodes", "seeds-length", len(s.RemoteSeeds))

	for _, addr := range s.RemoteSeeds {
		if addr == "" || s.ListenAddr == addr {
			continue
		}

		go func(addr string) {
			if err := s.node.Connect(addr); err != nil {
				_ = s.Logger.Log("error", "failed to connect to seed", "to", addr, "err", err)
			}
		}(addr)
	}
}

func (s *Server) loop() {
	if s.Tester {
		go s.testFunc()
	}

	statusTicker := time.NewTicker(5 * time.Second)

	_ = s.Logger.Log(
		"connected-peer", s.peerMap.Len(),
		"blockchain-height", s.chain.Height(),
	)

frontier:
	for {
		select {
		case <-statusTicker.C:
			_ = s.Logger.Log(
				"connected-peer", s.peerMap.Len(),
				"blockchain-height", s.chain.Height(),
				"status", s.status.String(),
			)

		case rawMessage := <-s.rawMessageCh:
			var (
				msg *DecodedMessage
				err error
			)

			if msg, err = s.RawMessageDecodeFunc(rawMessage); err != nil {
				_ = s.Logger.Log("msg", "message decode error", "error", err)
				continue
			}

			if err = s.MessageProcessFunc(msg); err != nil {
				_ = s.Logger.Log("msg", "message process error", "error", err)
				continue
			}

		case newPeer := <-s.peerCh:
			if err := s.processNewPeer(newPeer); err != nil {
				_ = newPeer.Close()
			}

		case <-s.quitCh:
			_ = s.node.Stop()

			for _, peer := range s.peerMap.Iterator() {
				s.closePeer(peer)
				_ = s.Logger.Log("msg", "shutting down peer", "addr", peer.Who())
			}

			close(s.peerCh)
			close(s.statCh)
			close(s.rawMessageCh)

			break frontier
		}
	}
}

func (s *Server) syncChain() {
	defer func() {
		s.setStatus(BlockchainStatusTypeOnline)
	}()

	s.setStatus(BlockchainStatusTypeSyncing)

	ticker := time.NewTicker(3 * time.Second)
	statMap := make(map[net.Addr]*ServerStatus)

sync:
	for {
		select {
		case stat := <-s.statCh:
			statMap[stat.Addr] = stat

		case <-ticker.C:
			if s.peerMap.Len() == 0 {
				return
			}

			highestHeight := uint64(0)
			var highestStatus *ServerStatus

			for _, stat := range statMap {
				if highestHeight <= stat.Height {
					highestHeight = stat.Height
					highestStatus = stat
				}
			}

			if s.chain.Height() == highestHeight {
				break sync
			}

			if s.chain.Height() < highestHeight {
				highestPeer, _ := s.peerMap.Get(highestStatus.Addr)

				_ = s.requestBlocks(highestPeer, s.chain.Height()+1, highestHeight)
			}

			for _, stat := range statMap {
				go func() {
					peer, ok := s.peerMap.Get(stat.Addr)
					fmt.Printf("%+v\n", peer)

					if !ok {
						return
					}

					req := new(RequestStatus)
					buf := new(bytes.Buffer)

					if err := gob.NewEncoder(buf).Encode(req); err != nil {
						return
					}

					msg := NewMessage(MessageTypeReqStatus, buf.Bytes())

					_ = peer.Send(msg.Bytes())

					//if err := ; err != nil {
					//	fmt.Println(err)
					//	return
					//}
				}()
			}
		}
	}
}

func (s *Server) testFunc() {
	testTicker := time.NewTicker(200 * time.Millisecond)

	for {
		go func() {
			key, _ := crypto.GeneratePrivateKey()
			tx := core.NewTransaction([]byte("Hello world"))
			_ = tx.Sign(key)

			_ = s.broadcast(tx)
		}()

		<-testTicker.C
	}
}

func (s *Server) broadcast(o any) error {
	var msgType MessageType

	buf := new(bytes.Buffer)

	switch t := o.(type) {
	case *core.Transaction:
		_ = t.Marshall(buf)
		msgType = MessageTypeNewTx
	case *core.Block:
		_ = t.Marshall(buf)
		msgType = MessageTypeNewBlock
	}

	msg := NewMessage(msgType, buf.Bytes())

	if s.Flooding {
		return s.flood(msg.Bytes())
	} else {
		return s.gossip(msg.Bytes())
	}
}

func (s *Server) flood(payload []byte) error {
	for _, peer := range s.peerMap.Iterator() {
		go func() {
			_ = s.send(peer, payload)
		}()
	}

	return nil
}

func (s *Server) gossip(payload []byte) error {
	if s.peerMap.Len() <= s.GossipFactor {
		return s.flood(payload)
	}

	peers := s.peerMap.Values()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(peers), func(i, j int) {
		peers[i], peers[j] = peers[j], peers[i]
	})

	for i := 0; i < s.GossipFactor; i++ {
		go func() {
			_ = s.send(peers[i], payload)
		}()
	}

	return nil
}

func (s *Server) send(peer Peer, payload []byte) error {
	if err := peer.Send(payload); err != nil {
		_ = s.Logger.Log(
			"msg", fmt.Sprintf("peer send error => %s", peer.Who()),
			"netAddr", peer.Who(),
			"err", err,
		)

		return err
	}

	return nil
}

func (s *Server) closePeer(peer Peer) {
	_ = peer.Close()
	s.peerMap.Remove(peer.Who())
}

func (s *Server) proposerLoop() {
	ticker := time.NewTicker(s.BlockTime)

	_ = s.Logger.Log("msg", "starting proposer loop")

	for {
		<-ticker.C

		if err := s.createNewBlock(); err != nil {
			_ = s.Logger.Log("msg", "error creating new block", "err", err)
		}
	}
}

func (s *Server) createNewBlock() error {
	if s.memPool.IsNilPending() {
		return nil
	}

	block, err := s.chain.CreateBlock()

	if err != nil {
		return err
	}

	go func() {
		if err = s.broadcast(block); err != nil {
			_ = s.Logger.Log("msg", "broadcast-block-error", "error", err)
		}
	}()

	return nil
}

func (s *Server) processNewPeer(peer Peer) error {
	if s.peerMap.Len() >= s.MaxPeers {
		_ = s.Logger.Log(
			"msg", "max peers reached",
			"from", peer.Who(),
			"max_peers", s.MaxPeers,
		)

		return fmt.Errorf("max peers reached")
	}

	s.peerMap.Put(peer.Who(), peer)

	go func() {
		if err := peer.Read(s.rawMessageCh); err != nil {
			s.peerMap.Remove(peer.Who())
		}
	}()

	_ = s.Logger.Log(
		"msg", "new peer connected",
		"addr", peer.Who(),
		"total_peers", s.peerMap.Len(),
	)

	if err := s.responseStatus(peer); err != nil {
		return err
	}

	return nil
}

func (s *Server) processTransaction(_ net.Addr, tx *core.Transaction) error {
	if s.status == BlockchainStatusTypeOnline {
		if s.memPool.Contains(tx) {
			return nil
		}

		if err := tx.Verify(); err != nil {
			return err
		}

		tx.SetFirstSeen()
		s.memPool.Add(tx)

		go func() {
			if err := s.broadcast(tx); err != nil {
				_ = s.Logger.Log("msg", "broadcast-tx-error", "error", err)
			}
		}()
	}
	return nil
}

func (s *Server) processBlock(_ net.Addr, block *core.Block) error {
	if err := s.chain.AddBlock(block); err != nil {
		if !errors.Is(err, core.ErrBlockKnown) {
			return err
		} else {
			return nil
		}
	}

	if s.status == BlockchainStatusTypeOnline {
		s.memPool.PrunePending(block.Transactions)

		go func() {
			if err := s.broadcast(block); err != nil {
				_ = s.Logger.Log("msg", "broadcast-block-error", "error", err)
			}
		}()
	}

	return nil
}

func (s *Server) processRequestStatus(from net.Addr, _ *RequestStatus) error {
	peer, ok := s.peerMap.Get(from)

	if !ok {
		return fmt.Errorf("peer (%s) not found", from.String())
	}

	return s.responseStatus(peer)
}

func (s *Server) processResponseStatus(from net.Addr, res *ResponseStatus) error {
	if s.status == BlockchainStatusTypeSyncing {
		stat := &ServerStatus{
			ResponseStatus: *res,
			Addr:           from,
		}

		// select 문을 사용하여 비동기로 채널에 전송 시도
		select {
		case s.statCh <- stat:
			// 성공적으로 전송됨
		default:
			// statCh가 꽉 찼거나 받는 쪽이 준비 안 됨.
			// 이 경우 메시지를 버리거나, 에러 로그를 남길 수 있습니다.
			_ = s.Logger.Log("msg", "statCh is blocked, dropping status message", "from", from)
		}

		return nil
	}

	if s.chain.Height() < res.Height {
		go s.syncChain()
	}

	//genesisHeader, _ := s.chain.GetHeader(0)
	//genesisBlockHash := core.BlockHasher{}.Hash(genesisHeader)
	//
	//if res.GenesisBlockHash != genesisBlockHash {
	//	panic("invalid genesis block hash provided")
	//}
	//
	//if s.chain.Height() < res.Height {
	//	peer, ok := s.peerMap.Get(from)
	//
	//	if !ok {
	//		return fmt.Errorf("peer (%s) not found", from.String())
	//	}
	//
	//	if err := s.requestBlocks(peer, s.chain.Height()+1, res.Height); err != nil {
	//		return err
	//	}
	//}

	return nil
}

func (s *Server) processRequestBlocks(from net.Addr, req *RequestBlocks) error {
	_ = s.Logger.Log(
		"msg", "process request blocks",
		"From", req.From,
		"To", req.To,
		"BatchSize", req.BatchSize,
	)

	peer, ok := s.peerMap.Get(from)

	if !ok {
		return fmt.Errorf("peer (%s) not found", from.String())
	}

	ourHeight := s.chain.Height()
	endHeight := req.From + req.BatchSize - 1

	if endHeight > req.To {
		endHeight = req.To
	}
	if endHeight > ourHeight {
		endHeight = ourHeight
	}

	blocksToSend := make([]*core.Block, 0)

	if req.From <= endHeight {
		for i := req.From; i <= endHeight; i++ {
			block, err := s.chain.GetBlockByHeight(i)

			if err != nil {
				return fmt.Errorf("failed to get block %d: %w", i, err)
			}

			blocksToSend = append(blocksToSend, block)
		}
	}

	buf := new(bytes.Buffer)
	res := &ResponseBlocks{
		Blocks: blocksToSend,
	}

	if err := gob.NewEncoder(buf).Encode(res); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeResBlock, buf.Bytes())

	return peer.Send(msg.Bytes())
}

func (s *Server) processResponseBlocks(from net.Addr, res *ResponseBlocks) error {
	for _, block := range res.Blocks {
		_ = s.processBlock(from, block)
	}

	return nil
}

func (s *Server) responseStatus(peer Peer) error {
	buf := new(bytes.Buffer)
	height := s.chain.Height()
	genesis, _ := s.chain.GetHeader(0)
	current, _ := s.chain.GetHeader(height)
	stat := &ResponseStatus{
		ID:               s.ID,
		Version:          s.chain.Version(),
		Height:           height,
		Status:           s.status,
		GenesisBlockHash: core.BlockHasher{}.Hash(genesis),
		CurrentBlockHash: core.BlockHasher{}.Hash(current),
	}

	if err := gob.NewEncoder(buf).Encode(stat); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeResStatus, buf.Bytes())

	if err := peer.Send(msg.Bytes()); err != nil {
		_ = s.Logger.Log("msg", "request status message send error. shutting down peer", "addr", peer.Who())
		return err
	}

	return nil
}

func (s *Server) requestBlocks(peer Peer, from uint64, to uint64) error {
	buf := new(bytes.Buffer)
	req := &RequestBlocks{
		From:      from,
		To:        to,
		BatchSize: 8,
	}

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeReqBlock, buf.Bytes())

	if err := peer.Send(msg.Bytes()); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	return nil
}
