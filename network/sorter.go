package network

import (
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/types"
	"sort"
)

type TxMapSorter struct {
	transactions []*core.Transaction
}

func (s *TxMapSorter) Len() int {
	return len(s.transactions)
}

func (s *TxMapSorter) Less(i, j int) bool {
	return s.transactions[i].FirstSeen() < s.transactions[j].FirstSeen()
}

func (s *TxMapSorter) Swap(i, j int) {
	s.transactions[i], s.transactions[j] = s.transactions[j], s.transactions[i]
}

func NewTxMapSorter(txMap map[types.Hash]*core.Transaction) *TxMapSorter {
	txx := make([]*core.Transaction, 0, len(txMap))

	for _, tx := range txMap {
		txx = append(txx, tx)
	}

	s := &TxMapSorter{
		transactions: txx,
	}

	sort.Sort(s)

	return s
}
