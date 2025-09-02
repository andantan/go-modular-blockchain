package types

import (
	"fmt"
	"reflect"
)

type List[T any] struct {
	data []T
}

func NewList[T any]() *List[T] {
	return &List[T]{
		data: []T{},
	}
}

func (l *List[T]) Insert(e T) {
	l.data = append(l.data, e)
}

func (l *List[T]) Get(index int) (T, error) {
	if index > len(l.data)-1 {
		return *new(T), fmt.Errorf("the given index (%d) is higher than the length (%d)", index, len(l.data))
	}

	return l.data[index], nil
}

func (l *List[T]) Pop(index int) {
	l.data = append(l.data[:index], l.data[index+1:]...)
}

func (l *List[T]) Remove(e T) {
	i, err := l.GetIndex(e)

	if err != nil {
		return
	}

	l.Pop(i)
}

func (l *List[T]) Clear() {
	l.data = []T{}
}

func (l *List[T]) GetIndex(e T) (int, error) {
	for i, v := range l.data {
		if reflect.DeepEqual(e, v) {
			return i, nil
		}
	}

	return 0, fmt.Errorf("no value found")
}

func (l *List[T]) Contains(e T) bool {
	_, err := l.GetIndex(e)

	if err != nil {
		return false
	}

	return true
}

func (l *List[T]) First() (T, error) {
	if len(l.data) == 0 {
		return *new(T), fmt.Errorf("no data in the list")
	}

	return l.data[0], nil
}

func (l *List[T]) Last() (T, error) {
	if len(l.data) == 0 {
		return *new(T), fmt.Errorf("no data in the list")
	}

	return l.data[len(l.data)-1], nil
}

func (l *List[T]) Len() int {
	return len(l.data)
}

func (l *List[T]) ToSlice() []T {
	snapshot := make([]T, len(l.data), cap(l.data))

	copy(snapshot, l.data)

	return snapshot
}

func (l *List[T]) Iterator() func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for _, v := range l.data {
			if !yield(v) {
				return
			}
		}
	}
}
