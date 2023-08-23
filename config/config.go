package config

import (
	"encoding/json"
	"os"

	"github.com/spf13/viper"
)

// Config is the configuration for the application
type Config struct {
	Env          EnvType `json:"env"`
	LogLevel     string  `mapstructure:"log_level" json:"log_level"`
	DatacenterID uint8   `mapstructure:"datacenter_id" json:"datacenter_id"`
	MachineID    uint8   `mapstructure:"machine_id" json:"machine_id"`
	Server       struct {
		Type ServerType `mapstructure:"type" json:"type"`
		Host string     `mapstructure:"host" json:"host"`
		Port int        `mapstructure:"port" json:"port"`
		TLS  struct {
			CertPath string `mapstructure:"cert_path" json:"cert_path"`
			KeyPath  string `mapstructure:"key_path" json:"key_path"`
		} `mapstructure:"tls" json:"tls"`
	} `mapstructure:"server" json:"server"`
	Flake struct {
		Epoch   uint64 `mapstructure:"epoch" json:"epoch"`
		TickMs  uint64 `mapstructure:"tick_ms" json:"tick_ms"`
		BitsLen struct {
			DatacenterID uint8 `mapstructure:"datacenter_id" json:"datacenter_id"`
			MachineID    uint8 `mapstructure:"machine_id" json:"machine_id"`
			Time         uint8 `mapstructure:"time" json:"time"`
			Sequence     uint8 `mapstructure:"sequence" json:"sequence"`
		} `mapstructure:"bits_len" json:"bits_len"`
	} `mapstructure:"flake" json:"flake"`
}

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
		return ErrInvalidConfig{"flake configuration is not properly configured, sum of bits length must be 63"}
	}
	if !IsValidServerType(c.Server.Type) {
		return ErrInvalidConfig{"server.type must be grpc or http"}
	}
	if !IsValidEnvType(c.Env) {
		return ErrInvalidConfig{"env must be development or production"}
	}
	if !IsValidTLSConfig(c) {
		return ErrInvalidConfig{"tls configuration is invalid"}
	}
	return nil
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

// IsFlakeConfiguredProperly returns true if the flake configuration is properly configured, sum of bits length must be 63
func IsFlakeConfiguredProperly(c Config) bool {
	sum := c.Flake.BitsLen.DatacenterID + c.Flake.BitsLen.MachineID + c.Flake.BitsLen.Time + c.Flake.BitsLen.Sequence
	return sum == 63
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
