package main

import (
	"bytes"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/network"
	"math/rand"
	"strconv"
	"time"
)

func init() {
	config.InitEnv()
	config.InitLogger(config.GetEnvVar("SERVER_NAME"))
}

func main() {
	trLocal := network.NewLocalTransport("LOCAL")
	trRemoteA := network.NewLocalTransport("REMOTE_A")
	trRemoteB := network.NewLocalTransport("REMOTE_B")
	trRemoteC := network.NewLocalTransport("REMOTE_C")

	// prototype
	if err := trLocal.Connect(trRemoteA); err != nil {
		panic(err)
	}

	if err := trRemoteA.Connect(trLocal); err != nil {
		panic(err)
	}

	if err := trRemoteA.Connect(trRemoteB); err != nil {
		panic(err)
	}

	if err := trRemoteB.Connect(trRemoteA); err != nil {
		panic(err)
	}

	if err := trRemoteB.Connect(trRemoteC); err != nil {
		panic(err)
	}

	if err := trRemoteC.Connect(trRemoteB); err != nil {
		panic(err)
	}

	initRemoteServers([]network.Transport{trRemoteA, trRemoteB, trRemoteC})

	go func() {
		for {
			if err := sendTransaction(trRemoteA, trLocal.Addr()); err != nil {
				config.GetDefaultLogger().Error(err)
			}

			time.Sleep(2 * time.Second)
		}
	}()

	privKey := crypto.GeneratePrivateKey()
	localServer := makeServer("local", trLocal, &privKey)
	localServer.Start()
}

func initRemoteServers(trs []network.Transport) {
	for i, tr := range trs {
		id := fmt.Sprintf("REMOTE_%d", i)
		s := makeServer(id, tr, nil)

		go s.Start()
	}
}

func makeServer(ID string, tr network.Transport, pk *crypto.PrivateKey) *network.Server {
	opts := network.ServerOpts{
		ID:         ID,
		Transports: []network.Transport{tr},
		PrivateKey: pk,
	}

	s, err := network.NewServer(opts)

	if err != nil {
		config.GetDefaultLogger().Fatal(err)
	}

	return s
}

func sendTransaction(tr network.Transport, to network.NetAddr) error {
	privkey := crypto.GeneratePrivateKey()
	data := []byte(strconv.FormatInt(int64(rand.Intn(1000)), 10))
	tx := core.NewTransaction(data)

	if err := tx.Sign(privkey); err != nil {
		return err
	}

	buf := &bytes.Buffer{}

	if err := tx.Encode(core.NewGobTxEncoder(buf)); err != nil {
		return err
	}

	msg := network.NewMessage(network.MessageTypeTx, buf.Bytes())

	return tr.SendMessage(to, msg.Bytes())
}
