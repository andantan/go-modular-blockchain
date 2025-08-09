package core

import (
	"github.com/andantan/go-modular-blockchain/util"
)

type Instruction byte

const (
	InstrPushInt  Instruction = 0x0a // 10
	InstrPushByte Instruction = 0x0b // 11
	InstrPack     Instruction = 0x0c // 12
	InstrAdd      Instruction = 0x0d // 13
	InstrSub      Instruction = 0x0e // 14
	InstrStore    Instruction = 0x0f // 15
)

type Stack struct {
	data []any
	sp   int // stack pointer
}

func NewStack(size int) *Stack {
	return &Stack{
		data: make([]any, size),
		sp:   0,
	}
}

func (s *Stack) Push(v any) {
	s.data[s.sp] = v
	s.sp++
}

func (s *Stack) Pop() any {
	v := s.data[0]
	s.data = append(s.data[:0], s.data[1:]...)
	s.sp--

	return v
}

func (s *Stack) PopAsInt() int {
	return s.Pop().(int)
}

func (s *Stack) PopAsByte() byte {
	return s.Pop().(byte)
}

func (s *Stack) PopAsByteSlice() []byte {
	return s.Pop().([]byte)
}

type VM struct {
	data          []byte
	ip            int // instruction pointer
	stack         *Stack
	contractState *State
}

func NewVM(data []byte, contractState *State) *VM {
	return &VM{
		data:          data,
		ip:            0,
		stack:         NewStack(1 << 8),
		contractState: contractState,
	}
}

func (vm *VM) Run() error {
	for {
		instr := Instruction(vm.data[vm.ip])

		if err := vm.Exec(instr); err != nil {
			return err
		}

		vm.ip++

		if vm.ip > len(vm.data)-1 {
			break
		}
	}

	return nil
}

func (vm *VM) Exec(instr Instruction) error {
	switch instr {
	case InstrPushInt:
		vm.stack.Push(int(vm.data[vm.ip-1]))

	case InstrPushByte:
		vm.stack.Push(vm.data[vm.ip-1])

	case InstrPack:
		n := vm.stack.PopAsInt()
		b := make([]byte, n)

		for i := 0; i < n; i++ {
			b[i] = vm.stack.PopAsByte()
		}

		vm.stack.Push(b)

	case InstrAdd:
		a := vm.stack.PopAsInt()
		b := vm.stack.PopAsInt()
		c := a + b

		vm.stack.Push(c)

	case InstrSub:
		a := vm.stack.PopAsInt()
		b := vm.stack.PopAsInt()
		c := a - b

		vm.stack.Push(c)

	case InstrStore:
		var (
			key            = vm.stack.PopAsByteSlice()
			value          = vm.stack.Pop()
			serializeValue []byte
		)

		switch v := value.(type) {
		case int:
			serializeValue = util.SerializeInt64(int64(v))
		default:
			panic("TODO: unknown type")
		}

		_ = vm.contractState.Put(key, serializeValue)
	}

	return nil
}
