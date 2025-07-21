package network

import (
	"fmt"
	"time"
)

type ServerOpts struct {
	Transports []Transport
}

type Server struct {
	ServerOpts

	messageCh chan Message
	quitCh    chan struct{}
}

func NewServer(opts ServerOpts) *Server {
	return &Server{
		ServerOpts: opts,
		messageCh:  make(chan Message),
		quitCh:     make(chan struct{}, 1),
	}
}

func (s *Server) Start() {
	s.initTransports()

	ticker := time.NewTicker(5 * time.Second)

free:
	for {
		select {
		case msg := <-s.messageCh:
			fmt.Printf("%+v\n", msg)
		case <-s.quitCh:
			break free
		case <-ticker.C:
			fmt.Println("tick")
		}
	}

	fmt.Println("Server shutdown")
}

func (s *Server) initTransports() {
	for _, tr := range s.Transports {
		go func(tr Transport) {
			for rpc := range tr.Consume() {
				s.messageCh <- rpc
			}
		}(tr)
	}
}
