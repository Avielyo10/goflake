package app

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Avielyo10/goflake/config"
)

func testConfig() config.Config {
	var cfg config.Config
	cfg.DatacenterID = 3
	cfg.MachineID = 17
	cfg.Flake.Epoch = 1659034655453
	cfg.Flake.TickMs = 1
	cfg.Flake.BitsLen.DatacenterID = 5
	cfg.Flake.BitsLen.MachineID = 5
	cfg.Flake.BitsLen.Sequence = 12
	cfg.Flake.BitsLen.Time = 41
	return cfg
}

// fakeClock returns a clock that advances by step every time it is read
func fakeClock(start time.Time, step time.Duration) func() time.Time {
	var mu sync.Mutex
	now := start
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		t := now
		now = now.Add(step)
		return t
	}
}

// mustNextUUID returns the next UUID and fails the test on error
func mustNextUUID(t testing.TB, f *Flacker) uint64 {
	t.Helper()
	uuid, err := f.NextUUID()
	if err != nil {
		t.Fatalf("NextUUID() error = %v", err)
	}
	return uuid
}

func TestNextUUIDUnique(t *testing.T) {
	f := NewFlacker(testConfig())
	seen := make(map[uint64]bool)
	var last uint64
	for i := 0; i < 100000; i++ {
		uuid := mustNextUUID(t, f)
		if seen[uuid] {
			t.Fatalf("duplicate uuid %d after %d uuids", uuid, i)
		}
		if uuid <= last {
			t.Fatalf("uuid %d is not greater than previous uuid %d", uuid, last)
		}
		seen[uuid] = true
		last = uuid
	}
}

func TestNextUUIDUniqueConcurrent(t *testing.T) {
	f := NewFlacker(testConfig())
	const goroutines, perGoroutine = 8, 10000
	results := make([][]uint64, goroutines)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				uuid, err := f.NextUUID()
				if err != nil {
					t.Errorf("NextUUID() error = %v", err)
					return
				}
				results[g] = append(results[g], uuid)
			}
		}(g)
	}
	wg.Wait()

	seen := make(map[uint64]bool)
	for _, uuids := range results {
		for _, uuid := range uuids {
			if seen[uuid] {
				t.Fatalf("duplicate uuid %d", uuid)
			}
			seen[uuid] = true
		}
	}
}

func TestNextUUIDDecompose(t *testing.T) {
	cfg := testConfig()
	f := NewFlacker(cfg)
	now := time.UnixMilli(int64(cfg.Flake.Epoch) + 123456789)
	f.now = func() time.Time { return now }

	first := f.Decompose(mustNextUUID(t, f))
	second := f.Decompose(mustNextUUID(t, f))

	want := map[string]uint64{"msb": 0, "time": 123456789, "datacenter_id": 3, "machine_id": 17, "sequence": 0}
	for k, v := range want {
		if first[k] != v {
			t.Errorf("first uuid %s = %d, want %d", k, first[k], v)
		}
	}
	if second["time"] != 123456789 || second["sequence"] != 1 {
		t.Errorf("second uuid time = %d sequence = %d, want time = 123456789 sequence = 1", second["time"], second["sequence"])
	}
}

func TestNextUUIDSequenceExhausted(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.BitsLen.Sequence = 2 // 4 uuids per millisecond
	cfg.Flake.BitsLen.Time = 51
	f := NewFlacker(cfg)
	// every clock read advances by a tenth of a millisecond
	f.now = fakeClock(time.UnixMilli(int64(cfg.Flake.Epoch)+1000), 100*time.Microsecond)

	seen := make(map[uint64]bool)
	for i := 0; i < 50; i++ {
		uuid := mustNextUUID(t, f)
		if seen[uuid] {
			t.Fatalf("duplicate uuid %d after %d uuids", uuid, i)
		}
		seen[uuid] = true
		if seq := f.Decompose(uuid)["sequence"]; seq > 3 {
			t.Fatalf("sequence %d overflows its 2 bits", seq)
		}
	}
}

func TestNextUUIDClockBackwards(t *testing.T) {
	cfg := testConfig()
	f := NewFlacker(cfg)
	now := time.UnixMilli(int64(cfg.Flake.Epoch) + 5000)
	f.now = func() time.Time { return now }

	before := mustNextUUID(t, f)
	now = now.Add(-2 * time.Second)
	after := mustNextUUID(t, f)
	if after <= before {
		t.Fatalf("uuid %d after clock moved backwards is not greater than %d", after, before)
	}
}

func TestNextUUIDTickMs(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.TickMs = 10
	f := NewFlacker(cfg)
	now := time.UnixMilli(int64(cfg.Flake.Epoch) + 12345)
	f.now = func() time.Time { return now }

	if got := f.Decompose(mustNextUUID(t, f))["time"]; got != 1234 {
		t.Fatalf("time = %d, want 1234", got)
	}
}

func TestNextUUIDClockBackwardsSequenceExhausted(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.BitsLen.Sequence = 2 // 4 uuids per millisecond
	cfg.Flake.BitsLen.Time = 51
	f := NewFlacker(cfg)
	now := time.UnixMilli(int64(cfg.Flake.Epoch) + 3600_000)
	f.now = func() time.Time { return now }
	f.sleep = func(d time.Duration) { now = now.Add(d) }

	last := mustNextUUID(t, f)
	// the clock jumps back by more than NextUUID is willing to wait for
	now = now.Add(-time.Minute)
	for i := 0; i < 3; i++ {
		uuid := mustNextUUID(t, f)
		if uuid <= last {
			t.Fatalf("uuid %d is not greater than previous uuid %d", uuid, last)
		}
		last = uuid
	}
	// the sequence of the last timestamp is used up, fail instead of blocking for a minute
	for i := 0; i < 2; i++ {
		if _, err := f.NextUUID(); !errors.Is(err, ErrClockMovedBackwards) {
			t.Fatalf("NextUUID() error = %v, want %v", err, ErrClockMovedBackwards)
		}
	}
	// once the clock caught up uuids are issued again, still in order
	now = now.Add(time.Minute + time.Millisecond)
	if uuid := mustNextUUID(t, f); uuid <= last {
		t.Fatalf("uuid %d is not greater than previous uuid %d", uuid, last)
	}
}

func TestNextUUIDClockSlightlyBackwardsWaits(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.BitsLen.Sequence = 0 // 1 uuid per millisecond
	cfg.Flake.BitsLen.Time = 53
	f := NewFlacker(cfg)
	now := time.UnixMilli(int64(cfg.Flake.Epoch) + 5000)
	f.now = func() time.Time { return now }
	f.sleep = func(d time.Duration) { now = now.Add(d) }

	first := mustNextUUID(t, f)
	now = now.Add(-200 * time.Millisecond)
	second := mustNextUUID(t, f)
	if got := f.Decompose(second)["time"]; second <= first || got != 5001 {
		t.Fatalf("second uuid = %d with time %d, want it greater than %d with time 5001", second, got, first)
	}
}

func TestNextUUIDTimeOverflow(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.BitsLen.Time = 10 // overflows 1024ms after the epoch
	cfg.Flake.BitsLen.Sequence = 43
	f := NewFlacker(cfg)
	now := time.UnixMilli(int64(cfg.Flake.Epoch) + 1023)
	f.now = func() time.Time { return now }

	uuid := mustNextUUID(t, f)
	if d := f.Decompose(uuid); d["msb"] != 0 || d["time"] != 1023 {
		t.Fatalf("msb = %d time = %d, want msb = 0 time = 1023", d["msb"], d["time"])
	}
	now = now.Add(time.Millisecond)
	if uuid, err := f.NextUUID(); !errors.Is(err, ErrTimeOverflow) {
		t.Fatalf("NextUUID() = %d, %v, want %v", uuid, err, ErrTimeOverflow)
	}
}

func TestNextUUIDClockBeforeEpoch(t *testing.T) {
	cfg := testConfig()
	f := NewFlacker(cfg)
	f.now = func() time.Time { return time.UnixMilli(int64(cfg.Flake.Epoch) - 1) }

	if uuid, err := f.NextUUID(); !errors.Is(err, ErrClockBeforeEpoch) {
		t.Fatalf("NextUUID() = %d, %v, want %v", uuid, err, ErrClockBeforeEpoch)
	}
}

func TestDecomposeTimeExcludesMSB(t *testing.T) {
	f := NewFlacker(testConfig())
	d := f.Decompose(1<<63 | 5<<22)
	if d["msb"] != 1 || d["time"] != 5 {
		t.Fatalf("msb = %d time = %d, want msb = 1 time = 5", d["msb"], d["time"])
	}
}

func TestTime(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.TickMs = 10
	f := NewFlacker(cfg)

	got, ok := f.Time(1234)
	if want := time.UnixMilli(int64(cfg.Flake.Epoch) + 12340); !ok || !got.Equal(want) {
		t.Fatalf("Time(1234) = %v, %v, want %v", got, ok, want)
	}
	for _, timestamp := range []uint64{1 << 62, 1 << 50} {
		if got, ok := f.Time(timestamp); ok {
			t.Errorf("Time(%d) = %v, want it out of range", timestamp, got)
		}
	}
}
