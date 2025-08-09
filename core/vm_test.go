package core

import (
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
		0x03, 0x0a, // PUSH INT(3)
		0x46, 0x0b, // PUSH BYTE('F')
		0x4f, 0x0b, // PUSH BYTE('O')
		0x4f, 0x0b, // PUSH BYTE('O')
		0x0c,       // PACK
		0x03, 0x0a, // PUSH INT(3)
		0x0f, // STORE
	}

	cs := NewState()
	vm := NewVM(data, cs)

	assert.Nil(t, vm.Run())
	v, err := cs.Get([]byte("FOO"))
	assert.Nil(t, err)
	assert.Equal(t, util.DeSerializeInt64(v), int64(3))

	//fmt.Printf("%+v\n", vm.stack.data)
	//fmt.Printf("%+v\n", cs)
}
