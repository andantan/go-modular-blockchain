package core

import (
	"fmt"
	"github.com/andantan/go-modular-blockchain/util"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStack(t *testing.T) {
	s := NewStack(1 << 4)

	s.Push(1)
	s.Push(2)

	v1 := s.Pop()
	v2 := s.Pop()

	assert.Equal(t, v1, 1)
	assert.Equal(t, v2, 2)

	assert.Nil(t, s.data[0])
	assert.Nil(t, s.data[1])
}

func TestVM(t *testing.T) {
	data := []byte{
		0x02, 0x0a, 0x03, 0x0a, 0x0d, // 2 + 3 = 5
		0x4f, 0x0b, 0x4f, 0x0b, 0x46, 0x0b, 0x03, 0x0a, 0x0c, // FOO
		0x0f, // STORE
	}

	pushFOO := []byte{0x4f, 0x0b, 0x4f, 0x0b, 0x46, 0x0b, 0x03, 0x0a, 0x0c, 0x10}

	data = append(data, pushFOO...)

	cs := NewState()
	vm := NewVM(data, cs)

	assert.Nil(t, vm.Run())

	fmt.Printf("%+v\n", vm.stack.data)
	fmt.Printf("%+v\n", cs.data)

	v, err := cs.Get([]byte("FOO"))
	assert.Nil(t, err)
	assert.Equal(t, util.DeSerializeInt64(v), int64(5))

	value := vm.stack.PopAsByteSlice()
	serialized := util.DeSerializeInt64(value)
	assert.Equal(t, serialized, int64(5))
	assert.Nil(t, vm.stack.Pop())
}

func TestVM_Mul(t *testing.T) {
	data := []byte{
		0x02, 0x0a, 0x03, 0x0a, 0x11,
	}

	cs := NewState()
	vm := NewVM(data, cs)

	assert.Nil(t, vm.Run())

	assert.Equal(t, vm.stack.PopAsInt(), 6)
}

func TestVM_Div(t *testing.T) {
	data := []byte{
		0x04, 0x0a, 0x02, 0x0a, 0x12,
	}

	cs := NewState()
	vm := NewVM(data, cs)

	assert.Nil(t, vm.Run())

	assert.Equal(t, vm.stack.PopAsInt(), 2)
}
