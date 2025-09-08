package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
)

type PeerStatus struct {
	ID             string `json:"id"`
	Addr           string `json:"addr"`
	Connections    uint8  `json:"connections"`
	MaxConnections uint8  `json:"max_connections"`
	Height         uint64 `json:"height"`
	Validator      bool   `json:"validator"`
	Proposer       bool   `json:"proposer"`
}

type PeerDNS interface {
	Register(*PeerStatus, *sync.WaitGroup) error
	DiscoverPeers(*PeerStatus, int) ([]PeerStatus, error)
	Heartbeat(*PeerStatus) error
	Deregister(*PeerStatus) error
}

type PeerDiscoveryResponse struct {
	Peers []PeerStatus `json:"peers"`
}

type DefaultPeerDNS struct {
	dnsServerAddr string
}

func NewDefaultPeerDNS(dnsServerAddr string) *DefaultPeerDNS {
	return &DefaultPeerDNS{
		dnsServerAddr: dnsServerAddr,
	}
}

func (dns *DefaultPeerDNS) Register(stat *PeerStatus, wg *sync.WaitGroup) error {
	defer wg.Done()

	url := fmt.Sprintf("http://%s/register", dns.dnsServerAddr)
	body, err := json.Marshal(stat)
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

func (dns *DefaultPeerDNS) DiscoverPeers(stat *PeerStatus, count int) ([]PeerStatus, error) {
	url := fmt.Sprintf("http://%s/peers", dns.dnsServerAddr)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("count", strconv.Itoa(count))
	q.Add("id", stat.ID)
	q.Add("addr", stat.Addr)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover failed with status: %s", resp.Status)
	}

	var discoveryResp PeerDiscoveryResponse

	if err = json.NewDecoder(resp.Body).Decode(&discoveryResp); err != nil {
		return nil, err
	}

	return discoveryResp.Peers, nil
}

func (dns *DefaultPeerDNS) Heartbeat(stat *PeerStatus) error {
	url := fmt.Sprintf("http://%s/heartbeat", dns.dnsServerAddr)
	body, err := json.Marshal(stat)
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

func (dns *DefaultPeerDNS) Deregister(stat *PeerStatus) error {
	url := fmt.Sprintf("http://%s/deregister", dns.dnsServerAddr)
	body, err := json.Marshal(stat)
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
