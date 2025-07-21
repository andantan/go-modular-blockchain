package network

type Message struct {
	From    NetAddr
	Payload []byte
}
