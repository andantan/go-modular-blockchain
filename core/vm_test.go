package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVM(t *testing.T) {
	// example: 1 + 2 = 3
	// solution
	// 1) 1
	// 2) push
	// 3) 2
	// 4) push
	// 5) add

	data := []byte{0x01, 0x0a, 0x02, 0x0a, 0x0b}

	vm := NewVM(data)

	assert.Nil(t, vm.Run())
	assert.Equal(t, byte(3), vm.stack[vm.sp])
	assert.NotEqual(t, byte(4), vm.stack[vm.sp])
}
