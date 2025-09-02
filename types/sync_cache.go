package types

import "sync"

type SyncCache[K comparable, V any] struct {
	lock    sync.RWMutex
	lookup  map[K]V
	ordered *List[K]
}

func NewSyncCache[K comparable, V any]() *SyncCache[K, V] {
	return &SyncCache[K, V]{
		lookup:  make(map[K]V),
		ordered: NewList[K](),
	}
}

func (sc *SyncCache[K, V]) Add(k K, v V) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	if _, ok := sc.lookup[k]; !ok {
		sc.lookup[k] = v
		sc.ordered.Insert(k)
	}
}

func (sc *SyncCache[K, V]) Get(k K) (V, bool) {
	sc.lock.RLock()
	defer sc.lock.RUnlock()

	val, ok := sc.lookup[k]
	return val, ok
}

func (sc *SyncCache[K, V]) Remove(k K) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	if _, ok := sc.lookup[k]; ok {
		delete(sc.lookup, k)
	}

	if sc.ordered.Contains(k) {
		sc.ordered.Remove(k)
	}
}

func (sc *SyncCache[K, V]) Prune(keysToRemove []K) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	toRemoveSet := make(map[K]struct{}, len(keysToRemove))

	for _, k := range keysToRemove {
		toRemoveSet[k] = struct{}{}
	}

	for k := range toRemoveSet {
		delete(sc.lookup, k)
	}

	newOrderedKeys := make([]K, 0, len(sc.ordered.data))

	for _, k := range sc.ordered.data {
		if _, found := toRemoveSet[k]; !found {
			newOrderedKeys = append(newOrderedKeys, k)
		}
	}

	sc.ordered.data = newOrderedKeys
}

func (sc *SyncCache[K, V]) First() (K, error) {
	sc.lock.RLock()
	defer sc.lock.RUnlock()

	return sc.ordered.First()
}

func (sc *SyncCache[K, V]) Count() int {
	sc.lock.RLock()
	defer sc.lock.RUnlock()

	return len(sc.lookup)
}

func (sc *SyncCache[K, V]) Contains(k K) bool {
	sc.lock.RLock()
	defer sc.lock.RUnlock()

	_, ok := sc.lookup[k]
	return ok
}

func (sc *SyncCache[K, V]) Clear() {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	sc.lookup = make(map[K]V)
	sc.ordered.Clear()
}

func (sc *SyncCache[K, V]) GetLookup() map[K]V {
	return sc.lookup
}

func (sc *SyncCache[K, V]) Values() []V {
	sc.lock.RLock()
	defer sc.lock.RUnlock()

	orderedKeys := sc.ordered.ToSlice()

	values := make([]V, len(orderedKeys))

	for i, k := range orderedKeys {
		values[i] = sc.lookup[k]
	}

	return values
}

func (sc *SyncCache[K, V]) LookupIterator() func(yield func(K, V) bool) {
	return func(yield func(K, V) bool) {
		sc.lock.RLock()
		defer sc.lock.RUnlock()

		for k, v := range sc.lookup {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (sc *SyncCache[K, V]) OrderedIterator() func(yield func(K) bool) {
	return func(yield func(K) bool) {
		sc.lock.RLock()
		defer sc.lock.RUnlock()

		listIterator := sc.ordered.Iterator()
		listIterator(yield)
	}
}
