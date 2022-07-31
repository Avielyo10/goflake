package server

import (
	"github.com/sirupsen/logrus"

	"github.com/Avielyo10/goflake/config"
	"github.com/Avielyo10/goflake/internal/app"
)

type Server interface {
	Serve() error
}

// NewServer creates a new server
func NewServer(cfg *config.Config, log *logrus.Logger) Server {
	switch cfg.Server.Type {
	case config.GRPCServerType:
		return &GRPCServer{Config: cfg, flacker: app.NewFlacker(*cfg), log: log}
	case config.RESTServerType:
		return &RESTServer{Config: cfg, flacker: app.NewFlacker(*cfg), log: log}
	default:
		return &GRPCServer{Config: cfg, flacker: app.NewFlacker(*cfg), log: log}
	}
}
