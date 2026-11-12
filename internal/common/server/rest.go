package server

import (
	"crypto/tls"
	"net/http"
	"strconv"
	"time"

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

	router, err := s.router()
	if err != nil {
		return err
	}
	// net/http has no timeouts by default, which lets slow or idle clients
	// hold connections open forever
	srv := &http.Server{
		Addr:              s.config.Server.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 16,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}

	// check for tls
	if s.config.Server.TLS.CertPath != "" && s.config.Server.TLS.KeyPath != "" {
		return srv.ListenAndServeTLS(s.config.Server.TLS.CertPath, s.config.Server.TLS.KeyPath)
	}
	return srv.ListenAndServe()
}

// router creates the gin engine and registers the routes
func (s *RESTServer) router() (*gin.Engine, error) {
	router := gin.Default()
	// gin trusts X-Forwarded-For from every peer by default, which lets any
	// client forge the client ip that ends up in the access log
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}

	v1 := router.Group("/v1")
	v1.GET("/uuid", s.getFlakeUUID)
	v1.GET("/decompose/:uuid", s.decompose)
	return router, nil
}

// getFlakeUUID implements the FlakeServiceServer interface
func (s *RESTServer) getFlakeUUID(c *gin.Context) {
	s.log.Debug("Generating new uuid")
	uuid, err := s.flacker.NextUUID()
	if err != nil {
		s.log.Error("Failed to generate uuid: ", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"uuid": uuid,
	})
}

// decompose implements the FlakeServiceServer interface
func (s *RESTServer) decompose(c *gin.Context) {
	param := c.Param("uuid")
	// if uuid is not a valid number, return error
	uuid, err := strconv.ParseUint(param, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid uuid: " + param,
		})
		return
	}
	s.log.Debug("Decomposing: ", uuid)
	decomposed := s.flacker.Decompose(uuid)
	timestamp, ok := s.flacker.Time(decomposed["time"])
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "uuid timestamp is out of range",
		})
		return
	}
	response := gin.H{"timestamp": timestamp.Format(time.RFC3339Nano)}
	for k, v := range decomposed {
		response[k] = v
	}
	c.JSON(http.StatusOK, response)
}
