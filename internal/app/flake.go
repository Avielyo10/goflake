package app

import (
	"time"

	"github.com/Avielyo10/goflake/config"
	"go.uber.org/atomic"
)

// These constants are the bit lengths of Flake ID parts.
var (
	BitLenTime         = config.GetConfig().Flake.BitsLen.Time         // bit length of time
	BitLenDatacenterID = config.GetConfig().Flake.BitsLen.DatacenterID // bit length of datacenter id
	BitLenMachineID    = config.GetConfig().Flake.BitsLen.MachineID    // bit length of machine id
	BitLenSequence     = config.GetConfig().Flake.BitsLen.Sequence     // bit length of sequence number
	Epoch              = config.GetConfig().Flake.Epoch                // the unix epoch in milliseconds
)

type Flacker struct {
	datacenterID uint8
	machineID    uint8
	sequence     atomic.Uint32
	ticker       *time.Ticker
}

// NewFlacker creates a new flacker
func NewFlacker(cfg config.Config) *Flacker {
	flaker := &Flacker{
		datacenterID: cfg.DatacenterID,
		machineID:    cfg.MachineID,
		sequence:     *atomic.NewUint32(0),
		ticker:       time.NewTicker(time.Millisecond * time.Duration(cfg.Flake.TickMs)),
	}
	flaker.startTicking()
	return flaker
}

// NextUUID returns the next UUID
func (f *Flacker) NextUUID() uint64 {
	sequence := f.sequence.Inc() - 1 // -1 because the sequence starts at 0
	timestamp := uint64(time.Now().UnixMilli()) - Epoch

	uuid := uint64(timestamp)<<(BitLenDatacenterID+BitLenMachineID+BitLenSequence) |
		uint64(f.datacenterID)<<(BitLenMachineID+BitLenSequence) |
		uint64(f.machineID)<<BitLenSequence |
		uint64(sequence)
	return uuid
}

// StartTicking starts the flacker's ticker
func (f *Flacker) startTicking() {
	go func() {
		// reset the sequence after the ticker has ticked, once per millisecond
		for range f.ticker.C {
			f.sequence.Store(0)
		}
	}()
}

// Decompose decomposes a UUID into its components
func (f *Flacker) Decompose(uuid uint64) map[string]uint64 {
	var maskSequence = uint64(1<<BitLenSequence - 1)
	var maskMachineID = uint64((1<<BitLenMachineID - 1) << BitLenSequence)
	var maskDatacenterID = uint64((1<<BitLenDatacenterID - 1) << (BitLenMachineID + BitLenSequence))

	msb := uuid >> 63
	time := uuid >> (BitLenDatacenterID + BitLenMachineID + BitLenSequence)
	datacenterID := uuid & maskDatacenterID >> (BitLenMachineID + BitLenSequence)
	machineID := uuid & maskMachineID >> BitLenSequence
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
