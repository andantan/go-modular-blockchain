package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/network"
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

	if err := trRemoteA.Connect(trRemoteB); err != nil {
		panic(err)
	}

	if err := trRemoteB.Connect(trRemoteC); err != nil {
		panic(err)
	}

	if err := trRemoteB.Connect(trRemoteA); err != nil {
		panic(err)
	}

	if err := trRemoteA.Connect(trLocal); err != nil {
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

	if err := sendGetStatusMessage(trRemoteA, "REMOTE_B"); err != nil {
		config.GetDefaultLogger().Error(err)
	}

	//go func() {
	//	// syncing lazy transport
	//	time.Sleep(7 * time.Second)
	//
	//	trLate := network.NewLocalTransport("LATE_REMOTE")
	//	if err := trRemoteC.Connect(trLate); err != nil {
	//		panic(err)
	//	}
	//
	//	lateServer := makeServer(string(trLate.Addr()), trLate, nil)
	//
	//	go lateServer.Start()
	//}()

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
		Transport:  tr,
		Transports: []network.Transport{tr},
		PrivateKey: pk,
	}

	s, err := network.NewServer(opts)

	if err != nil {
		config.GetDefaultLogger().Fatal(err)
	}

	return s
}

func sendGetStatusMessage(tr network.Transport, to network.NetAddr) error {
	var (
		getStatusMsg = new(network.GetStatusMessage)
		buf          = new(bytes.Buffer)
	)

	if err := gob.NewEncoder(buf).Encode(getStatusMsg); err != nil {
		return err
	}

	msg := network.NewMessage(network.MessageTypeGetStatus, buf.Bytes())

	return tr.SendMessage(to, msg.Bytes())
}

func sendTransaction(tr network.Transport, to network.NetAddr) error {
	privkey := crypto.GeneratePrivateKey()
	tx := core.NewTransaction(contract())

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

func contract() []byte {
	data := []byte{
		0x02, 0x0a, 0x03, 0x0a,
		0x0d, 0x4f, 0x0b, 0x4f,
		0x0b, 0x46, 0x0b, 0x03,
		0x0a, 0x0c, 0x0f,
	}

	pushFOO := []byte{
		0x4f, 0x0b, 0x4f, 0x0b,
		0x46, 0x0b, 0x03, 0x0a,
		0x0c, 0x10,
	}

	data = append(data, pushFOO...)

	return data
}
