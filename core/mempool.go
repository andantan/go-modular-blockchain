package core

import (
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/types"
)

type Mempool struct {
	all     *types.SyncCache[types.Hash, *Transaction]
	pending *types.SyncCache[types.Hash, *Transaction]

	capacity int
}

func NewMemPool(cap int) *Mempool {
	if cap <= 0 {
		panic("mempool capacity must be greater than zero")
	}

	return &Mempool{
		all:      types.NewSyncCache[types.Hash, *Transaction](),
		pending:  types.NewSyncCache[types.Hash, *Transaction](),
		capacity: cap,
	}
}

func (mp *Mempool) Debug() {
	logger := config.DefaultDebugLogger("Mempool")

	_ = logger.Log(
		"cap", mp.capacity,
		"allLength", mp.all.Count(),
		"pendingLength", mp.pending.Count(),
	)
}

func (mp *Mempool) IsNilPending() bool {
	return mp.PendingCount() == 0
}

func (mp *Mempool) Add(tx *Transaction) {
	// pruning the oldest transaction in the all pool
	if mp.all.Count() == mp.capacity {
		oldest, _ := mp.all.First()
		mp.all.Remove(oldest)
	}

	hash := tx.Hash(TxHasher{})

	if !mp.all.Contains(hash) {
		mp.all.Add(hash, tx)
		mp.pending.Add(hash, tx)
	}
}

func (mp *Mempool) Contains(tx *Transaction) bool {
	return mp.all.Contains(tx.Hash(TxHasher{}))
}

func (mp *Mempool) Pending() []*Transaction {
	return mp.pending.Values()
}

func (mp *Mempool) PendingCount() int {
	return mp.pending.Count()
}

func (mp *Mempool) ClearPending() {
	mp.pending.Clear()
}

func (mp *Mempool) PrunePending(txx []*Transaction) {
	hashesToRemove := make([]types.Hash, len(txx))

	for i, tx := range txx {
		hashesToRemove[i] = tx.Hash(TxHasher{})
	}

	mp.pending.Prune(hashesToRemove)
}
