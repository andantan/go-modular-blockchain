package main

import (
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/network"
	"time"
)

func init() {
	config.InitEnv()
}

func main() {
	trLocal := network.NewLocalTransport("LOCAL")
	trRemote := network.NewLocalTransport("REMOTE")

	if err := trLocal.Connect(trRemote); err != nil {
		panic(err)
	}

	if err := trRemote.Connect(trLocal); err != nil {
		panic(err)
	}

	go func() {
		for {
			if err := trRemote.SendMessage(trLocal.Addr(), []byte("Hello World")); err != nil {
				panic(err)
			}

			time.Sleep(1 * time.Second)
		}
	}()

	opts := network.ServerOpts{
		Transports: []network.Transport{trLocal},
	}

	s := network.NewServer(opts)

	s.Start()
}
