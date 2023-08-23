package main

import (
	"github.com/Avielyo10/goflake/config"
	"github.com/Avielyo10/goflake/internal/common/logger"
	"github.com/Avielyo10/goflake/internal/common/server"
)

func main() {
	cfg := config.MustConfig()
	log := logger.NewLogger(cfg.LogLevel)
	server := server.NewServer(cfg, log)

	tlsEnabled := cfg.Server.TLS.CertPath != "" && cfg.Server.TLS.KeyPath != ""
	if tlsEnabled {
		log.Info("Starting ", cfg.Server.Type, " TLS server at ", cfg.Server.Host, ":", cfg.Server.Port)
	} else {
		log.Info("Starting ", cfg.Server.Type, " server at ", cfg.Server.Host, ":", cfg.Server.Port)
	}
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
