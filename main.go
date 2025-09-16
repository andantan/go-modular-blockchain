package main

import (
	"flag"
	"fmt"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/network"
	"log"
	"strings"
	"time"
)

func main() {
	var port string
	flag.StringVar(&port, "p", "4000", "port for the server")
	flag.StringVar(&port, "port", "4000", "port for the server")

	var domain string
	flag.StringVar(&domain, "d", "", "domain name")
	flag.StringVar(&domain, "domain", "", "domain name")

	var isValidator bool
	flag.BoolVar(&isValidator, "validator", false, "enable validating")

	var isProposer bool
	flag.BoolVar(&isProposer, "proposer", false, "enable proposing")

	var isTester bool
	flag.BoolVar(&isTester, "tester", false, "run the tester")

	flag.Parse()

	if strings.Trim(domain, " ") == "" {
		panic("domain name must not be empty")
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%s", port)
	params := network.NewChainParameter(30*time.Second, 2<<10)

	opts := network.NewServerOpts(listenAddr, domain)
	opts = opts.WithDNS(network.NewDefaultPeerDNS("127.0.0.1:6550"))
	opts = opts.WithFlooding()

	if isTester {
		opts = opts.WithTester()
	}

	privKey, err := crypto.GeneratePrivateKey()

	if err != nil {
		panic(err)
	}

	s, err := network.NewServer(*opts, *params, privKey)

	if err != nil {
		panic(err)
	}

	if isValidator {
		s = s.UpgradeToValidator(isProposer)
	}

	go func() {
		log.Fatal(s.Start())
	}()

	select {}

	//<-time.After(20 * time.Second)
	//
	//wg := &sync.WaitGroup{}
	//wg.Add(1)
	//s.Shutdown(wg)
	//
	//wg.Wait()
	//
	//<-time.After(time.Minute)
}
