package core

import (
	"encoding/binary"
	"io"

	"github.com/andantan/go-modular-blockchain/types"
)

type Header struct {
	Version   uint32
	PrevBlock types.Hash
	Timestamp uint64
	Height    uint32
	Nonce     uint32
}

func (h *Header) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, &h.Version); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, &h.PrevBlock); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, &h.Timestamp); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, &h.Height); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, &h.Nonce); err != nil {
		return err
	}

	return nil
}

func (h *Header) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &h.Version); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &h.PrevBlock); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &h.Timestamp); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &h.Height); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &h.Nonce); err != nil {
		return err
	}

	return nil
}
