package app

import (
	"sync"
	"time"

	"github.com/Avielyo10/goflake/config"
)

type Flacker struct {
	datacenterID uint64
	machineID    uint64

	// bit lengths of Flake ID parts
	bitLenDatacenterID uint8
	bitLenMachineID    uint8
	bitLenSequence     uint8

	epoch  uint64 // the unix epoch in milliseconds
	tickMs uint64 // length of one time unit in milliseconds

	mu            sync.Mutex
	lastTimestamp uint64 // time units since epoch of the last generated UUID
	sequence      uint64 // sequence number within lastTimestamp
	now           func() time.Time
}

// NewFlacker creates a new flacker
func NewFlacker(cfg config.Config) *Flacker {
	tickMs := cfg.Flake.TickMs
	if tickMs == 0 {
		tickMs = 1
	}
	return &Flacker{
		datacenterID:       uint64(cfg.DatacenterID),
		machineID:          uint64(cfg.MachineID),
		bitLenDatacenterID: cfg.Flake.BitsLen.DatacenterID,
		bitLenMachineID:    cfg.Flake.BitsLen.MachineID,
		bitLenSequence:     cfg.Flake.BitsLen.Sequence,
		epoch:              cfg.Flake.Epoch,
		tickMs:             tickMs,
		now:                time.Now,
	}
}

// NextUUID returns the next UUID
func (f *Flacker) NextUUID() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	maxSequence := uint64(1)<<f.bitLenSequence - 1
	timestamp := f.currentTimestamp()
	if timestamp < f.lastTimestamp {
		// the clock moved backwards, keep issuing from the last timestamp
		timestamp = f.lastTimestamp
	}
	if timestamp == f.lastTimestamp {
		f.sequence = (f.sequence + 1) & maxSequence
		if f.sequence == 0 {
			// sequence exhausted for this time unit, wait for the next one
			for timestamp <= f.lastTimestamp {
				time.Sleep(time.Duration(f.tickMs) * time.Millisecond / 10)
				timestamp = f.currentTimestamp()
			}
		}
	} else {
		f.sequence = 0
	}
	f.lastTimestamp = timestamp

	return timestamp<<(f.bitLenDatacenterID+f.bitLenMachineID+f.bitLenSequence) |
		f.datacenterID<<(f.bitLenMachineID+f.bitLenSequence) |
		f.machineID<<f.bitLenSequence |
		f.sequence
}

// currentTimestamp returns the number of time units elapsed since the epoch
func (f *Flacker) currentTimestamp() uint64 {
	nowMs := uint64(f.now().UnixMilli())
	if nowMs < f.epoch {
		return 0
	}
	return (nowMs - f.epoch) / f.tickMs
}

// Decompose decomposes a UUID into its components
func (f *Flacker) Decompose(uuid uint64) map[string]uint64 {
	var maskSequence = uint64(1)<<f.bitLenSequence - 1
	var maskMachineID = (uint64(1)<<f.bitLenMachineID - 1) << f.bitLenSequence
	var maskDatacenterID = (uint64(1)<<f.bitLenDatacenterID - 1) << (f.bitLenMachineID + f.bitLenSequence)

	msb := uuid >> 63
	time := uuid >> (f.bitLenDatacenterID + f.bitLenMachineID + f.bitLenSequence)
	datacenterID := uuid & maskDatacenterID >> (f.bitLenMachineID + f.bitLenSequence)
	machineID := uuid & maskMachineID >> f.bitLenSequence
	sequence := uuid & maskSequence
	return map[string]uint64{
		"uuid":          uuid,
		"msb":           msb,
		"time":          time,
		"datacenter_id": datacenterID,
		"machine_id":    machineID,
		"sequence":      sequence,
	}
}
