package core

type Contract interface {
	Execute(*Transaction) error
}
