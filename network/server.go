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

type ServerStatusType byte

const (
	ServerStatusTypeBootstrap ServerStatusType = iota
	ServerStatusTypeSyncing
	ServerStatusTypeOnline
	ServerStatusTypeForked
	ServerStatusTypeInConsensus
	ServerStatusTypeOffline
)

func (s ServerStatusType) String() string {
	switch s {
	case ServerStatusTypeBootstrap:
		return "BOOTSTRAP"
	case ServerStatusTypeSyncing:
		return "SYNCING"
	case ServerStatusTypeOnline:
		return "ONLINE"
	case ServerStatusTypeForked:
		return "FORKED"
	case ServerStatusTypeInConsensus:
		return "IN_CONSENSUS"
	case ServerStatusTypeOffline:
		return "OFFLINE"
	default:
		return "UNKNOWN"
	}
}

type ServerStatus struct {
	Address          types.Address
	NetAddr          string
	Status           ServerStatusType
	Height           uint64
	GenesisBlockHash types.Hash
	CurrentBlockHash types.Hash
}

type ServerOpts struct {
	logger log.Logger

	listenAddr string
	domain     string

	dns          PeerDNS
	maxPeers     int
	flooding     bool
	gossipFactor int

	tester   bool
	blockDir string

	rawMessageDecodeFunc RawMessageDecodeFunc
	messageProcessFunc   MessageProcessFunc
}

func NewServerOpts(ListenAddr string, domain string) *ServerOpts {
	return &ServerOpts{
		listenAddr: ListenAddr,
		domain:     domain,
	}
}

func (opts *ServerOpts) WithLogger(logger log.Logger) *ServerOpts {
	opts.logger = logger
	return opts
}

func (opts *ServerOpts) WithDNS(dns PeerDNS) *ServerOpts {
	opts.dns = dns
	return opts
}

func (opts *ServerOpts) WithMaxPeers(maxPeers int) *ServerOpts {
	if maxPeers <= 0 {
		panic("MaxPeers must be greater than 0")
	}

	opts.maxPeers = maxPeers
	return opts
}

func (opts *ServerOpts) WithFlooding() *ServerOpts {
	opts.flooding = true
	return opts
}

func (opts *ServerOpts) WithGossipFactor(factor int) *ServerOpts {
	if factor <= 0 {
		panic("factor must be greater than 0")
	}

	opts.gossipFactor = factor
	return opts
}

func (opts *ServerOpts) WithTester() *ServerOpts {
	opts.tester = true
	return opts
}

func (opts *ServerOpts) WithBlockDir(blockDir string) *ServerOpts {
	if blockDir == "" {
		panic("BlockDir must not be equal \"\"")
	}

	opts.blockDir = blockDir
	return opts
}

func (opts *ServerOpts) WithRawMessageDecodeFunc(f RawMessageDecodeFunc) *ServerOpts {
	if opts.rawMessageDecodeFunc == nil {
		panic("nil RawMessageDecodeFunc")
	}

	opts.rawMessageDecodeFunc = f

	return opts
}

func (opts *ServerOpts) WithDecodedMessageProcessFunc(p MessageProcessFunc) *ServerOpts {
	if p == nil {
		panic("nil DecodedMessageProcessor")
	}

	opts.messageProcessFunc = p

	return opts
}

type Server struct {
	ServerOpts
	ChainParameter

	privKey crypto.PrivateKey
	pubKey  crypto.PublicKey
	address types.Address

	node    Node
	peerMap *types.SyncMap[types.Address, Peer]
	status  *types.AtomicNumber[ServerStatusType]

	validatorSet     *types.SyncSet[types.Address]
	consensusEngines *types.SyncMap[uint64, PBFTConsensusEngine]

	chain   *core.Blockchain
	memPool *core.Mempool

	broadcaster Broadcaster
	processor   core.Processor
	validator   core.Validator
	proposer    core.Proposer

	rawMessageCh chan RawMessage
	newPeerCh    chan Peer
	delPeerCh    chan Peer
	quitCh       chan struct{}

	syncStatCh chan *ServerStatus
	syncQuitCh chan struct{}
}

func NewServer(opts ServerOpts, chainParams ChainParameter, privKey crypto.PrivateKey) (*Server, error) {
	if chainParams.MaxPoolSize <= 0 {
		panic("MaxPoolSize must be greater than zero")
	}

	if chainParams.BlockTime == time.Duration(0) {
		panic("BlockTime must be greater than zero")
	}

	if opts.dns == nil {
		panic("DNS must not be nil")
	}

	addressStr := privKey.PublicKey().Address().ShortString(8)

	if opts.logger == nil {
		opts.logger = config.LoggerWithPrefixes("server", "address", addressStr, "domain", opts.domain)
	}

	if opts.maxPeers == 0 {
		opts.maxPeers = 4
	}

	if !opts.flooding && opts.gossipFactor == 0 {
		opts.gossipFactor = (opts.maxPeers * 2) / 3

		if opts.gossipFactor == 0 && opts.maxPeers > 1 {
			opts.gossipFactor = 1
		}
	}

	if opts.blockDir == "" {
		opts.blockDir = fmt.Sprintf("blocks_%s", opts.domain)
	}

	chain, err := core.NewBlockchain(opts.blockDir)

	if err != nil {
		var syncErr *core.BlockSyncingError

		if errors.As(err, &syncErr) {
			panic("TODO: syncing blocks")
		}

		return nil, err
	}

	quitCh := make(chan struct{})
	node := NewTCPNode(privKey, opts.listenAddr, opts.domain, quitCh)
	messageCh := node.ConsumeMessage()
	newPeerCh, delPeerCh := node.ConsumePeer()

	s := &Server{
		ServerOpts:       opts,
		ChainParameter:   chainParams,
		privKey:          privKey,
		pubKey:           privKey.PublicKey(),
		address:          privKey.PublicKey().Address(),
		node:             node,
		peerMap:          types.NewSyncMap[types.Address, Peer](),
		validatorSet:     types.NewSyncSet[types.Address](),
		consensusEngines: types.NewSyncMap[uint64, PBFTConsensusEngine](),
		status:           types.NewAtomicNumber[ServerStatusType](ServerStatusTypeBootstrap),
		chain:            chain,
		memPool:          core.NewMemPool(chainParams.MaxPoolSize),
		rawMessageCh:     messageCh,
		newPeerCh:        newPeerCh,
		delPeerCh:        delPeerCh,
		quitCh:           quitCh,
		syncStatCh:       nil,
		syncQuitCh:       nil,
	}

	if s.processor == nil {
		s.processor = s.chain.Processor()
	}

	if s.broadcaster == nil {
		s.broadcaster = s
	}

	if opts.rawMessageDecodeFunc == nil {
		s.rawMessageDecodeFunc = s.DefaultRawMessageDecodeFunc
	}

	if opts.messageProcessFunc == nil {
		s.messageProcessFunc = s.DefaultMessageProcessFunc
	}

	return s, nil
}

func (s *Server) UpgradeToValidator(pro bool) *Server {
	s.validator = core.NewBlockValidator(s.privKey)
	_ = s.logger.Log("event", "upgraded_to_validator")

	if pro {
		_ = s.logger.Log("event", "upgraded_to_proposer")
		s.proposer = core.NewBlockProposer(s.chain, s.memPool, s.privKey)
	}

	return s
}

func (s *Server) Start() error {
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

func (s *Server) DefaultRawMessageDecodeFunc(rm RawMessage) (*DecodedMessage, error) {
	msg := new(Message)

	if err := gob.NewDecoder(rm.Payload).Decode(&msg); err != nil {
		_ = s.logger.Log("event", "top_level_message_decode_error", "from", rm.From, "error", err)

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
	case MessageTypePrePrepare:
		o = new(PrePrepareMessage)
	case MessageTypePrepare:
		o = new(PrepareMessage)
	case MessageTypeCommit:
		o = new(CommitMessage)
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

func (s *Server) DefaultMessageProcessFunc(msg *DecodedMessage) {
	switch t := msg.Data.(type) {
	case *core.Transaction:
		s.processTransaction(msg.From, t)
	case *core.Block:
		s.processBlock(msg.From, t)
	case *RequestStatus:
		s.processRequestStatus(msg.From, t)
	case *ResponseStatus:
		s.processResponseStatus(msg.From, t)
	case *RequestHeaders:
		s.processRequestHeaders(msg.From, t)
	case *ResponseHeaders:
		s.processResponseHeaders(msg.From, t)
	case *RequestBlocks:
		s.processRequestBlocks(msg.From, t)
	case *ResponseBlocks:
		s.processResponseBlocks(msg.From, t)
	case *PrePrepareMessage:
		s.processPrePrepareMessage(msg.From, t)
	case *PrepareMessage:
		s.processPrepareMessage(msg.From, t)
	case *CommitMessage:
		s.processCommitMessage(msg.From, t)
	default:
	}
}

func (s *Server) isOnline() bool {
	return s.status.Eq(ServerStatusTypeOnline)
}

func (s *Server) isSyncing() bool {
	return s.status.Eq(ServerStatusTypeSyncing)
}

func (s *Server) isForked() bool {
	return s.status.Eq(ServerStatusTypeForked)
}

func (s *Server) setStatus(status ServerStatusType) {
	s.status.Set(status)
	_ = s.logger.Log("event", "update_status", "status", status.String())
}

func (s *Server) getDNSPeerStatus() *PeerStatus {
	return &PeerStatus{
		Address:        s.address.String(),
		NetAddr:        s.listenAddr,
		Domain:         s.domain,
		Connections:    uint8(s.peerMap.Len()),
		MaxConnections: uint8(s.maxPeers),
		Height:         s.chain.Height(),
		IsValidator:    s.validator != nil,
	}
}

func (s *Server) mustRegisterDNS() {
	_ = s.logger.Log("msg", "register to DNS")

	stat := s.getDNSPeerStatus()
	err := s.dns.Register(stat)

	if err != nil {
		panic("failed to register DNS: " + err.Error())
	}
}

func (s *Server) discoverPeers() {
	currentPeerCount := s.peerMap.Len()
	needed := s.maxPeers - currentPeerCount

	if needed <= 0 {
		return
	}

	peerCandidates, err := s.dns.DiscoverPeers()

	if err != nil {
		panic(fmt.Sprintf("failed to discover peers: %s", err))
	}

	if len(peerCandidates) == 0 {
		return
	}

	connectedPeers := make(map[types.Address]struct{})

	for _, peer := range s.peerMap.Iterator() {
		connectedPeers[peer.Identity().Address] = struct{}{}
	}

	filteredCandidates := make([]PeerStatus, 0)

	for _, candidate := range peerCandidates {
		candidateAddress, err := types.AddressFromHexString(candidate.Address)

		if err != nil {
			continue
		}

		if _, ok := connectedPeers[candidateAddress]; ok {
			continue
		}

		if s.address == candidateAddress {
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

	bestCandidate := filteredCandidates[0]
	bestAddress, _ := types.AddressFromHexString(bestCandidate.Address)

	_ = s.logger.Log(
		"msg", "found peer candidate, attempting to connect",
		"peer-address", bestAddress.ShortString(8),
		"peer-net-addr", bestCandidate.NetAddr,
		"peer-connections", bestCandidate.Connections,
	)

	go func() {
		if err := s.node.Connect(bestCandidate.NetAddr); err != nil {
			_ = s.logger.Log("error", "failed to connect to best peer", "err", err)
		}
	}()
}

func (s *Server) heartbeat() {
	stat := s.getDNSPeerStatus()
	if err := s.dns.Heartbeat(stat); err != nil {
		_ = s.logger.Log("event", "failed_to_heartbeat", "error", err)
	}
}

func (s *Server) loop() {
	s.setStatus(ServerStatusTypeOnline)

	if s.tester {
		go s.testFunc()
	}

	_ = s.logger.Log("event", "server_started", "height", s.chain.Height())

frontier:
	for {
		select {
		case rawMessage := <-s.rawMessageCh:
			var (
				msg *DecodedMessage
				err error
			)

			if msg, err = s.rawMessageDecodeFunc(rawMessage); err != nil {
				_ = s.logger.Log("event", "message_decode_error", "error", err)
				continue
			}

			s.messageProcessFunc(msg)

		case newPeer := <-s.newPeerCh:
			if err := s.processNewPeer(newPeer); err != nil {
				identity := newPeer.Identity()
				_ = s.logger.Log(
					"msg", "failed to connect peer",
					"peer-address", identity.Address.ShortString(8),
					"peer-addr", identity.NetAddr,
					"error", err,
				)
				continue
			}

			s.heartbeat()

		case delPeer := <-s.delPeerCh:
			identity := delPeer.Identity()

			s.peerMap.Remove(identity.Address)
			s.node.Remove(delPeer)

			_ = s.logger.Log(
				"msg", "peer terminated",
				"peer-address", identity.Address.ShortString(8),
				"peer-net-addr", identity.NetAddr,
			)

			s.heartbeat()

		case <-s.quitCh:
			_ = s.logger.Log("event", "shutdown_signal_received")
			_ = s.node.Stop()
			_ = s.logger.Log("event", "closing_all_peer", "count", s.peerMap.Len())

			stat := s.getDNSPeerStatus()
			go func() {
				_ = s.dns.Deregister(stat)
			}()

			for _, peer := range s.peerMap.Iterator() {
				peer.Close()
			}

			_ = s.logger.Log("event", "close_server_channels")
			close(s.newPeerCh)
			close(s.delPeerCh)
			close(s.rawMessageCh)

			break frontier
		}
	}
}

func (s *Server) manageConnections() {
	// Delay for managing safety
	<-time.After(time.Duration(rand.Intn(60)) * time.Second)
	s.discoverPeers()

	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.quitCh:
			return
		case <-ticker.C:
			if s.peerMap.Len() < s.maxPeers {
				s.discoverPeers()
			} else {
				var oldestPeer Peer
				oldestTime := time.Now().UnixNano()

				for _, peer := range s.peerMap.Values() {
					identify := peer.Identity()
					if identify.ConnectedTime < oldestTime {
						oldestTime = identify.ConnectedTime
						oldestPeer = peer
					}
				}

				if oldestPeer != nil {
					_ = s.logger.Log(
						"msg", "churning oldest peer",
						"peer-address", oldestPeer.Identity().Address.ShortString(8),
					)
					oldestPeer.Close()
				}
			}
		}
	}
}

func (s *Server) statusLogLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quitCh:
			return
		case <-ticker.C:
			peers := s.peerMap.Values()

			sort.Slice(peers, func(i, j int) bool {
				return peers[i].Identity().ConnectedTime < peers[j].Identity().ConnectedTime
			})

			peerIDs := make([]string, len(peers))
			for i, p := range peers {
				peerIDs[i] = p.Identity().Domain
			}
			peerStr := fmt.Sprintf("[%s]", strings.Join(peerIDs, ", "))
			connStr := fmt.Sprintf("%d/%d", len(peers), s.maxPeers)
			_ = s.logger.Log(
				"event", "status_update",
				"connections", connStr,
				"chain-height", s.chain.Height(),
				"status", s.status.Get().String(),
				"peers", peerStr,
			)
		}
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
	s.setStatus(ServerStatusTypeSyncing)

	_ = s.logger.Log("event", "start_sync_chain", "full_sync", fullSync)

	defer func() {
		_ = s.logger.Log("event", "sync_chain_terminated", "msg", "resetting syncing fields")
		s.setStatus(ServerStatusTypeOnline)

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
		_ = s.logger.Log("event", "full_sync_triggered", "msg", "resetting local chain state")

		s.chain.ClearHeader()

		if err := s.chain.ClearStorage(); err != nil {
			_ = s.logger.Log("event", "clearing_block_storage_failed", "error", err)
			return
		}

		_ = s.chain.AddBlock(core.GetGenesisBlock())
		_ = s.logger.Log("event", "local_chain_re_initiated", "msg", "local chain reset and genesis block added")
	}

	ticker := time.NewTicker(2 * time.Second)
	onlinePeerMap := make(map[types.Address]*ServerStatus)

sync:
	for {
		select {
		case stat := <-s.syncStatCh:
			_ = s.logger.Log(
				"msg", "received peer status during syncing chain",
				"peer-address", stat.Address.ShortString(8),
				"peer-height", stat.Height,
				"peer-status", stat.Status.String(),
			)

			genesis, _ := s.chain.GetHeader(0)
			localGenesisBlockHash := core.BlockHasher{}.Hash(genesis)

			if stat.GenesisBlockHash != localGenesisBlockHash {
				_ = s.logger.Log(
					"msg", "peer has different genesis block, disconnecting",
					"peer-address", stat.Address.ShortString(8),
				)

				peer, ok := s.peerMap.Get(stat.Address)

				if ok {
					peer.Close()
				}

				continue
			}

			if stat.Status == ServerStatusTypeOnline {
				if stat.Height > s.chain.Height() {
					onlinePeerMap[stat.Address] = stat
					continue
				}

				localCurrentBlockHeader, _ := s.chain.GetHeader(s.chain.Height())
				localCurrentBlockHash := core.BlockHasher{}.Hash(localCurrentBlockHeader)

				if stat.Height == s.chain.Height() || stat.CurrentBlockHash == localCurrentBlockHash {
					onlinePeerMap[stat.Address] = stat
					continue
				}
			}

		case <-ticker.C:
			if s.peerMap.Len() == 0 {
				_ = s.logger.Log("event", "no_peers_connected", "msg", "stopping sync chain")
				break sync
			}

			if len(onlinePeerMap) == 0 {
				_ = s.logger.Log("event", "no_online_peers", "msg", "requesting status from all")
				if err := s.floodRequestStatus(); err != nil {
					_ = s.logger.Log("event", "flood_request_status_error", "error", err)
				}

				continue
			}

			highestHeight := s.chain.Height()
			var highestPeerListenAddr types.Address

			for address, stat := range onlinePeerMap {
				if highestHeight <= stat.Height {
					highestHeight = stat.Height
					highestPeerListenAddr = address
				}
			}

			if s.chain.Height() >= highestHeight {
				_ = s.logger.Log("event", "sync_finished", "msg", "chain is fully synced", "height", s.chain.Height())
				break sync
			}

			if s.chain.Height() < highestHeight {
				p, _ := s.peerMap.Get(highestPeerListenAddr)

				identify := p.Identity()
				_ = s.logger.Log(
					"msg", "request blocks",
					"from_height", s.chain.Height()+1,
					"peer-address", identify.Address.ShortString(8),
					"peer-net-addr", identify.NetAddr,
				)
				_ = s.requestBlocks(p, s.chain.Height()+1, 16)
			}

			if err := s.floodRequestStatus(); err != nil {
				_ = s.logger.Log("event", "flood_request_status_error", "error", err)
			}

		case <-s.syncQuitCh:
			_ = s.logger.Log("event", "canceled_sync_chain")
			break sync

		case <-s.quitCh: // server cut-down signal
			_ = s.logger.Log("event", "shutdown", "msg", "server shutdown signal received, terminating sync loop")
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

				_ = s.broadcaster.Broadcast(tx)
			}()
		}

		<-testTicker.C
	}
}

func (s *Server) Broadcast(o any) error {
	var msgType MessageType

	buf := new(bytes.Buffer)

	switch t := o.(type) {
	case *core.Transaction:
		_ = t.Marshall(buf)
		msgType = MessageTypeNewTx
	case *core.Block:
		_ = t.Marshall(buf)
		msgType = MessageTypeNewBlock
	case *PrePrepareMessage:
		_ = t.Marshall(buf)
		msgType = MessageTypePrePrepare
	case *PrepareMessage:
		_ = t.Marshall(buf)
		msgType = MessageTypePrepare
	case *CommitMessage:
		_ = t.Marshall(buf)
		msgType = MessageTypeCommit
	}

	msg := NewMessage(msgType, buf.Bytes())

	if s.flooding {
		return s.flood(msg.Bytes())
	}

	return s.gossip(msg.Bytes())
}

func (s *Server) flood(payload []byte) error {
	for _, peer := range s.peerMap.Iterator() {
		go s.send(peer, payload)
	}

	return nil
}

func (s *Server) floodRequestStatus() error {
	req := new(RequestStatus)
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		_ = s.logger.Log("event", "encode_request_status_error", "err", err)
		return err
	}

	msg := NewMessage(MessageTypeReqStatus, buf.Bytes())

	return s.flood(msg.Bytes())
}

func (s *Server) gossip(payload []byte) error {
	if s.peerMap.Len() <= s.gossipFactor {
		return s.flood(payload)
	}

	peers := s.peerMap.Values()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(peers), func(i, j int) {
		peers[i], peers[j] = peers[j], peers[i]
	})

	for _, peer := range peers {
		go s.send(peer, payload)
	}

	return nil
}

func (s *Server) send(peer Peer, payload []byte) {
	if err := peer.Send(payload); IsUnrecoverableTCPError(err) {
		identify := peer.Identity()
		_ = s.logger.Log(
			"msg", "send message error",
			"peer-address", identify.Address.ShortString(8),
			"peer-net-addr", identify.NetAddr,
			"err", err,
		)

		peer.Close()
	}

	return
}

func (s *Server) proposerLoop() {
	ticker := time.NewTicker(s.BlockTime)

	_ = s.logger.Log("msg", "starting proposer loop")

	for {
		<-ticker.C

		if err := s.createNewBlock(); err != nil {
			_ = s.logger.Log("msg", "error creating new block", "err", err)
		}
	}
}

func (s *Server) createNewBlock() error {
	if s.memPool.IsNilPending() {
		_ = s.logger.Log("msg", "no pending transactions, skipping block creation")
		return nil
	}

	block, err := s.proposer.CreateBlock()

	if err != nil {
		_ = s.logger.Log("msg", "failed to create block", "err", err)
		return err
	}

	_ = s.logger.Log(
		"msg", "new block created successfully",
		"height", block.Height,
		"hash", block.Hash(core.BlockHasher{}).ShortString(10),
		"transactions", len(block.Transactions),
	)

	prePrepareMsg := NewPrePrepareMessage(block)

	if err = prePrepareMsg.Sign(s.validator.PrivateKey()); err != nil {
		return err
	}

	go func() {
		if err = s.broadcaster.Broadcast(prePrepareMsg); err != nil {
			_ = s.logger.Log("msg", "broadcast-pre-prepare-error", "error", err)
		}
	}()

	return nil
}

func (s *Server) processNewPeer(peer Peer) error {
	if s.peerMap.Len() >= s.maxPeers {
		peer.Close()
		return fmt.Errorf("max peers reached")
	}

	identify := peer.Identity()
	s.peerMap.Put(identify.Address, peer)
	_ = s.logger.Log(
		"msg", "new peer registered",
		"peer-address", identify.Address.ShortString(8),
		"peer-net-addr", identify.NetAddr,
		"peer-net-domain", identify.Domain,
	)

	go peer.Read()

	return nil
}

func (s *Server) processTransaction(_ types.Address, tx *core.Transaction) {
	if !s.isOnline() {
		return
	}

	hash := tx.Hash(core.TxHasher{})

	if s.memPool.Contains(tx) {
		return
	}

	// _ = s.Logger.Log("msg", "processing transaction", "tx", tx.Nonce)

	if err := tx.Verify(); err != nil {
		_ = s.logger.Log("msg", "invalid transaction signature", "hash", hash.ShortString(8), "err", err)
		return
	}

	tx.SetFirstSeen()
	s.memPool.Add(tx)

	go func() {
		if err := s.broadcaster.Broadcast(tx); err != nil {
			_ = s.logger.Log("msg", "failed to broadcast transaction", "hash", hash.ShortString(8), "err", err)
		}
	}()

	return
}

func (s *Server) processBlock(from types.Address, block *core.Block) {
	hash := block.Hash(core.BlockHasher{})

	if err := s.chain.AddBlock(block); err != nil {
		switch {
		case errors.Is(err, core.ErrBlockKnown):
			return

		case errors.Is(err, core.ErrFutureBlock):
			if s.isOnline() {
				_ = s.logger.Log(
					"msg", "future block received, triggering sync",
					"our_height", s.chain.Height(),
					"network_height", block.Height,
				)

				go s.syncChainLoop(false)
			}

			return

		case errors.Is(err, core.ErrUnknownParent):
			s.setStatus(ServerStatusTypeForked)

			_ = s.logger.Log(
				"msg", "block with unknown parent, fork detected",
				"local-height", s.chain.Height(),
				"network-height", block.Height,
			)

			if peer, ok := s.peerMap.Get(from); ok {
				go func(peer Peer) {
					req := uint64(1)
					if s.chain.Height() > 7 {
						req = s.chain.Height() - 7
					}
					_ = s.requestHeaders(peer, req, 8)
				}(peer)
			}

			return

		default:
			_ = s.logger.Log("msg", "failed to add block to chain", "hash", hash.ShortString(8), "err", err)
			return
		}
	}

	if s.isOnline() {
		s.memPool.PrunePending(block.Transactions)

		go func() {
			if err := s.broadcaster.Broadcast(block); err != nil {
				_ = s.logger.Log("msg", "broadcast-block-error", "error", err)
			}
		}()
	}

	return
}

func (s *Server) processRequestStatus(from types.Address, _ *RequestStatus) {
	peer, ok := s.peerMap.Get(from)

	if !ok {
		_ = s.logger.Log("msg", "peer not found", "not_found", from.ShortString(8))
		return
	}

	_ = s.logger.Log("msg", "received status request", "peer-addr", from.ShortString(8))

	s.responseStatus(peer)

	return
}

func (s *Server) processResponseStatus(from types.Address, res *ResponseStatus) {
	_ = s.logger.Log(
		"msg", "received status response",
		"peer-address", from.ShortString(8),
		"peer-net-addr", res.NetAddr,
		"peer-height", res.Height,
		"peer-status", res.Status.String(),
	)

	if s.isSyncing() || s.isForked() {
		stat := &ServerStatus{
			Address:          from,
			NetAddr:          res.NetAddr,
			Status:           res.Status,
			Height:           res.Height,
			GenesisBlockHash: res.GenesisBlockHash,
			CurrentBlockHash: res.CurrentBlockHash,
		}

		go func(stat *ServerStatus) {
			select {
			case s.syncStatCh <- stat:
			case <-time.After(5 * time.Second):
				_ = s.logger.Log(
					"msg", "deliver to syncStatCh timeout, dropping status message",
					"peer-address", from.ShortString(8),
					"peer-net-addr", stat.NetAddr,
				)
			}
		}(stat)

		return
	}

	if s.chain.Height() < res.Height {
		_ = s.logger.Log(
			"msg", "higher chain height detected, triggering sync",
			"peer-address", from.ShortString(8),
			"peer-net-addr", res.NetAddr,
			"network-height", res.Height,
		)
		go s.syncChainLoop(false)
	}

	return
}

func (s *Server) processRequestHeaders(from types.Address, req *RequestHeaders) {
	peer, ok := s.peerMap.Get(from)

	if !ok {
		_ = s.logger.Log("msg", "peer not found", "not_found", from.ShortString(8))
		return
	}

	_ = s.logger.Log(
		"msg", "received headers request",
		"peer-address", from.ShortString(8),
		"header_from", req.From,
		"count", req.Count,
	)

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
				_ = s.logger.Log("error", "failed to get block for sync response", "height", i, "err", err)
				return
			}

			headersToSend = append(headersToSend, block)
		}
	}

	buf := new(bytes.Buffer)
	res := &ResponseHeaders{
		Headers: headersToSend,
	}

	if err := gob.NewEncoder(buf).Encode(res); err != nil {
		_ = s.logger.Log("msg", "message encode error", "error", err)
		return
	}

	msg := NewMessage(MessageTypeResHeaders, buf.Bytes())

	go s.send(peer, msg.Bytes())

	return
}

func (s *Server) processResponseHeaders(from types.Address, res *ResponseHeaders) {
	if len(res.Headers) == 0 {
		return
	}

	_ = s.logger.Log(
		"msg", "validating received header chain",
		"peer-address", from.ShortString(8),
		"header-from", res.Headers[0].Height,
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
		_ = s.logger.Log("msg", "fork point detected", "height", forkPoint)

		if err := s.chain.Rollback(forkPoint); err != nil {
			return
		}

		if s.isOnline() {
			go s.syncChainLoop(false)
			return
		}

		if s.isSyncing() {
			select {
			case s.syncQuitCh <- struct{}{}:
				_ = s.logger.Log("msg", "fork detected during sync, cancelling current sync")
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
		_ = s.logger.Log("msg", "could not find fork point with peer")
	}

	return
}

func (s *Server) processRequestBlocks(from types.Address, req *RequestBlocks) {
	peer, ok := s.peerMap.Get(from)

	if !ok {
		_ = s.logger.Log("msg", "peer not found", "not_found", from.String())
		return
	}

	_ = s.logger.Log(
		"msg", "received block request",
		"peer-address", from.ShortString(8),
		"block-from", req.From,
		"count", req.Count,
	)

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
				_ = s.logger.Log("error", "failed to get block for sync response", "height", i, "err", err)
				return
			}

			blocksToSend = append(blocksToSend, block)
		}
	}

	buf := new(bytes.Buffer)
	res := &ResponseBlocks{
		Blocks: blocksToSend,
	}

	if err := gob.NewEncoder(buf).Encode(res); err != nil {
		_ = s.logger.Log("msg", "response blocks message encode error", "error", err)
		return
	}

	msg := NewMessage(MessageTypeResBlocks, buf.Bytes())
	go s.send(peer, msg.Bytes())

	return
}

func (s *Server) processResponseBlocks(from types.Address, res *ResponseBlocks) {
	if len(res.Blocks) == 0 {
		return
	}

	_ = s.logger.Log(
		"msg", "received blocks from peer",
		"peer-address", from.ShortString(8),
		"count", len(res.Blocks),
		"start_height", res.Blocks[0].Height,
	)

	for _, block := range res.Blocks {
		s.processBlock(from, block)
	}

	return
}

func (s *Server) processPrePrepareMessage(from types.Address, msg *PrePrepareMessage) {
	if s.validator != nil && s.isOnline() {
		height := msg.Block.Height
		//_ = s.logger.Log(
		//	"msg", "received pre-prepare message",
		//	"from", from.ShortString(8),
		//	"height", height,
		//)

		if s.consensusEngines.Exists(height) {
			return
		}

		s.validatorSet.Clear()

		newValidators, err := s.dns.ValidatorSet()

		if err != nil {
			_ = s.logger.Log("msg", "failed to get validator set", "err", err)
		}

		for _, v := range newValidators {
			validatorAddress, err := types.AddressFromHexString(v.Address)

			if err != nil {
				continue
			}

			s.validatorSet.Put(validatorAddress)
		}

		_ = s.logger.Log("msg", "validator set updated", "target-height", height, "count", len(newValidators))

		engine := NewDefaultPBFTConsensusEngine(s.broadcaster, s.processor, s.validator, s.validatorSet.Values())
		s.consensusEngines.Put(height, engine)

		if _, ok := s.peerMap.Get(from); ok {
			if err = engine.HandleMessage(from, msg); err != nil {
				if errors.Is(err, core.ErrFutureBlock) || errors.Is(err, core.ErrUnknownParent) {
					go s.syncChainLoop(false)
					return
				} else {
					_ = s.logger.Log("msg", "failed to handle pre-prepare message", "err", err)
				}
			}
		}
	}
}

func (s *Server) processPrepareMessage(from types.Address, msg *PrepareMessage) {
	if s.validator != nil && s.isOnline() {
		height := msg.Height
		// _ = s.logger.Log("event", "received_prepare", "from", from.ShortString(8), "height", height)

		engine, ok := s.consensusEngines.Get(height)
		if !ok {
			_ = s.logger.Log("error", "no consensus engine found for height", "height", height)
			return
		}

		if _, ok = s.peerMap.Get(from); ok {
			if err := engine.HandleMessage(from, msg); err != nil {
				_ = s.logger.Log("error", "prepare_handle_failed", "err", err)
			}
		}
	}
}

func (s *Server) processCommitMessage(from types.Address, msg *CommitMessage) {
	if s.validator != nil && s.isOnline() {
		height := msg.Height
		// _ = s.logger.Log("event", "received_commit", "from", from.ShortString(8), "height", height)

		engine, ok := s.consensusEngines.Get(height)
		if !ok {
			_ = s.logger.Log("error", "no consensus engine found for height", "height", height)
			return
		}

		if _, ok = s.peerMap.Get(from); ok {
			if err := engine.HandleMessage(from, msg); err != nil {
				_ = s.logger.Log("error", "commit_handle_failed", "err", err)
			}
		}
	}
}

func (s *Server) requestStatus(peer Peer) error {
	req := new(RequestStatus)
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(req); err != nil {
		_ = s.logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeReqStatus, buf.Bytes())
	s.send(peer, msg.Bytes())

	return nil
}

func (s *Server) responseStatus(peer Peer) {
	buf := new(bytes.Buffer)
	height := s.chain.Height()
	genesis, _ := s.chain.GetHeader(0)
	current, _ := s.chain.GetHeader(height)
	stat := &ResponseStatus{
		Address:          s.address,
		NetAddr:          s.listenAddr,
		Version:          s.chain.Version(),
		Height:           height,
		Status:           s.status.Get(),
		GenesisBlockHash: core.BlockHasher{}.Hash(genesis),
		CurrentBlockHash: core.BlockHasher{}.Hash(current),
	}

	if err := gob.NewEncoder(buf).Encode(stat); err != nil {
		_ = s.logger.Log("msg", "message encode error", "error", err)
		return
	}

	msg := NewMessage(MessageTypeResStatus, buf.Bytes())

	go s.send(peer, msg.Bytes())

	return
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
		_ = s.logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeReqHeaders, buf.Bytes())

	go s.send(peer, msg.Bytes())

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
		_ = s.logger.Log("msg", "message encode error", "error", err)
		return err
	}

	msg := NewMessage(MessageTypeReqBlocks, buf.Bytes())
	s.send(peer, msg.Bytes())

	return nil
}
