package network

type Transport interface {
	Consume() <-chan Message
	Connect(Transport) error
	SendMessage(NetAddr, []byte) error
	Addr() NetAddr
}
