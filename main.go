package main

import (
	"bytes"
	"github.com/andantan/go-modular-blockchain/config"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/crypto"
	"github.com/andantan/go-modular-blockchain/network"
	"github.com/sirupsen/logrus"
	"math/rand"
	"strconv"
	"time"
)

func init() {
	config.InitEnv()
	config.InitLogger()
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
			if err := sendTransaction(trRemote, trLocal.Addr()); err != nil {
				logrus.Error(err)
			}

			time.Sleep(1 * time.Second)
		}
	}()

	privKey := crypto.GeneratePrivateKey()

	opts := network.ServerOpts{
		Transports: []network.Transport{trLocal},
		PrivateKey: &privKey,
	}

	s, err := network.NewServer(opts)

	if err != nil {
		config.GetDefaultLogger().Fatal(err)
	}

	s.Start()
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
