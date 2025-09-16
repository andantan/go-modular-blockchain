package types

import "sync"

type SignedInteger interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

type UnsignedInteger interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

type Integer interface {
	SignedInteger | UnsignedInteger
}

type Float interface {
	~float32 | ~float64
}

type Number interface {
	Integer | Float
}

type AtomicOperation[N Number] interface {
	Set(N)
	Get() N
	Add(N)
	Sub(N)
	Mul(N)
	Div(N)
	Eq(N) bool
	Gt(N) bool
	Gte(N) bool
	Lt(N) bool
	Lte(N) bool
}

type AtomicNumber[N Number] struct {
	lock sync.RWMutex
	n    N
}

func NewAtomicNumber[N Number](initialValue N) *AtomicNumber[N] {
	return &AtomicNumber[N]{
		n: initialValue,
	}
}

func (a *AtomicNumber[N]) Set(newValue N) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.n = newValue
}

func (a *AtomicNumber[N]) Get() N {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.n
}

func (a *AtomicNumber[N]) Add(val N) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.n += val
}

func (a *AtomicNumber[N]) Sub(val N) {
	a.lock.Lock()
	defer a.lock.Unlock()

	currentVal := any(a.n)
	switch currentVal.(type) {
	case uint, uint8, uint16, uint32, uint64, uintptr:
		if a.n < val {
			a.n = 0
		} else {
			a.n -= val
		}
	default:
		a.n -= val
	}
}

func (a *AtomicNumber[N]) Mul(val N) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.n *= val
}

func (a *AtomicNumber[N]) Div(val N) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if val == 0 {
		return
	}
	a.n /= val
}

func (a *AtomicNumber[N]) Eq(val N) bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.n == val
}

func (a *AtomicNumber[N]) Gt(val N) bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.n > val
}

func (a *AtomicNumber[N]) Gte(val N) bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.n >= val
}

func (a *AtomicNumber[N]) Lt(val N) bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.n < val
}

func (a *AtomicNumber[N]) Lte(val N) bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.n <= val
}
