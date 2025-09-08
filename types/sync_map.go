package types

import "sync"

type SyncMap[K comparable, V any] struct {
	lock sync.RWMutex
	m    map[K]V
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{
		m: make(map[K]V),
	}
}

func (sm *SyncMap[K, V]) Get(k K) (V, bool) {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	val, ok := sm.m[k]
	return val, ok
}

func (sm *SyncMap[K, V]) Put(k K, v V) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	sm.m[k] = v
}

func (sm *SyncMap[K, V]) Exists(k K) bool {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	_, ok := sm.m[k]
	return ok
}

func (sm *SyncMap[K, V]) PutIfNotExists(k K, v V) bool {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if _, ok := sm.m[k]; ok {
		return false
	}

	sm.m[k] = v

	return true
}

func (sm *SyncMap[K, V]) Len() int {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	return len(sm.m)
}

func (sm *SyncMap[K, V]) Remove(k K) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	delete(sm.m, k)
}

func (sm *SyncMap[K, V]) Keys() []K {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	keys := make([]K, 0, len(sm.m))
	for k := range sm.m {
		keys = append(keys, k)
	}

	return keys
}

func (sm *SyncMap[K, V]) Values() []V {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	values := make([]V, 0, len(sm.m))
	for _, v := range sm.m {
		values = append(values, v)
	}

	return values
}

func (sm *SyncMap[K, V]) Iterator() func(yield func(K, V) bool) {
	return func(yield func(K, V) bool) {
		sm.lock.RLock()
		defer sm.lock.RUnlock()

		for k, v := range sm.m {
			if !yield(k, v) {
				return
			}
		}
	}
}
