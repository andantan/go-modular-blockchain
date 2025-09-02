package types

import "sync"

type SyncList[T any] struct {
	lock sync.RWMutex
	list *List[T]
}

func NewSyncList[T any]() *SyncList[T] {
	return &SyncList[T]{
		list: NewList[T](),
	}
}

func (sl *SyncList[T]) Insert(e T) {
	sl.lock.Lock()
	defer sl.lock.Unlock()

	sl.list.Insert(e)
}

func (sl *SyncList[T]) Get(index int) (T, error) {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	return sl.list.Get(index)
}

func (sl *SyncList[T]) Pop(index int) {
	sl.lock.Lock()
	defer sl.lock.Unlock()

	sl.list.Pop(index)
}

func (sl *SyncList[T]) Remove(e T) {
	sl.lock.Lock()
	defer sl.lock.Unlock()

	sl.list.Remove(e)
}

func (sl *SyncList[T]) Clear() {
	sl.lock.Lock()
	defer sl.lock.Unlock()

	sl.list.Clear()
}

func (sl *SyncList[T]) GetIndex(e T) (int, error) {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	return sl.list.GetIndex(e)
}

func (sl *SyncList[T]) Contains(e T) bool {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	return sl.list.Contains(e)
}

func (sl *SyncList[T]) First() (T, error) {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	return sl.list.First()
}

func (sl *SyncList[T]) Last() (T, error) {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	return sl.list.Last()
}

func (sl *SyncList[T]) Len() int {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	return sl.list.Len()
}

func (sl *SyncList[T]) GetData() []T {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	return sl.list.ToSlice()
}

func (sl *SyncList[T]) Iterator() func(yield func(T) bool) {
	return func(yield func(T) bool) {
		sl.lock.RLock()
		defer sl.lock.RUnlock()

		listIterator := sl.list.Iterator()
		listIterator(yield)
	}
}
