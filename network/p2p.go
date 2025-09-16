package network

type Broadcaster interface {
	Broadcast(any) error
}
