package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type PeerStatus struct {
	Address        string `json:"address"`
	NetAddr        string `json:"net_addr"`
	Domain         string `json:"domain"`
	Connections    uint8  `json:"connections"`
	MaxConnections uint8  `json:"max_connections"`
	Height         uint64 `json:"height"`
	IsValidator    bool   `json:"is_validator"`
}

type PeerDNS interface {
	Register(*PeerStatus) error
	DiscoverPeers() ([]PeerStatus, error)
	ValidatorSet() ([]PeerStatus, error)
	Heartbeat(*PeerStatus) error
	Deregister(*PeerStatus) error
}

type PeerSliceResponse struct {
	Peers []PeerStatus `json:"peers"`
}

type DefaultPeerDNS struct {
	dnsServerAddr string
	dnsUri        string
}

func NewDefaultPeerDNS(dnsServerAddr string) *DefaultPeerDNS {
	return &DefaultPeerDNS{
		dnsServerAddr: dnsServerAddr,
		dnsUri:        fmt.Sprintf("http://%s", dnsServerAddr),
	}
}

func (dns *DefaultPeerDNS) Register(ourStatus *PeerStatus) error {
	body, err := json.Marshal(ourStatus)
	if err != nil {
		return err
	}

	url := dns.dnsUri + "/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register failed with status: %s", resp.Status)
	}

	return nil
}

func (dns *DefaultPeerDNS) DiscoverPeers() ([]PeerStatus, error) {
	url := dns.dnsUri + "/peers"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover failed with status: %s", resp.Status)
	}

	var discoveryResp PeerSliceResponse

	if err = json.NewDecoder(resp.Body).Decode(&discoveryResp); err != nil {
		return nil, err
	}

	return discoveryResp.Peers, nil
}

func (dns *DefaultPeerDNS) ValidatorSet() ([]PeerStatus, error) {
	url := dns.dnsUri + "/validators"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover failed with status: %s", resp.Status)
	}

	var validatorsRes PeerSliceResponse
	if err = json.NewDecoder(resp.Body).Decode(&validatorsRes); err != nil {
		return nil, err
	}

	return validatorsRes.Peers, nil
}

func (dns *DefaultPeerDNS) Heartbeat(ourStatus *PeerStatus) error {
	url := dns.dnsUri + "/heartbeat"
	body, err := json.Marshal(ourStatus)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register failed with status: %s", resp.Status)
	}

	return nil
}

func (dns *DefaultPeerDNS) Deregister(ourStatus *PeerStatus) error {
	url := dns.dnsUri + "/deregister"
	body, err := json.Marshal(ourStatus)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register failed with status: %s", resp.Status)
	}

	return nil
}
