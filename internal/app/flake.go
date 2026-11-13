package app

import (
	"errors"
	"math"
	"math/bits"
	"sync"
	"time"

	"github.com/Avielyo10/goflake/config"
)

var (
	// ErrClockBeforeEpoch is returned when the clock is behind the configured epoch
	ErrClockBeforeEpoch = errors.New("clock is before the flake epoch")
	// ErrClockMovedBackwards is returned when the clock moved too far backwards to wait for it to catch up
	ErrClockMovedBackwards = errors.New("clock moved backwards, refusing to generate uuid")
	// ErrTimeOverflow is returned when the time elapsed since the epoch no longer fits in the time bits
	ErrTimeOverflow = errors.New("time since the flake epoch does not fit in flake.bits_len.time bits")
)

// maxClockCatchUp is how long NextUUID is willing to block, on top of one
// tick, waiting for a clock that moved backwards to catch up again
const maxClockCatchUp = time.Second

type Flacker struct {
	datacenterID uint64
	machineID    uint64

	// bit lengths of Flake ID parts
	bitLenDatacenterID uint8
	bitLenMachineID    uint8
	bitLenSequence     uint8

	epoch        uint64 // the unix epoch in milliseconds
	tickMs       uint64 // length of one time unit in milliseconds
	maxTimestamp uint64 // the largest timestamp that fits in the time bits
	maxSequence  uint64 // the largest sequence that fits in the sequence bits

	mu            sync.Mutex
	lastTimestamp uint64 // time units since epoch of the last generated UUID
	sequence      uint64 // sequence number within lastTimestamp
	now           func() time.Time
	sleep         func(time.Duration)
}

// NewFlacker creates a new flacker
func NewFlacker(cfg config.Config) *Flacker {
	tickMs := cfg.Flake.TickMs
	if tickMs == 0 {
		tickMs = 1
	}
	return &Flacker{
		datacenterID:       cfg.DatacenterID,
		machineID:          cfg.MachineID,
		bitLenDatacenterID: uint8(cfg.Flake.BitsLen.DatacenterID),
		bitLenMachineID:    uint8(cfg.Flake.BitsLen.MachineID),
		bitLenSequence:     uint8(cfg.Flake.BitsLen.Sequence),
		epoch:              cfg.Flake.Epoch,
		tickMs:             tickMs,
		maxTimestamp:       uint64(1)<<cfg.Flake.BitsLen.Time - 1,
		maxSequence:        uint64(1)<<cfg.Flake.BitsLen.Sequence - 1,
		now:                time.Now,
		sleep:              time.Sleep,
	}
}

// NextUUID returns the next UUID
func (f *Flacker) NextUUID() (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	timestamp, err := f.timestampAt(f.now())
	if err != nil {
		return 0, err
	}
	if timestamp < f.lastTimestamp {
		// the clock moved backwards, keep issuing from the last timestamp
		timestamp = f.lastTimestamp
	}
	sequence := uint64(0)
	if timestamp == f.lastTimestamp {
		sequence = (f.sequence + 1) & f.maxSequence
		if sequence == 0 {
			// sequence exhausted for this time unit, wait for the next one
			if timestamp, err = f.waitNextTimestamp(); err != nil {
				return 0, err
			}
		}
	}
	if timestamp > f.maxTimestamp {
		// shifting the timestamp would silently drop its high bits and
		// start repeating UUIDs that were already issued
		return 0, ErrTimeOverflow
	}
	// only commit the new state once a UUID is actually issued
	f.lastTimestamp, f.sequence = timestamp, sequence

	return timestamp<<(f.bitLenDatacenterID+f.bitLenMachineID+f.bitLenSequence) |
		f.datacenterID<<(f.bitLenMachineID+f.bitLenSequence) |
		f.machineID<<f.bitLenSequence |
		sequence, nil
}

// waitNextTimestamp blocks until the clock passes lastTimestamp and returns the new timestamp
func (f *Flacker) waitNextTimestamp() (uint64, error) {
	tick := time.Duration(f.tickMs) * time.Millisecond
	for {
		now := f.now()
		timestamp, err := f.timestampAt(now)
		if err != nil {
			return 0, err
		}
		if timestamp > f.lastTimestamp {
			return timestamp, nil
		}
		// lastTimestamp was read from the clock, so the start of the time unit after it fits in a time.Time
		wait := time.UnixMilli(int64(f.epoch + (f.lastTimestamp+1)*f.tickMs)).Sub(now)
		if wait > tick+maxClockCatchUp {
			// don't hold every caller hostage until the clock catches up
			return 0, ErrClockMovedBackwards
		}
		f.sleep(max(wait, 50*time.Microsecond))
	}
}

// timestampAt returns the number of time units elapsed between the epoch and t
func (f *Flacker) timestampAt(t time.Time) (uint64, error) {
	nowMs := t.UnixMilli()
	if nowMs < 0 || uint64(nowMs) < f.epoch {
		return 0, ErrClockBeforeEpoch
	}
	return (uint64(nowMs) - f.epoch) / f.tickMs, nil
}

// Time returns the wall clock time of a timestamp (the "time" part of a
// decomposed UUID), ok is false if it is too far in the future to represent
func (f *Flacker) Time(timestamp uint64) (t time.Time, ok bool) {
	hi, ms := bits.Mul64(timestamp, f.tickMs)
	ms, carry := bits.Add64(ms, f.epoch, 0)
	if hi != 0 || carry != 0 || ms > math.MaxInt64 {
		return time.Time{}, false
	}
	t = time.UnixMilli(int64(ms)).UTC()
	// RFC 3339 only has four digits for the year
	return t, t.Year() <= 9999
}

// Decompose decomposes a UUID into its components
func (f *Flacker) Decompose(uuid uint64) map[string]uint64 {
	var maskSequence = uint64(1)<<f.bitLenSequence - 1
	var maskMachineID = (uint64(1)<<f.bitLenMachineID - 1) << f.bitLenSequence
	var maskDatacenterID = (uint64(1)<<f.bitLenDatacenterID - 1) << (f.bitLenMachineID + f.bitLenSequence)

	msb := uuid >> 63
	// the msb is not part of the time
	time := uuid &^ (1 << 63) >> (f.bitLenDatacenterID + f.bitLenMachineID + f.bitLenSequence)
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
