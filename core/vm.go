package core

type VM interface {
	Run() error
	Result() (any, error)
}
