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

	log.Info("Starting ", cfg.Server.Type, " server at ", cfg.Server.Host, ":", cfg.Server.Port)
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
