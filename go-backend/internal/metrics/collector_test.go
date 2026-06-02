package metrics

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestCollectReturnsRuntimeData(t *testing.T) {
	c := NewCollector(time.Second)

	_, rt := c.Collect()

	if rt.Scheduler.Goroutines <= 0 {
		t.Errorf("expected at least one goroutine, got %d", rt.Scheduler.Goroutines)
	}
	if rt.Scheduler.NumCPU != runtime.NumCPU() {
		t.Errorf("NumCPU = %d, want %d", rt.Scheduler.NumCPU, runtime.NumCPU())
	}
	if rt.Scheduler.GOMAXPROCS <= 0 {
		t.Errorf("expected GOMAXPROCS > 0, got %d", rt.Scheduler.GOMAXPROCS)
	}
	if rt.Heap.SysBytes == 0 {
		t.Error("expected non-zero heap sys bytes")
	}
}

func TestCollectReadsHostMetricsOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc metrics only available on linux")
	}

	c := NewCollector(time.Second)
	sys, _ := c.Collect()

	if sys.Memory.OSTotalBytes == 0 {
		t.Error("expected non-zero total OS memory from /proc/meminfo")
	}
	if sys.Memory.ProcessRSSBytes == 0 {
		t.Error("expected non-zero process RSS from /proc/self/statm")
	}
	if sys.CPU.Cores != runtime.NumCPU() {
		t.Errorf("CPU.Cores = %d, want %d", sys.CPU.Cores, runtime.NumCPU())
	}
}

func TestCPUPercentPopulatedAfterSecondSample(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc metrics only available on linux")
	}

	// A zero TTL is bumped to the 1s default, so force two refreshes by
	// reaching into the collector with a short TTL via a fresh instance.
	c := NewCollector(time.Millisecond)
	c.Collect() // first sample: no delta yet

	// Burn a little CPU so the delta is observable, then wait past the TTL.
	busyUntil := time.Now().Add(20 * time.Millisecond)
	for time.Now().Before(busyUntil) {
	}
	time.Sleep(5 * time.Millisecond)

	sys, _ := c.Collect()
	if sys.CPU.TotalPercent < 0 || sys.CPU.TotalPercent > 100*float64(sys.CPU.Cores) {
		t.Errorf("CPU total percent out of range: %f", sys.CPU.TotalPercent)
	}
	if sys.CPU.UserPercent < 0 || sys.CPU.SystemPercent < 0 {
		t.Errorf("negative CPU percentages: user=%f system=%f", sys.CPU.UserPercent, sys.CPU.SystemPercent)
	}
}

func TestCollectCachesWithinTTL(t *testing.T) {
	c := NewCollector(time.Hour) // effectively never expires during the test
	c.Collect()                  // build the first snapshot

	first := c.snap.Load()
	if first == nil {
		t.Fatal("expected a cached snapshot after first Collect")
	}

	c.Collect()
	second := c.snap.Load()
	if first != second {
		t.Error("expected the cached snapshot pointer to be reused within TTL (no recompute)")
	}
}

func TestCollectConcurrent(t *testing.T) {
	c := NewCollector(50 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sys, rt := c.Collect()
				if rt.Scheduler.NumCPU == 0 {
					t.Error("got empty runtime snapshot")
					return
				}
				_ = sys
			}
		}()
	}
	wg.Wait()
}
