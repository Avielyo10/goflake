package config

import (
	"encoding/json"
	"math"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

// Config is the configuration for the application
type Config struct {
	Env          EnvType      `json:"env"`
	LogLevel     string       `mapstructure:"log_level" json:"log_level"`
	DatacenterID uint64       `mapstructure:"datacenter_id" json:"datacenter_id"`
	MachineID    uint64       `mapstructure:"machine_id" json:"machine_id"`
	Server       ServerConfig `mapstructure:"server" json:"server"`
	Flake        struct {
		Epoch   uint64 `mapstructure:"epoch" json:"epoch"`
		TickMs  uint64 `mapstructure:"tick_ms" json:"tick_ms"`
		BitsLen struct {
			DatacenterID uint64 `mapstructure:"datacenter_id" json:"datacenter_id"`
			MachineID    uint64 `mapstructure:"machine_id" json:"machine_id"`
			Time         uint64 `mapstructure:"time" json:"time"`
			Sequence     uint64 `mapstructure:"sequence" json:"sequence"`
		} `mapstructure:"bits_len" json:"bits_len"`
	} `mapstructure:"flake" json:"flake"`
}

// ServerConfig is the configuration of the server
type ServerConfig struct {
	Type ServerType `mapstructure:"type" json:"type"`
	Host string     `mapstructure:"host" json:"host"`
	Port int        `mapstructure:"port" json:"port"`
	TLS  struct {
		CertPath string `mapstructure:"cert_path" json:"cert_path"`
		KeyPath  string `mapstructure:"key_path" json:"key_path"`
	} `mapstructure:"tls" json:"tls"`
}

// MaxTickMs is the largest flake.tick_ms that still fits in a time.Duration
const MaxTickMs = uint64(math.MaxInt64 / int64(time.Millisecond))

var config Config

// MustConfig configures the application and panics on error
func MustConfig() *Config {
	viper.SetConfigName("config.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/goflake")
	viper.AddConfigPath(".")

	// set defaults
	viper.SetDefault("env", DevelopmentEnvType)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("datacenter_id", 0)
	viper.SetDefault("machine_id", 0)
	viper.SetDefault("server.type", "grpc")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.tls.cert_path", "")
	viper.SetDefault("server.tls.key_path", "")
	viper.SetDefault("flake.epoch", 1659034655453) // Thu Jul 28 2022 18:57:35 UTC
	viper.SetDefault("flake.tick_ms", 1)           // 1 millisecond
	viper.SetDefault("flake.bits_len.datacenter_id", 5)
	viper.SetDefault("flake.bits_len.machine_id", 5)
	viper.SetDefault("flake.bits_len.time", 41)
	viper.SetDefault("flake.bits_len.sequence", 12)

	// catch env variables
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		// if we can't find the config file and the error is not a missing file error
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}
	if err := viper.Unmarshal(&config); err != nil {
		panic(err)
	}
	if err := config.validate(); err != nil {
		panic(err)
	}
	return &config
}

// GetConfig returns the configuration
func GetConfig() Config {
	return config
}

// validate the configuration
func (c Config) validate() error {
	if !IsFlakeConfiguredProperly(c) {
		return ErrInvalidConfig{"flake configuration is not properly configured, flake.bits_len.time must be at least 1 and the sum of bits length must be 63"}
	}
	if !IsValidNodeIDs(c) {
		return ErrInvalidConfig{"datacenter_id and machine_id must fit in flake.bits_len.datacenter_id and flake.bits_len.machine_id bits"}
	}
	if c.Flake.TickMs == 0 || c.Flake.TickMs > MaxTickMs {
		return ErrInvalidConfig{"flake.tick_ms must be greater than 0 and fit in a duration"}
	}
	if !IsValidEpoch(c, time.Now()) {
		return ErrInvalidConfig{"flake.epoch must be in the past and the time elapsed since must fit in flake.bits_len.time bits"}
	}
	if c.Server.Port < 0 || c.Server.Port > 65535 {
		return ErrInvalidConfig{"server.port must be between 0 and 65535"}
	}
	if !IsValidServerType(c.Server.Type) {
		return ErrInvalidConfig{"server.type must be grpc or rest"}
	}
	if !IsValidEnvType(c.Env) {
		return ErrInvalidConfig{"env must be development or production"}
	}
	if !IsValidTLSConfig(c) {
		return ErrInvalidConfig{"tls configuration is invalid"}
	}
	return nil
}

// Addr returns the address the server listens on, brackets an IPv6 host
func (s ServerConfig) Addr() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

// ErrInvalidConfig is returned when the configuration is invalid
type ErrInvalidConfig struct {
	Err string
}

// Error returns the error message
func (e ErrInvalidConfig) Error() string {
	return e.Err
}

// ToString returns the configuration as a string
func (c Config) ToString() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// ServerType is the type of server (grpc, rest, etc)
type ServerType string

var (
	GRPCServerType = ServerType("grpc")
	RESTServerType = ServerType("rest")
)

// ServerTypes is a list of server types
var ServerTypes = []ServerType{
	GRPCServerType,
	RESTServerType,
}

// IsValidServerType returns true if the server type is valid
func IsValidServerType(serverType ServerType) bool {
	for _, t := range ServerTypes {
		if t == serverType {
			return true
		}
	}
	return false
}

// EnvType is the type of environment (development, production, etc)
type EnvType string

var (
	DevelopmentEnvType = EnvType("development")
	ProductionEnvType  = EnvType("production")
)

var EnvTypes = []EnvType{
	DevelopmentEnvType,
	ProductionEnvType,
}

// IsValidEnvType returns true if the environment type is valid
func IsValidEnvType(envType EnvType) bool {
	for _, t := range EnvTypes {
		if t == envType {
			return true
		}
	}
	return false
}

// IsFlakeConfiguredProperly returns true if the flake configuration is properly configured,
// the time needs at least 1 bit and the sum of bits length must be 63
func IsFlakeConfiguredProperly(c Config) bool {
	b := c.Flake.BitsLen
	// bound every length first so the sum can't wrap around to 63
	if b.DatacenterID > 63 || b.MachineID > 63 || b.Time > 63 || b.Sequence > 63 || b.Time == 0 {
		return false
	}
	return b.DatacenterID+b.MachineID+b.Time+b.Sequence == 63
}

// IsValidNodeIDs returns true if the datacenter id and machine id fit in their configured bits length
func IsValidNodeIDs(c Config) bool {
	b := c.Flake.BitsLen
	return b.DatacenterID < 64 && b.MachineID < 64 &&
		c.DatacenterID < uint64(1)<<b.DatacenterID &&
		c.MachineID < uint64(1)<<b.MachineID
}

// IsValidEpoch returns true if the epoch is not after now and the time elapsed since, in ticks, fits in the time bits
func IsValidEpoch(c Config, now time.Time) bool {
	nowMs := now.UnixMilli()
	if nowMs < 0 || c.Flake.Epoch > uint64(nowMs) || c.Flake.TickMs == 0 || c.Flake.BitsLen.Time > 63 {
		return false
	}
	return (uint64(nowMs)-c.Flake.Epoch)/c.Flake.TickMs < uint64(1)<<c.Flake.BitsLen.Time
}

// IsValidTLSConfig returns true if the tls configuration is valid
func IsValidTLSConfig(c Config) bool {
	return (c.Server.TLS.CertPath == "" && c.Server.TLS.KeyPath == "") ||
		(fileExists(c.Server.TLS.CertPath) && fileExists(c.Server.TLS.KeyPath))
}

// fileExists returns true if the file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
