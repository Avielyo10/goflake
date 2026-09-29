package config

import "testing"

func TestIsValidNodeIDs(t *testing.T) {
	tests := []struct {
		name         string
		datacenterID uint8
		machineID    uint8
		want         bool
	}{
		{"zero", 0, 0, true},
		{"max", 31, 31, true},
		{"datacenter too large", 32, 0, false},
		{"machine too large", 0, 32, false},
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
