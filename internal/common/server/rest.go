package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/Avielyo10/goflake/config"
	"github.com/Avielyo10/goflake/internal/app"
)

type RESTServer struct {
	config  *config.Config
	flacker *app.Flacker
	log     *logrus.Logger
}

// NewRESTServer creates a new rest server
func NewRESTServer(cfg *config.Config, log *logrus.Logger) *RESTServer {
	return &RESTServer{config: cfg, flacker: app.NewFlacker(*cfg), log: log}
}

// Serve starts the rest server, implements the Server interface
func (s *RESTServer) Serve() error {
	// Switch to "release" mode in production.
	if config.ProductionEnvType == s.config.Env {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create a gin engine
	router := gin.Default()

	v1 := router.Group("/v1")
	v1.GET("/uuid", s.getFlakeUUID)
	v1.GET("/decompose/:uuid", s.decompose)

	// check for tls
	if s.config.Server.TLS.CertPath != "" && s.config.Server.TLS.KeyPath != "" {
		return router.RunTLS(fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port), s.config.Server.TLS.CertPath, s.config.Server.TLS.KeyPath)
	} else {
		return router.Run(fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port))
	}
}

// getFlakeUUID implements the FlakeServiceServer interface
func (s *RESTServer) getFlakeUUID(c *gin.Context) {
	s.log.Debug("Generating new uuid")
	c.JSON(http.StatusOK, gin.H{
		"uuid": s.flacker.NextUUID(),
	})
}

// decompose implements the FlakeServiceServer interface
func (s *RESTServer) decompose(c *gin.Context) {
	uuid := c.Param("uuid")
	// if uuid is not a valid number, return error
	if uuid, err := strconv.ParseUint(uuid, 10, 64); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprint("invalid uuid: ", uuid),
		})
		return
	} else {
		s.log.Debug("Decomposing: ", uuid)
		c.JSON(http.StatusOK, s.flacker.Decompose(uuid))
	}
}
