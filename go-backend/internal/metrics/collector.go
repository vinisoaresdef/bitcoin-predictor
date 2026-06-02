// Package metrics provides low-overhead, thread-safe collection of host,
// runtime and process telemetry for an internal observability endpoint.
//
// The collector caches expensive readings (Linux /proc files and the
// stop-the-world runtime.ReadMemStats call) for a configurable TTL so a burst
// of concurrent requests triggers at most one real collection per interval.
// Reads are lock-free on the warm path: a stale snapshot is served immediately
// while a single goroutine refreshes in the background (single-flight via
// TryLock), so the calling request goroutine is never blocked on I/O.
package metrics

import (
	"bufio"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SystemHardware holds OS/host-level metrics sourced from /proc.
type SystemHardware struct {
	CPU         CPUStats    `json:"cpu"`
	Memory      MemoryStats `json:"memory"`
	LoadAverage LoadAverage `json:"load_average"`
}

// CPUStats reports CPU utilisation since the previous collection, split into
// user-space and kernel/system time. Values are percentages (0-100, aggregated
// across all cores). On the very first collection there is no previous sample
// to diff against, so all values are 0.
type CPUStats struct {
	UserPercent   float64 `json:"user_percent"`
	SystemPercent float64 `json:"system_percent"`
	TotalPercent  float64 `json:"total_percent"`
	Cores         int     `json:"cores"`
}

// MemoryStats reports host memory and the current process resident set size.
type MemoryStats struct {
	OSTotalBytes     uint64 `json:"os_total_bytes"`
	OSFreeBytes      uint64 `json:"os_free_bytes"`
	OSAvailableBytes uint64 `json:"os_available_bytes"`
	OSUsedBytes      uint64 `json:"os_used_bytes"`
	ProcessRSSBytes  uint64 `json:"process_rss_bytes"`
}

// LoadAverage mirrors the three figures from /proc/loadavg.
type LoadAverage struct {
	One     float64 `json:"1m"`
	Five    float64 `json:"5m"`
	Fifteen float64 `json:"15m"`
}

// RuntimeInternals holds Go runtime metrics: heap, GC and scheduler.
type RuntimeInternals struct {
	Heap      HeapStats      `json:"heap"`
	GC        GCStats        `json:"gc"`
	Scheduler SchedulerStats `json:"scheduler"`
}

// HeapStats reports heap usage. AllocBytes is the live (in-use) heap; SysBytes
// is what the heap has reserved from the OS; TotalSysBytes is the whole
// process footprint obtained from the OS across all runtime arenas.
type HeapStats struct {
	AllocBytes        uint64 `json:"alloc_bytes"`
	InUseBytes        uint64 `json:"in_use_bytes"`
	SysBytes          uint64 `json:"sys_bytes"`
	TotalSysBytes     uint64 `json:"total_sys_bytes"`
	NextGCTargetBytes uint64 `json:"next_gc_target_bytes"`
}

// GCStats reports garbage collector activity.
type GCStats struct {
	NumGC          uint32  `json:"num_gc"`
	PauseTotalNs   uint64  `json:"pause_total_ns"`
	PauseTotalMs   float64 `json:"pause_total_ms"`
	CPUFraction    float64 `json:"cpu_fraction"`
	LastGCUnixNano uint64  `json:"last_gc_unix_nano"`
}

// SchedulerStats reports concurrency counts.
type SchedulerStats struct {
	Goroutines int `json:"goroutines"`
	OSThreads  int `json:"os_threads"`
	GOMAXPROCS int `json:"gomaxprocs"`
	NumCPU     int `json:"num_cpu"`
}

// Collector gathers and caches telemetry. It is safe for concurrent use.
type Collector struct {
	ttl  time.Duration
	snap atomic.Pointer[cachedSnapshot]

	// refreshMu serialises refreshes (single-flight) and guards prevCPU.
	refreshMu  sync.Mutex
	prevCPU    cpuTimes
	hasPrevCPU bool
}

type cachedSnapshot struct {
	at      time.Time
	system  SystemHardware
	runtime RuntimeInternals
}

// NewCollector returns a Collector that caches readings for ttl. A ttl <= 0
// defaults to 1 second.
func NewCollector(ttl time.Duration) *Collector {
	if ttl <= 0 {
		ttl = time.Second
	}
	return &Collector{ttl: ttl}
}

// Collect returns the latest system and runtime metrics. It never blocks the
// caller once a snapshot exists: a fresh snapshot is returned directly, a stale
// one triggers a single background-style refresh (other concurrent callers get
// the stale copy), and only the very first cold call blocks to build the
// initial snapshot.
func (c *Collector) Collect() (SystemHardware, RuntimeInternals) {
	cur := c.snap.Load()
	if cur != nil && time.Since(cur.at) < c.ttl {
		return cur.system, cur.runtime
	}

	if cur == nil {
		// Cold start: block (only ever happens on the first request) so the
		// caller gets real data instead of zeros.
		c.refreshMu.Lock()
		defer c.refreshMu.Unlock()
		if got := c.snap.Load(); got != nil {
			return got.system, got.runtime
		}
		return c.refreshLocked()
	}

	// Warm but stale: try to become the single refresher without blocking.
	if c.refreshMu.TryLock() {
		defer c.refreshMu.Unlock()
		if got := c.snap.Load(); got != nil && time.Since(got.at) < c.ttl {
			return got.system, got.runtime // refreshed while we acquired the lock
		}
		return c.refreshLocked()
	}

	// Another goroutine is already refreshing — serve the stale snapshot now.
	return cur.system, cur.runtime
}

// refreshLocked recomputes the snapshot. Caller must hold refreshMu.
func (c *Collector) refreshLocked() (SystemHardware, RuntimeInternals) {
	sys := c.readSystemLocked()
	rt := readRuntime()
	c.snap.Store(&cachedSnapshot{at: time.Now(), system: sys, runtime: rt})
	return sys, rt
}

// readSystemLocked reads host metrics from /proc. Caller must hold refreshMu
// because it diffs against prevCPU. Individual read failures degrade to zero
// values rather than failing the whole snapshot.
func (c *Collector) readSystemLocked() SystemHardware {
	var sys SystemHardware
	sys.CPU.Cores = runtime.NumCPU()

	if runtime.GOOS != "linux" {
		// /proc is Linux-only; other platforms get runtime metrics only.
		return sys
	}

	if cur, err := readProcStat(); err == nil {
		if c.hasPrevCPU {
			sys.CPU.UserPercent, sys.CPU.SystemPercent, sys.CPU.TotalPercent = cpuDelta(c.prevCPU, cur)
		}
		c.prevCPU = cur
		c.hasPrevCPU = true
	}

	if total, free, avail, err := readMemInfo(); err == nil {
		sys.Memory.OSTotalBytes = total
		sys.Memory.OSFreeBytes = free
		sys.Memory.OSAvailableBytes = avail
		if total >= avail {
			sys.Memory.OSUsedBytes = total - avail
		}
	}

	if rss, err := readProcessRSS(); err == nil {
		sys.Memory.ProcessRSSBytes = rss
	}

	if one, five, fifteen, err := readLoadAvg(); err == nil {
		sys.LoadAverage = LoadAverage{One: one, Five: five, Fifteen: fifteen}
	}

	return sys
}

// readRuntime gathers Go runtime metrics. ReadMemStats briefly stops the world,
// but the Collector cache bounds it to at most once per TTL.
func readRuntime() RuntimeInternals {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	threads := 0
	if p := pprof.Lookup("threadcreate"); p != nil {
		threads = p.Count()
	}

	return RuntimeInternals{
		Heap: HeapStats{
			AllocBytes:        m.HeapAlloc,
			InUseBytes:        m.HeapInuse,
			SysBytes:          m.HeapSys,
			TotalSysBytes:     m.Sys,
			NextGCTargetBytes: m.NextGC,
		},
		GC: GCStats{
			NumGC:          m.NumGC,
			PauseTotalNs:   m.PauseTotalNs,
			PauseTotalMs:   float64(m.PauseTotalNs) / 1e6,
			CPUFraction:    m.GCCPUFraction,
			LastGCUnixNano: m.LastGC,
		},
		Scheduler: SchedulerStats{
			Goroutines: runtime.NumGoroutine(),
			OSThreads:  threads,
			GOMAXPROCS: runtime.GOMAXPROCS(0),
			NumCPU:     runtime.NumCPU(),
		},
	}
}

// --- /proc parsers -------------------------------------------------------

// cpuTimes holds the jiffie counters needed to compute utilisation.
type cpuTimes struct {
	user   uint64 // user + nice
	system uint64 // system + irq + softirq
	idle   uint64 // idle + iowait
	total  uint64 // sum of all fields
}

// cpuDelta returns user%, system% and total-busy% between two samples.
// Busy time is everything that is not idle/iowait, so it captures steal and
// guest time too, not just user+system.
func cpuDelta(prev, cur cpuTimes) (userPct, sysPct, totalPct float64) {
	dTotal := float64(cur.total - prev.total)
	if dTotal <= 0 {
		return 0, 0, 0
	}
	dIdle := float64(cur.idle - prev.idle)
	userPct = float64(cur.user-prev.user) / dTotal * 100
	sysPct = float64(cur.system-prev.system) / dTotal * 100
	totalPct = (dTotal - dIdle) / dTotal * 100
	return userPct, sysPct, totalPct
}

// readProcStat parses the aggregate "cpu" line of /proc/stat.
// Fields: user nice system idle iowait irq softirq steal guest guest_nice.
func readProcStat() (cpuTimes, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:] // drop the "cpu" label
		vals := make([]uint64, 0, len(fields))
		for _, s := range fields {
			v, _ := strconv.ParseUint(s, 10, 64)
			vals = append(vals, v)
		}
		var t cpuTimes
		for _, v := range vals {
			t.total += v
		}
		get := func(i int) uint64 {
			if i < len(vals) {
				return vals[i]
			}
			return 0
		}
		t.user = get(0) + get(1)            // user + nice
		t.system = get(2) + get(5) + get(6) // system + irq + softirq
		t.idle = get(3) + get(4)            // idle + iowait
		return t, nil
	}
	if err := sc.Err(); err != nil {
		return cpuTimes{}, err
	}
	return cpuTimes{}, nil
}

// readMemInfo parses MemTotal, MemFree and MemAvailable (bytes) from /proc/meminfo.
func readMemInfo() (total, free, available uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		kb, _ := strconv.ParseUint(fields[1], 10, 64)
		bytes := kb * 1024
		switch fields[0] {
		case "MemTotal:":
			total = bytes
		case "MemFree:":
			free = bytes
		case "MemAvailable:":
			available = bytes
		}
	}
	return total, free, available, sc.Err()
}

// readProcessRSS returns the resident set size of the current process in bytes,
// read from /proc/self/statm (field 2 = resident pages).
func readProcessRSS() (uint64, error) {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, nil
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return pages * uint64(os.Getpagesize()), nil
}

// readLoadAvg parses the 1/5/15 minute figures from /proc/loadavg.
func readLoadAvg() (one, five, fifteen float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, nil
	}
	one, _ = strconv.ParseFloat(fields[0], 64)
	five, _ = strconv.ParseFloat(fields[1], 64)
	fifteen, _ = strconv.ParseFloat(fields[2], 64)
	return one, five, fifteen, nil
}
