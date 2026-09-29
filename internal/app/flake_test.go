package app

import (
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

func TestNextUUIDUnique(t *testing.T) {
	f := NewFlacker(testConfig())
	seen := make(map[uint64]bool)
	var last uint64
	for i := 0; i < 100000; i++ {
		uuid := f.NextUUID()
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
				results[g] = append(results[g], f.NextUUID())
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

	first := f.Decompose(f.NextUUID())
	second := f.Decompose(f.NextUUID())

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
		uuid := f.NextUUID()
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

	before := f.NextUUID()
	now = now.Add(-2 * time.Second)
	after := f.NextUUID()
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

	if got := f.Decompose(f.NextUUID())["time"]; got != 1234 {
		t.Fatalf("time = %d, want 1234", got)
	}
}
