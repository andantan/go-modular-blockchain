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
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxHeadersPerRequest = 512
	maxBlocksPerRequest  = 16
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
	BlockchainStatusTypeBootstrap BlockchainStatusType = iota
	BlockchainStatusTypeSyncing
	BlockchainStatusTypeOnline
	BlockchainStatusTypeForked
	BlockchainStatusTypeInConsensus
	BlockchainStatusTypeOffline
)

func (s BlockchainStatusType) String() string {
	switch s {
	case BlockchainStatusTypeBootstrap:
		return "BOOTSTRAP"
	case BlockchainStatusTypeSyncing:
		return "SYNCING"
	case BlockchainStatusTypeOnline:
		return "ONLINE"
	case BlockchainStatusTypeForked:
		return "FORKED"
	case BlockchainStatusTypeInConsensus:
		return "IN_CONSENSUS"
	case BlockchainStatusTypeOffline:
		return "OFFLINE"
	default:
		return "UNKNOWN"
	}
}

type ServerStatus struct {
	ID               string
	ListenAddr       string
	Status           BlockchainStatusType
	Height           uint64
	GenesisBlockHash types.Hash
	CurrentBlockHash types.Hash
}

type ServerOpts struct {
	Logger log.Logger

	ID         string
	ListenAddr string

	DNS          PeerDNS
	Seeds        []string
	MaxPeers     int
	Flooding     bool
	GossipFactor int

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

func (opts *ServerOpts) WithDNS(dns PeerDNS) *ServerOpts {
	opts.DNS = dns
	return opts
}

func (opts *ServerOpts) WithSeeds(seeds []string) *ServerOpts {
	opts.Seeds = seeds
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
	peerMap *types.SyncMap[string, Peer]

	chain    *core.Blockchain
	memPool  *core.Mempool
	proposer core.Proposer

	statusLock sync.RWMutex
	status     BlockchainStatusType

	rawMessageCh chan RawMessage
	newPeerCh    chan Peer
	delPeerCh    chan Peer
	quitCh       chan struct{}

	syncStatCh chan *ServerStatus
	syncQuitCh chan struct{}
}

func NewServer(opts ServerOpts, chainParams ChainParameter) (*Server, error) {
	if chainParams.MaxPoolSize <= 0 {
		panic("MaxPoolSize must be greater than zero")
	}

	if chainParams.BlockTime == time.Duration(0) {
		panic("BlockTime must be greater than zero")
	}

	if opts.DNS == nil {
		panic("DNS must not be nil")
	}

	if opts.Logger == nil {
		opts.Logger = config.LoggerWithPrefixes("server", "id", opts.ID, "addr", opts.ListenAddr)
	}

	if opts.MaxPeers == 0 {
		opts.MaxPeers = 4
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
			panic("TODO: syncing blocks")
		}

		return nil, err
	}

	messageCh := make(chan RawMessage)
	newPeerCh := make(chan Peer)
	delPeerCh := make(chan Peer)

	node := NewTCPNode(opts.ID, opts.ListenAddr, messageCh, newPeerCh, delPeerCh)

	s := &Server{
		ServerOpts:     opts,
		ChainParameter: chainParams,
		node:           node,
		peerMap:        types.NewSyncMap[string, Peer](),
		chain:          chain,
		memPool:        core.NewMemPool(chainParams.MaxPoolSize),
		rawMessageCh:   messageCh,
		newPeerCh:      newPeerCh,
		delPeerCh:      delPeerCh,
		quitCh:         make(chan struct{}),
		syncStatCh:     nil,
		syncQuitCh:     nil,
	}

	if opts.RawMessageDecodeFunc == nil {
		s.RawMessageDecodeFunc = s.DefaultRawMessageDecodeFunc
	}

	if opts.MessageProcessFunc == nil {
		s.MessageProcessFunc = s.DefaultMessageProcessFunc
	}

	return s, nil
}

func (s *Server) UpgradeToProposer(privKey crypto.PrivateKey) (*Server, error) {
	s.proposer = core.NewBlockPropoesr(s.chain, s.memPool, privKey)

	_ = s.Logger.Log(
		"event", "upgraded_to_proposer",
		"proposer_address", privKey.PublicKey().Address().String(),
	)

	return s, nil
}

func (s *Server) Start() error {
	s.setStatus(BlockchainStatusTypeBootstrap)

	if err := s.node.Listen(); err != nil {
		return err
	}

	s.mustRegisterDNS()

	if s.proposer != nil {
		go s.proposerLoop()
	}

	go s.manageConnections()
	go s.statusLogLoop()
	go s.heartbeatLoop()

	s.loop()

	return nil
}

func (s *Server) Shutdown(wg *sync.WaitGroup) {
	defer wg.Done()
	close(s.quitCh)
}

func (s *Server) isOnline() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()

	return s.status == BlockchainStatusTypeOnline
}

func (s *Server) isSyncing() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()

	return s.status == BlockchainStatusTypeSyncing
}

func (s *Server) isForked() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()

	return s.status == BlockchainStatusTypeForked
}

func (s *Server) DefaultRawMessageDecodeFunc(rm RawMessage) (*DecodedMessage, error) {
	msg := new(Message)

	if err := gob.NewDecoder(rm.Payload).Decode(&msg); err != nil {
		_ = s.Logger.Log("event", "top_level_message_decode_error", "from", rm.From, "error", err)

		return nil, fmt.Errorf("failed to decode message from %s: %s", rm.From, err)
	}

	var o any

	switch msg.Type {
	case MessageTypeNewTx:
		o = new(core.Transaction)
	case MessageTypeNewBlock:
		o = new(core.Block)
	case MessageTypeReqStatus:
		o = new(RequestStatus)
	case MessageTypeResStatus:
		o = new(ResponseStatus)
	case MessageTypeReqHeaders:
		o = new(RequestHeaders)
	case MessageTypeResHeaders:
		o = new(ResponseHeaders)
	case MessageTypeReqBlocks:
		o = new(RequestBlocks)
	case MessageTypeResBlocks:
		o = new(ResponseBlocks)
	default:
		// TODO: WHAT THE FUCK IS THIS CASE
		return nil, fmt.Errorf("unknown message type: %s", msg.Type)
	}

	if err := gob.NewDecoder(bytes.NewReader(msg.Data)).Decode(o); err != nil {
		return nil, err
	}

	return &DecodedMessage{
		From: rm.From,
		Data: o,
	}, nil
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
	case *RequestHeaders:
		return s.processRequestHeaders(msg.From, t)
	case *ResponseHeaders:
		return s.processResponseHeaders(msg.From, t)
	case *RequestBlocks:
		return s.processRequestBlocks(msg.From, t)
	case *ResponseBlocks:
		return s.processResponseBlocks(msg.From, t)
	default:
		return fmt.Errorf("unknown message type in process func: %T", t)
	}
}

func (s *Server) setStatus(status BlockchainStatusType) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()

	s.status = status
	_ = s.Logger.Log("event", "update_status", "status", status.String())
}

func (s *Server) mustRegisterDNS() {
	_ = s.Logger.Log("event", "register_DNS")

	stat := &PeerStatus{
		ID:             s.ID,
		Addr:           s.ListenAddr,
		Connections:    0,
		MaxConnections: 0,
		Height:         0,
		Validator:      s.chain.IsValidator(),
		Proposer:       s.proposer != nil,
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	err := s.DNS.Register(stat, wg)
	wg.Wait()

	if err != nil {
		panic("failed to register DNS: " + err.Error())
	}
}

func (s *Server) discoverPeers() {
	currentPeerCount := s.peerMap.Len()
	needed := s.MaxPeers - currentPeerCount

	if needed <= 0 {
		return
	}

	stat := &PeerStatus{
		ID:             s.ID,
		Addr:           s.ListenAddr,
		Connections:    0,
		MaxConnections: 0,
		Height:         0,
		Validator:      s.chain.IsValidator(),
		Proposer:       s.proposer != nil,
	}

	peerCandidates, err := s.DNS.DiscoverPeers(stat, 16)

	if err != nil {
		panic(fmt.Sprintf("failed to discover peers: %s", err))
	}

	if len(peerCandidates) == 0 {
		return
	}

	connectedPeers := make(map[string]bool)
	connectedIDs := make(map[string]bool)

	for _, peer := range s.peerMap.Iterator() {
		connectedPeers[peer.Addr()] = true
		connectedIDs[peer.ID()] = true
	}

	filteredCandidates := make([]PeerStatus, 0)

	for _, candidate := range peerCandidates {
		if connectedPeers[candidate.Addr] {
			continue
		}

		if connectedIDs[candidate.ID] {
			continue
		}

		if s.ID == candidate.ID {
			continue
		}

		if candidate.Connections < candidate.MaxConnections {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}

	if len(filteredCandidates) == 0 {
		return
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(filteredCandidates), func(i, j int) {
		filteredCandidates[i], filteredCandidates[j] = filteredCandidates[j], filteredCandidates[i]
	})

	sort.Slice(filteredCandidates, func(i, j int) bool {
		return filteredCandidates[i].Connections < filteredCandidates[j].Connections
	})

	_ = s.Logger.Log("event", "new_peer_candidates", "needed", needed, "found_new", len(filteredCandidates))

	connectedCount := 0
	for _, candidate := range filteredCandidates {
		if connectedCount >= needed || s.peerMap.Len() >= s.MaxPeers {
			break
		}

		if err = s.node.Connect(candidate.Addr); err != nil {
			if strings.Contains(err.Error(), "actively refused") {
				return
			}

			connectedCount++

			_ = s.Logger.Log(
				"event", "failed_to_connect_peer",
				"peer-addr", candidate.Addr,
				"error", err,
			)
		}
	}
}

func (s *Server) loop() {
	s.setStatus(BlockchainStatusTypeOnline)

	if s.Tester {
		go s.testFunc()
	}

	_ = s.Logger.Log("event", "server_started", "height", s.chain.Height())

frontier:
	for {
		select {
		case rawMessage := <-s.rawMessageCh:
			var (
				msg *DecodedMessage
				err error
			)

			if msg, err = s.RawMessageDecodeFunc(rawMessage); err != nil {
				_ = s.Logger.Log("event", "message_decode_error", "error", err)
				continue
			}

			if err = s.MessageProcessFunc(msg); err != nil {
				_ = s.Logger.Log("event", "message_process_error", "error", err)
				continue
			}

		case newPeer := <-s.newPeerCh:
			if err := s.processNewPeer(newPeer); err != nil {
				_ = s.Logger.Log("event", "failed_to_process_peer", "peer-id", newPeer.ID(), "peer-addr", newPeer.Addr(), "error", err)
				continue
			}

			s.heartbeat()

		case delPeer := <-s.delPeerCh:
			s.peerMap.Remove(delPeer.Addr())
			s.node.Remove(delPeer)

			s.heartbeat()

		case <-s.quitCh:
			_ = s.Logger.Log("event", "shutdown_signal_received")
			_ = s.node.Stop()
			_ = s.Logger.Log("event", "closing_all_peer", "count", s.peerMap.Len())

			for _, peer := range s.peerMap.Iterator() {
				peer.Close()
			}

			_ = s.Logger.Log("event", "close_server_channels")
			close(s.newPeerCh)
			close(s.delPeerCh)
			close(s.rawMessageCh)

			break frontier
		}
	}
}

func (s *Server) manageConnections() {
	time.Sleep(time.Duration(rand.Intn(60)) * time.Second)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.quitCh:
			return
		case <-ticker.C:
			if s.peerMap.Len() < s.MaxPeers {
				s.discoverPeers()
			} else {
				peers := s.peerMap.Values()
				if len(peers) > 0 {
					randomPeer := peers[rand.Intn(len(peers))]
					_ = s.Logger.Log("event", "churning", "msg", "cycling peer connection", "disconnecting", randomPeer.ID())
					randomPeer.Close()
				}
			}
		}
	}
}

func (s *Server) statusLogLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quitCh:
			return
		case <-ticker.C:
			connStr := fmt.Sprintf("%d/%d", s.peerMap.Len(), s.MaxPeers)
			_ = s.Logger.Log("event", "status_update", "connections", connStr, "chain_height", s.chain.Height(), "status", s.status.String())
		}
	}
}

func (s *Server) heartbeat() {
	stat := &PeerStatus{
		ID:             s.ID,
		Addr:           s.ListenAddr,
		Connections:    uint8(s.peerMap.Len()),
		MaxConnections: uint8(s.MaxPeers),
		Height:         s.chain.Height(),
		Validator:      s.chain.IsValidator(),
		Proposer:       s.proposer != nil,
	}

	if err := s.DNS.Heartbeat(stat); err != nil {
		_ = s.Logger.Log("event", "failed_to_heartbeat", "error", err)
	}
}

func (s *Server) heartbeatLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quitCh:
			return
		case <-ticker.C:
			s.heartbeat()
		}
	}
}

func (s *Server) syncChainLoop(fullSync bool) {
	s.syncStatCh = make(chan *ServerStatus, 16)
	s.syncQuitCh = make(chan struct{})
	s.setStatus(BlockchainStatusTypeSyncing)

	_ = s.Logger.Log("event", "start_sync_chain", "full_sync", fullSync)

	defer func() {
		_ = s.Logger.Log("event", "sync_chain_terminated", "msg", "resetting syncing fields")
		s.setStatus(BlockchainStatusTypeOnline)

		close(s.syncStatCh)
		close(s.syncQuitCh)

		s.syncStatCh = nil
		s.syncQuitCh = nil
	}()

	if !s.memPool.IsNilAll() {
		s.memPool.ClearAll()
	}

	if !s.memPool.IsNilPending() {
		s.memPool.ClearPending()
	}

	if fullSync {
		_ = s.Logger.Log("event", "full_sync_triggered", "msg", "resetting local chain state")

		s.chain.ClearHeader()

		if err := s.chain.ClearStorage(); err != nil {
			_ = s.Logger.Log("event", "clearing_block_storage_failed", "error", err)
			return
		}

		_ = s.chain.AddBlock(core.GetGenesisBlock())
		_ = s.Logger.Log("event", "local_chain_re_initiated", "msg", "local chain reset and genesis block added")
	}

	ticker := time.NewTicker(2 * time.Second)
	onlinePeerMap := make(map[string]*ServerStatus)

sync:
	for {
		select {
		case stat := <-s.syncStatCh:
			_ = s.Logger.Log("event", "received_peer_status", "from", stat.ListenAddr, "height", stat.Height, "status", stat.Status.String())

			genesis, _ := s.chain.GetHeader(0)
			localGenesisBlockHash := core.BlockHasher{}.Hash(genesis)

			if stat.GenesisBlockHash != localGenesisBlockHash {
				_ = s.Logger.Log("event", "different_network", "msg", "peer has different genesis block, disconnecting", "peer", stat.ListenAddr)

				peer, ok := s.peerMap.Get(stat.ListenAddr)

				if ok {
					peer.Close()
				}

				continue
			}

			if stat.Status == BlockchainStatusTypeOnline {
				if stat.Height > s.chain.Height() {
					onlinePeerMap[stat.ListenAddr] = stat
					continue
				}

				localCurrentBlockHeader, _ := s.chain.GetHeader(s.chain.Height())
				localCurrentBlockHash := core.BlockHasher{}.Hash(localCurrentBlockHeader)

				if stat.Height == s.chain.Height() || stat.CurrentBlockHash == localCurrentBlockHash {
					onlinePeerMap[stat.ListenAddr] = stat
					continue
				}
			}

		case <-ticker.C:
			if s.peerMap.Len() == 0 {
				_ = s.Logger.Log("event", "no_peers_connected", "msg", "stopping sync chain")
				break sync
			}

			if len(onlinePeerMap) == 0 {
				_ = s.Logger.Log("event", "no_online_peers", "msg", "requesting status from all")
				if err := s.floodRequestStatus(); err != nil {
					_ = s.Logger.Log("event", "flood_request_status_error", "error", err)
				}

				continue
			}

			highestHeight := s.chain.Height() // will be 0
			var highestPeerListenAddr string

			for l, stat := range onlinePeerMap {
				if highestHeight <= stat.Height {
					highestHeight = stat.Height
					highestPeerListenAddr = l
				}
			}

			if s.chain.Height() >= highestHeight {
				_ = s.Logger.Log("event", "sync_finished", "msg", "chain is fully synced", "height", s.chain.Height())
				break sync
			}

			if s.chain.Height() < highestHeight {
				p, _ := s.peerMap.Get(highestPeerListenAddr)

				_ = s.Logger.Log("event", "request_blocks", "from_height", s.chain.Height()+1, "peer-addr", p.Addr())
				_ = s.requestBlocks(p, s.chain.Height()+1, 16)
			}

			if err := s.floodRequestStatus(); err != nil {
				_ = s.Logger.Log("event", "flood_request_status_error", "error", err)
			}

		case <-s.syncQuitCh:
			_ = s.Logger.Log("event", "canceled_sync_chain")
			break sync

		case <-s.quitCh: // server cut-down signal
			_ = s.Logger.Log("event", "shutdown", "msg", "server shutdown signal received, terminating sync loop")
			break sync
		}
	}
}

func (s *Server) testFunc() {
	ticker := time.NewTicker(4 * time.Second)

	for {
		if s.isOnline() {
			break
		}
		<-ticker.C
	}

	ticker.Stop()

	testTicker := time.NewTicker(1 * time.Second)

	for {
		if s.isOnline() {
			go func() {
				key, _ := crypto.GeneratePrivateKey()
				tx := core.NewTransaction([]byte("Hello world"))
				_ = tx.Sign(key)

				_ = s.broadcast(tx)
			}()
		}

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
	}

	return s.gossip(msg.Bytes())
}

func (s *Server) flood(payload []byte) error {
	for _, peer := range s.peerMap.Iterator() {
		go func(peer Peer) {
			if err := s.send(peer, payload); IsUnrecoverableTCPError(err) {
				if IsUnrecoverableTCPError(err) {
					s.delPeerCh <- peer
				}
			}
		}(peer)
	}

	return nil
}

func (s *Server) floodRequestStatus() error {
	req := new(RequestStatus)
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		_ = s.Logger.Log("event", "encode_request_status_error", "err", err)
		return err
	}

	msg := NewMessage(MessageTypeReqStatus, buf.Bytes())

	return s.flood(msg.Bytes())
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
		go func(peer Peer) {
			if err := s.send(peer, payload); IsUnrecoverableTCPError(err) {
				if IsUnrecoverableTCPError(err) {
					s.delPeerCh <- peer
				}
			}
		}(peers[i])
	}

	return nil
}

func (s *Server) send(peer Peer, payload []byte) error {
	if err := peer.Send(payload); err != nil {
		_ = s.Logger.Log("event", "peer_send_error", "peer_id", peer.ID(), "err", err)
		return err
	}

	return nil
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
		_ = s.Logger.Log("msg", "no pending transactions, skipping block creation")
		return nil
	}

	block, err := s.proposer.CreateBlock()

	if err != nil {
		_ = s.Logger.Log("msg", "failed to create block", "err", err)
		return err
	}

	_ = s.Logger.Log(
		"msg", "new block created successfully",
		"height", block.Height,
		"hash", block.Hash(core.BlockHasher{}).ShortString(10),
		"transactions", len(block.Transactions),
	)

	go func() {
		if err = s.broadcast(block); err != nil {
			_ = s.Logger.Log("msg", "broadcast-block-error", "error", err)
		}
	}()

	return nil
}

func (s *Server) processNewPeer(peer Peer) error {
	if s.peerMap.Len() >= s.MaxPeers {
		peer.Close()
		return fmt.Errorf("max peers reached")
	}

	s.peerMap.Put(peer.Addr(), peer)

	go peer.Read()

	return nil
}

func (s *Server) processTransaction(_ net.Addr, tx *core.Transaction) error {
	if !s.isOnline() {
		return nil
	}

	hash := tx.Hash(core.TxHasher{})

	if s.memPool.Contains(tx) {
		return nil
	}

	// _ = s.Logger.Log("msg", "processing transaction", "tx", tx.Nonce)

	if err := tx.Verify(); err != nil {
		_ = s.Logger.Log("msg", "invalid transaction signature", "hash", hash.ShortString(8), "err", err)
		return nil
	}

	tx.SetFirstSeen()
	s.memPool.Add(tx)

	go func() {
		if err := s.broadcast(tx); err != nil {
			_ = s.Logger.Log("msg", "failed to broadcast transaction", "hash", hash.ShortString(8), "err", err)
		}
	}()

	return nil
}

func (s *Server) processBlock(from net.Addr, block *core.Block) error {
	hash := block.Hash(core.BlockHasher{})

	if err := s.chain.AddBlock(block); err != nil {
		switch {
		case errors.Is(err, core.ErrBlockKnown):
			return nil

		case errors.Is(err, core.ErrFutureBlock):
			if s.isOnline() {
				_ = s.Logger.Log("msg", "future block received, triggering sync", "our_height", s.chain.Height(), "network_height", block.Height)

				go s.syncChainLoop(false)
			}

			return nil

		case errors.Is(err, core.ErrUnknownParent):
			s.setStatus(BlockchainStatusTypeForked)

			_ = s.Logger.Log("msg", "block with unknown parent, fork detected", "our_height", s.chain.Height(), "forked_height", block.Height)

			if peer, ok := s.peerMap.Get(from.String()); ok {
				go func(peer Peer) {
					req := uint64(1)
					if s.chain.Height() > 7 {
						req = s.chain.Height() - 7
					}
					_ = s.requestHeaders(peer, req, 8)
				}(peer)
			}

			return nil

		default:
			_ = s.Logger.Log("msg", "failed to add block to chain", "hash", hash.ShortString(8), "err", err)
			return err
		}
	}

	if s.isOnline() {
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
	peer, ok := s.peerMap.Get(from.String())

	if !ok {
		return fmt.Errorf("peer (%s) not found", from.String())
	}

	_ = s.Logger.Log("msg", "received status request", "peer-addr", from.String())

	return s.responseStatus(peer)
}

func (s *Server) processResponseStatus(from net.Addr, res *ResponseStatus) error {
	_ = s.Logger.Log("msg", "received status response", "peer-id", res.ID, "peer-addr", from.String(), "peer-height", res.Height, "peer-status", res.Status.String())

	if s.isSyncing() || s.isForked() {
		stat := &ServerStatus{
			ID:               res.ID,
			ListenAddr:       from.String(),
			Status:           res.Status,
			Height:           res.Height,
			GenesisBlockHash: res.GenesisBlockHash,
			CurrentBlockHash: res.CurrentBlockHash,
		}

		go func(stat *ServerStatus) {
			select {
			case s.syncStatCh <- stat:
			case <-time.After(5 * time.Second):
				_ = s.Logger.Log("msg", "deliver to syncStatCh timeout, dropping status message", "peer-id", res.ID, "peer-addr", from.String())
			}
		}(stat)

		return nil
	}

	if s.chain.Height() < res.Height {
		_ = s.Logger.Log("msg", "higher chain height detected, triggering sync", "peer-id", res.ID, "peer-addr", from.String(), "network-height", res.Height)
		go s.syncChainLoop(false)
	}

	return nil
}

func (s *Server) processRequestHeaders(from net.Addr, req *RequestHeaders) error {
	peer, ok := s.peerMap.Get(from.String())

	if !ok {
		return fmt.Errorf("peer (%s) not found", from.String())
	}

	_ = s.Logger.Log("msg", "received headers request", "peer-addr", from.String(), "req-header-from", req.From, "req-header-count", req.Count)

	ourHeight := s.chain.Height()
	count := req.Count

	if count > maxHeadersPerRequest {
		count = maxHeadersPerRequest
	}

	endHeight := req.From + count - 1

	if endHeight > ourHeight {
		endHeight = ourHeight
	}

	headersToSend := make([]*core.Header, 0)

	if req.From <= endHeight {
		for i := req.From; i <= endHeight; i++ {
			block, err := s.chain.GetHeader(i)

			if err != nil {
				_ = s.Logger.Log("error", "failed to get block for sync response", "height", i, "err", err)
				return err
			}

			headersToSend = append(headersToSend, block)
		}
	}

	//_ = s.Logger.Log(
	//	"msg", "sending headers to peer",
	//	"to", from,
	//	"count", len(headersToSend),
	//	"from_height", req.From,
	//	"to_height", endHeight,
	//)

	buf := new(bytes.Buffer)
	res := &ResponseHeaders{
		Headers: headersToSend,
	}

	if err := gob.NewEncoder(buf).Encode(res); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeResHeaders, buf.Bytes())

	return peer.Send(msg.Bytes())
}

func (s *Server) processResponseHeaders(from net.Addr, res *ResponseHeaders) error {
	if len(res.Headers) == 0 {
		return nil
	}

	_ = s.Logger.Log(
		"msg", "validating received header chain",
		"peer-addr", from.String(),
		"count", len(res.Headers),
	)

	var forkPoint uint64
	var forkPointFound bool

	for _, header := range res.Headers {
		if !s.chain.HasHeader(header.PrevBlockHash) {
			forkPoint = header.Height - 1
			forkPointFound = true
			break
		}
	}

	// Reorg
	if forkPointFound {
		_ = s.Logger.Log("msg", "fork point detected", "height", forkPoint)

		if err := s.chain.Rollback(forkPoint); err != nil {
			return err
		}

		if s.isOnline() {
			go s.syncChainLoop(false)
			return nil
		}

		if s.isSyncing() {
			select {
			case s.syncQuitCh <- struct{}{}:
				_ = s.Logger.Log("msg", "fork detected during sync, cancelling current sync")
			default:
			}

			go func() {
				ticker := time.NewTicker(100 * time.Millisecond)

				for {
					if s.isOnline() {
						break
					}

					<-ticker.C
				}

				ticker.Stop()

				s.syncChainLoop(false)
			}()

		}
	}

	if forkPoint == 0 {
		return fmt.Errorf("could not find fork point with peer")
	}

	return nil
}

func (s *Server) processRequestBlocks(from net.Addr, req *RequestBlocks) error {
	peer, ok := s.peerMap.Get(from.String())

	if !ok {
		return fmt.Errorf("peer (%s) not found", from.String())
	}

	_ = s.Logger.Log("msg", "received block request", "peer-addr", from.String(), "req_block_from", req.From, "req_block_count", req.Count)

	ourHeight := s.chain.Height()
	count := req.Count

	if count > maxBlocksPerRequest {
		count = maxBlocksPerRequest
	}

	endHeight := req.From + count - 1

	if endHeight > ourHeight {
		endHeight = ourHeight
	}

	blocksToSend := make([]*core.Block, 0)

	if req.From <= endHeight {
		for i := req.From; i <= endHeight; i++ {
			block, err := s.chain.GetBlock(i)

			if err != nil {
				_ = s.Logger.Log("error", "failed to get block for sync response", "height", i, "err", err)
				return err
			}

			blocksToSend = append(blocksToSend, block)
		}
	}

	//_ = s.Logger.Log(
	//	"msg", "sending blocks to peer",
	//	"to", from,
	//	"count", len(blocksToSend),
	//	"from_height", req.From,
	//	"to_height", endHeight,
	//)

	buf := new(bytes.Buffer)
	res := &ResponseBlocks{
		Blocks: blocksToSend,
	}

	if err := gob.NewEncoder(buf).Encode(res); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeResBlocks, buf.Bytes())

	return peer.Send(msg.Bytes())
}

func (s *Server) processResponseBlocks(from net.Addr, res *ResponseBlocks) error {
	if len(res.Blocks) == 0 {
		return nil
	}

	_ = s.Logger.Log(
		"msg", "received blocks from peer",
		"peer-addr", from.String(),
		"count", len(res.Blocks),
		"start_height", res.Blocks[0].Height,
	)

	for _, block := range res.Blocks {
		if err := s.processBlock(from, block); err != nil {
			switch {
			case errors.Is(err, core.ErrBlockKnown):
				return nil
			case errors.Is(err, core.ErrFutureBlock):
				return nil
			case errors.Is(err, core.ErrUnknownParent):
				return nil
			default:
				_ = s.Logger.Log("msg", "process-block-error", "recv-height", block.Height, "error", err)
				return err
			}
		}
	}

	return nil
}

func (s *Server) requestStatus(peer Peer) error {
	req := new(RequestStatus)
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeReqStatus, buf.Bytes())

	if err := peer.Send(msg.Bytes()); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
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
		_ = s.Logger.Log("msg", "request status message send error. shutting down peer", "addr", peer.Addr())
		return err
	}

	return nil
}

func (s *Server) requestHeaders(peer Peer, from uint64, count uint64) error {
	reqCount := count

	if reqCount > maxHeadersPerRequest {
		reqCount = maxHeadersPerRequest
	}

	buf := new(bytes.Buffer)
	req := &RequestHeaders{
		From:  from,
		Count: reqCount,
	}

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeReqHeaders, buf.Bytes())

	if err := peer.Send(msg.Bytes()); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	return nil
}

func (s *Server) requestBlocks(peer Peer, from uint64, count uint64) error {
	reqCount := count

	if reqCount > maxBlocksPerRequest {
		reqCount = maxBlocksPerRequest
	}

	buf := new(bytes.Buffer)
	req := &RequestBlocks{
		From:  from,
		Count: reqCount,
	}

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeReqBlocks, buf.Bytes())

	if err := peer.Send(msg.Bytes()); err != nil {
		_ = s.Logger.Log("msg", "message encode error", "error", err)
		return err
	}

	return nil
}
