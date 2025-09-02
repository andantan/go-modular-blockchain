package core

import (
	"fmt"
	"github.com/andantan/go-modular-blockchain/types"
)

type State[K comparable, V any] interface {
	Put(K, V) error
	Get(K) (V, bool)
	Has(K) bool
	Delete(K) error
}

type DefaultState[K comparable, V any] struct {
	data *types.SyncMap[K, V]
}

func NewDefaultState[K comparable, V any]() *DefaultState[K, V] {
	return &DefaultState[K, V]{
		data: types.NewSyncMap[K, V](),
	}
}

func (s *DefaultState[K, V]) Put(k K, v V) error {
	s.data.Put(k, v)

	return nil
}

func (s *DefaultState[K, V]) Get(k K) (V, bool) {
	return s.data.Get(k)
}

func (s *DefaultState[K, V]) Has(k K) bool {
	_, ok := s.data.Get(k)

	return ok
}

func (s *DefaultState[K, V]) Delete(k K) error {
	if !s.Has(k) {
		return fmt.Errorf("key not found")
	}

	s.data.Remove(k)

	return nil
}
