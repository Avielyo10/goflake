package config

import (
	"testing"
	"time"
)

// validConfig returns a configuration that passes validation
func validConfig() Config {
	var c Config
	c.Env = DevelopmentEnvType
	c.Server.Type = GRPCServerType
	c.Server.Port = 8080
	c.Flake.Epoch = 1659034655453
	c.Flake.TickMs = 1
	c.Flake.BitsLen.DatacenterID = 5
	c.Flake.BitsLen.MachineID = 5
	c.Flake.BitsLen.Time = 41
	c.Flake.BitsLen.Sequence = 12
	return c
}

func TestIsValidNodeIDs(t *testing.T) {
	tests := []struct {
		name         string
		datacenterID uint64
		machineID    uint64
		want         bool
	}{
		{"zero", 0, 0, true},
		{"max", 31, 31, true},
		{"datacenter too large", 32, 0, false},
		{"machine too large", 0, 32, false},
		// used to be silently truncated to 44 and 1 by the uint8 fields
		{"machine above 255", 0, 300, false},
		{"negative datacenter", uint64(1<<64 - 255), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Config
			c.DatacenterID = tt.datacenterID
			c.MachineID = tt.machineID
			c.Flake.BitsLen.DatacenterID = 5
			c.Flake.BitsLen.MachineID = 5
			if got := IsValidNodeIDs(c); got != tt.want {
				t.Errorf("IsValidNodeIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsFlakeConfiguredProperly(t *testing.T) {
	tests := []struct {
		name                                  string
		datacenterID, machineID, tm, sequence uint64
		want                                  bool
	}{
		{"default", 5, 5, 41, 12, true},
		{"no node ids", 0, 0, 51, 12, true},
		{"sum too small", 5, 5, 40, 12, false},
		{"sum too large", 5, 5, 42, 12, false},
		{"no time bits", 5, 5, 0, 53, false},
		// used to pass as the uint8 sum wrapped around to 63
		{"sum wraps uint8", 5, 5, 200, 109, false},
		{"sum wraps uint64", 5, 5, 1<<64 - 1, 54, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Config
			c.Flake.BitsLen.DatacenterID = tt.datacenterID
			c.Flake.BitsLen.MachineID = tt.machineID
			c.Flake.BitsLen.Time = tt.tm
			c.Flake.BitsLen.Sequence = tt.sequence
			if got := IsFlakeConfiguredProperly(c); got != tt.want {
				t.Errorf("IsFlakeConfiguredProperly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidEpoch(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	tests := []struct {
		name     string
		epoch    uint64
		tickMs   uint64
		timeBits uint64
		want     bool
	}{
		{"default", 1659034655453, 1, 41, true},
		{"unix epoch", 0, 1, 41, true},
		{"now", 1_800_000_000_000, 1, 41, true},
		{"future", 1_800_000_000_001, 1, 41, false},
		{"time bits already used up", 0, 1, 40, false},
		{"larger tick fits", 0, 10, 40, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Config
			c.Flake.Epoch = tt.epoch
			c.Flake.TickMs = tt.tickMs
			c.Flake.BitsLen.Time = tt.timeBits
			if got := IsValidEpoch(c, now); got != tt.want {
				t.Errorf("IsValidEpoch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr bool
	}{
		{"valid", func(c *Config) {}, false},
		{"zero tick", func(c *Config) { c.Flake.TickMs = 0 }, true},
		{"tick overflows duration", func(c *Config) { c.Flake.TickMs = MaxTickMs + 1 }, true},
		{"future epoch", func(c *Config) { c.Flake.Epoch = 99999999999999999 }, true},
		{"negative port", func(c *Config) { c.Server.Port = -1 }, true},
		{"port too large", func(c *Config) { c.Server.Port = 65536 }, true},
		{"unknown server type", func(c *Config) { c.Server.Type = "http" }, true},
		{"cert without key", func(c *Config) { c.Server.TLS.CertPath = "config.go" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig()
			tt.mutate(&c)
			if err := c.validate(); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServerConfigAddr(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{"0.0.0.0", 8080, "0.0.0.0:8080"},
		{"", 8080, ":8080"},
		{"localhost", 443, "localhost:443"},
		{"::1", 8080, "[::1]:8080"},
	}
	for _, tt := range tests {
		var s ServerConfig
		s.Host, s.Port = tt.host, tt.port
		if got := s.Addr(); got != tt.want {
			t.Errorf("Addr() = %q, want %q", got, tt.want)
		}
	}
}
