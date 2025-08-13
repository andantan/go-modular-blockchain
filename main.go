package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/network"
	"log"
)

func init() {
	config.InitEnv()
	// config.InitLogger(config.GetEnvVar("SERVER_NAME"))
}

var transports = []network.Transport{
	network.NewLocalTransport("LOCAL"),
	network.NewLocalTransport("REMOTE_A"),
	//network.NewLocalTransport("REMOTE_B"),
	//network.NewLocalTransport("REMOTE_C"),
}

func main() {
	initRemoteServers(transports)

	localTr := transports[0]
	// lateTr := network.NewLocalTransport("LATE_NODE")
	// remoteNodeA := transports[1]
	// remoteNodeC := transports[3]

	//go func() {
	//	for {
	//		if err := sendTransaction(remoteNodeA, localNode.Addr()); err != nil {
	//			config.GetDefaultLogger().Error(err)
	//		}
	//
	//		time.Sleep(2 * time.Second)
	//	}
	//}()

	//go func() {
	//	// syncing lazy transport
	//	time.Sleep(7 * time.Second)
	//
	//	//if err := remoteNodeC.Connect(lateTr); err != nil {
	//	//	panic(err)
	//	//}
	//
	//	lateServer := makeServer(string(lateTr.Addr()), lateTr, nil)
	//
	//	go lateServer.Start()
	//}()

	privKey := crypto.GeneratePrivateKey()
	localServer := makeServer("LOCAL", localTr, &privKey)
	localServer.Start()
}

func initRemoteServers(trs []network.Transport) {
	for i := 0; i < len(trs); i++ {
		id := fmt.Sprintf("REMOTE_%d", i)
		s := makeServer(id, trs[i], nil)

		go s.Start()
	}
}

func makeServer(ID string, tr network.Transport, pk *crypto.PrivateKey) *network.Server {
	opts := network.ServerOpts{
		ID:         ID,
		Transport:  tr,
		Transports: transports,
		PrivateKey: pk,
	}

	s, err := network.NewServer(opts)

	if err != nil {
		log.Fatal(err)
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
