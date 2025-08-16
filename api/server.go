package api

import (
	"encoding/hex"
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/andantan/go-modular-blockchain/types"
	"github.com/go-kit/log"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

type TxResponse struct {
	TxCount uint32
	Hashes  []string
}

type APIError struct {
	Error string
}

type Block struct {
	Version       uint32
	DataHash      string
	BlockHash     string
	PrevBlockHash string
	Height        uint32
	TimeStamp     uint64
	Validator     string
	Signature     string

	TxResponse TxResponse
}

type ServerConfig struct {
	Logger     log.Logger
	ListenAddr string
}

type Server struct {
	ServerConfig
	bc *core.Blockchain
}

func NewServer(cfg ServerConfig, bc *core.Blockchain) *Server {
	return &Server{
		ServerConfig: cfg,
		bc:           bc,
	}
}

func (s *Server) Start() error {
	e := echo.New()

	e.GET("/block/:hashorid", s.handleGetBlock)
	e.GET("/tx/:hash", s.handleGetTx)

	return e.Start(s.ListenAddr)
}

func (s *Server) handleGetTx(c echo.Context) error {
	hash := c.Param("hash")

	h, err := hex.DecodeString(hash)

	if err != nil {
		return c.JSON(http.StatusBadRequest, APIError{Error: err.Error()})
	}

	tx, err := s.bc.GetTxByHash(types.Hash(h))

	if err != nil {
		return c.JSON(http.StatusBadRequest, APIError{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, tx)
}

func (s *Server) handleGetBlock(c echo.Context) error {
	hashOrId := c.Param("hashorid")

	height, err := strconv.Atoi(hashOrId)

	// find block by height
	if err == nil {
		block, err := s.bc.GetBlock(uint32(height))

		if err != nil {
			return c.JSON(http.StatusBadRequest, APIError{Error: err.Error()})
		}

		jsonBlock := intoJSONBlock(block)

		return c.JSON(http.StatusOK, jsonBlock)
	}

	hash, err := hex.DecodeString(hashOrId)

	if err != nil {
		return c.JSON(http.StatusBadRequest, APIError{Error: err.Error()})
	}

	block, err := s.bc.GetBlockByHash(types.MustHashFromBytes(hash))

	if err != nil {
		return c.JSON(http.StatusBadRequest, APIError{Error: err.Error()})
	}

	jsonBlock := intoJSONBlock(block)

	return c.JSON(http.StatusOK, jsonBlock)
}

func intoJSONBlock(block *core.Block) *Block {
	txCount := len(block.Transactions)
	txResponse := TxResponse{
		TxCount: uint32(txCount),
		Hashes:  make([]string, txCount),
	}

	for i, tx := range block.Transactions {
		txResponse.Hashes[i] = tx.String()
	}

	return &Block{
		Version:       block.Version,
		Height:        block.Height,
		DataHash:      block.DataHash.String(),
		BlockHash:     block.BlockHash.String(),
		PrevBlockHash: block.PrevBlockHash.String(),
		TimeStamp:     block.Header.Timestamp,
		Validator:     block.Validator.Address().String(),
		Signature:     block.Signature.String(),
		TxResponse:    txResponse,
	}
}
