package server

import (
	"github.com/sirupsen/logrus"

	"github.com/Avielyo10/goflake/config"
)

type Server interface {
	Serve() error
}

// NewServer creates a new server
func NewServer(cfg *config.Config, log *logrus.Logger) Server {
	switch cfg.Server.Type {
	case config.GRPCServerType:
		return NewGRPCServer(cfg, log)
	case config.RESTServerType:
		return NewRESTServer(cfg, log)
	default:
		return NewGRPCServer(cfg, log)
	}
}
