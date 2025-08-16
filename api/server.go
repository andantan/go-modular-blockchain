package api

import (
	"github.com/andantan/go-modular-blockchain/core"
	"github.com/go-kit/log"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

type Block struct {
	Version       uint32
	DataHash      string
	BlockHash     string
	PrevBlockHash string
	Height        uint32
	TimeStamp     uint64
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

	return e.Start(s.ListenAddr)
}

func (s *Server) handleGetBlock(c echo.Context) error {
	hashOrId := c.Param("hashorid")

	height, err := strconv.Atoi(hashOrId)

	if err != nil {
		return err
	}

	block, err := s.bc.GetBlock(uint32(height))

	if err != nil {
		return err
	}

	jsonBlock := &Block{
		Version:       block.Version,
		Height:        block.Height,
		DataHash:      block.DataHash.String(),
		BlockHash:     block.BlockHash.String(),
		PrevBlockHash: block.PrevBlockHash.String(),
		TimeStamp:     block.Header.Timestamp,
	}

	return c.JSON(http.StatusOK, jsonBlock)
}
