package main

import (
	"flag"
	"fmt"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/network"
	"time"
)

func main() {
	port := flag.String("port", "4000", "the port for the server to listen on")
	id := flag.String("id", "DEFAULT", "server identifier to use")
	blockDirName := flag.String("block-dir-name", "", "directory to store blocks (not path) in")
	flood := flag.Bool("flood", false, "enable flooding mode")
	proposer := flag.Bool("proposer", false, "enable validating the blockchain")
	tester := flag.Bool("tester", false, "whether to run the tester")

	flag.Parse()

	listenAddr := fmt.Sprintf("127.0.0.1:%s", *port)
	storeDir := fmt.Sprintf("%s_%s", *blockDirName, *id)
	params := network.NewChainParameter(30*time.Second, 2<<10)

	opts := network.NewServerOpts(*id, listenAddr)
	opts = opts.WithBlockDir(storeDir)
	opts = opts.WithDNS(network.NewDefaultPeerDNS("127.0.0.1:6550"))
	opts = opts.WithFlooding()

	if *tester {
		opts = opts.WithTester()
	}

	if *flood {
		opts = opts.WithFlooding()
	}

	s, err := network.NewServer(*opts, *params)

	if err != nil {
		panic(err)
	}

	if *proposer {
		privKey, err := crypto.GeneratePrivateKey()

		if err != nil {
			panic(err)
		}

		if _, err := s.UpgradeToProposer(privKey); err != nil {
			panic(err)
		}
	}

	go func() {
		_ = s.Start()
	}()

	select {}

	//<-time.After(20 * time.Second)
	//
	//wg := &sync.WaitGroup{}
	//wg.Add(1)
	//s.Shutdown(wg)
	//
	//wg.Wait()
}
