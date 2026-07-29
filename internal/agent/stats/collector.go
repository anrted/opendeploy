// Package stats implements the system metrics collector for the OpenDeploy Agent.
//
// It reads directly from the Linux /proc and /sys virtual filesystems to gather
// CPU, memory, disk, network and load statistics without any external dependencies.
// This keeps the Agent binary small and avoids CGO where possible.
package stats

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
	"github.com/shirou/gopsutil/v4/process"
)

// Collector gathers system statistics.
// It maintains previous network byte counts to compute deltas.
type Collector struct {
	mu            sync.Mutex
	prevNetRx     uint64
	prevNetTx     uint64
	prevStatIdle  uint64
	prevStatTotal uint64
	lastCollect   time.Time
}

// NewCollector creates a new Collector and primes the delta counters.
func NewCollector() *Collector {
	c := &Collector{}
	// Prime delta counters so the first Collect() returns meaningful values.
	c.prevNetRx, c.prevNetTx, _ = readNetworkBytes()
	c.prevStatIdle, c.prevStatTotal, _ = readCPUStat()
	c.lastCollect = time.Now()
	return c
}

// Collect gathers a snapshot of all system metrics.
func (c *Collector) Collect() (*contract.SystemStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(c.lastCollect).Seconds()
	if elapsed < 0.1 {
		elapsed = 1
	}
	c.lastCollect = now

	stats := &contract.SystemStats{}

	// ── CPU ───────────────────────────────────────────────────────────────
	idle, total, err := readCPUStat()
	if err == nil {
		idleDelta := idle - c.prevStatIdle
		totalDelta := total - c.prevStatTotal
		if totalDelta > 0 {
			stats.CPU.UsagePercent = (1 - float64(idleDelta)/float64(totalDelta)) * 100
		}
		c.prevStatIdle = idle
		c.prevStatTotal = total
	}
	stats.CPU.Cores = runtime.NumCPU()

	// ── Memory ────────────────────────────────────────────────────────────
	memTotal, memAvail, err := readMemInfo()
	if err == nil {
		stats.Memory.Total = memTotal
		stats.Memory.Free = memAvail
		stats.Memory.Used = memTotal - memAvail
		if memTotal > 0 {
			stats.Memory.UsedPercent = float64(stats.Memory.Used) / float64(memTotal) * 100
		}
	}

	// ── Swap ──────────────────────────────────────────────────────────────
	swapTotal, swapFree, err := readSwapInfo()
	if err == nil {
		stats.Swap.Total = swapTotal
		stats.Swap.Free = swapFree
		stats.Swap.Used = swapTotal - swapFree
		if swapTotal > 0 {
			stats.Swap.UsedPercent = float64(stats.Swap.Used) / float64(swapTotal) * 100
		}
	}

	// ── Disk ──────────────────────────────────────────────────────────────
	stats.Disk, _ = readDiskStats()

	// ── Network ───────────────────────────────────────────────────────────
	netRx, netTx, err := readNetworkBytes()
	if err == nil {
		stats.Network.BytesRecv = netRx
		stats.Network.BytesSent = netTx
		c.prevNetRx = netRx
		c.prevNetTx = netTx
	}

	// ── Load average ──────────────────────────────────────────────────────
	stats.LoadAverage, _ = readLoadAverage()

	// ── Uptime ────────────────────────────────────────────────────────────
	stats.Uptime, _ = readUptime()

	// ── CPU temperature ───────────────────────────────────────────────────
	stats.Temperature, _ = readCPUTemperature()

	return stats, nil
}

// ─── /proc/stat — CPU ──────────────────────────────────────────────────────

// readCPUStat returns total idle and total CPU ticks from /proc/stat.
func readCPUStat() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return 0, 0, fmt.Errorf("unexpected /proc/stat format")
		}
		// Fields: cpu user nice system idle iowait irq softirq steal
		var vals [8]uint64
		for i := 1; i <= 8; i++ {
			vals[i-1], _ = strconv.ParseUint(fields[i], 10, 64)
		}
		idle = vals[3] + vals[4] // idle + iowait
		for _, v := range vals {
			total += v
		}
		return idle, total, nil
	}
	return 0, 0, fmt.Errorf("cpu line not found in /proc/stat")
}

// ─── /proc/meminfo ─────────────────────────────────────────────────────────

// readMemInfo returns total and available memory in bytes.
func readMemInfo() (total, available uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var memAvailable bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemKB(line) * 1024
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemKB(line) * 1024
			memAvailable = true
		}
		if total > 0 && memAvailable {
			break
		}
	}
	return total, available, nil
}

func readSwapInfo() (total, free uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "SwapTotal:"):
			total = parseMemKB(line) * 1024
		case strings.HasPrefix(line, "SwapFree:"):
			free = parseMemKB(line) * 1024
		}
	}
	return total, free, nil
}

func parseMemKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}

// ─── /proc/mounts + syscall.Statfs — Disk ──────────────────────────────────

func readDiskStats() ([]contract.DiskStats, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	var disks []contract.DiskStats

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		fstype := ""
		if len(fields) >= 3 {
			fstype = fields[2]
		}
		// Skip pseudo-filesystems.
		if isPseudoFS(fstype) {
			continue
		}
		mount := fields[1]
		if _, ok := seen[mount]; ok {
			continue
		}
		seen[mount] = struct{}{}

		// Use os.Stat to check existence and then a separate approach for disk stats.
		d, err := getDiskUsage(mount)
		if err != nil {
			continue
		}
		disks = append(disks, *d)
	}
	return disks, nil
}

func isPseudoFS(fstype string) bool {
	pseudo := map[string]bool{
		"proc": true, "sysfs": true, "devpts": true, "tmpfs": true,
		"cgroup": true, "cgroup2": true, "devtmpfs": true, "hugetlbfs": true,
		"mqueue": true, "debugfs": true, "securityfs": true, "configfs": true,
		"fusectl": true, "pstore": true, "efivarfs": true, "bpf": true,
		"autofs": true, "ramfs": true,
	}
	return pseudo[fstype]
}

// ─── /proc/net/dev — Network ───────────────────────────────────────────────

func readNetworkBytes() (rx, tx uint64, err error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Skip 2 header lines.
	scanner.Scan()
	scanner.Scan()
	for scanner.Scan() {
		line := scanner.Text()
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:colonIdx])
		// Skip loopback.
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(line[colonIdx+1:])
		if len(fields) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
	}
	return rx, tx, nil
}

// ─── /proc/loadavg ─────────────────────────────────────────────────────────

func readLoadAverage() ([3]float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return [3]float64{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return [3]float64{}, fmt.Errorf("unexpected /proc/loadavg format")
	}
	var la [3]float64
	la[0], _ = strconv.ParseFloat(fields[0], 64)
	la[1], _ = strconv.ParseFloat(fields[1], 64)
	la[2], _ = strconv.ParseFloat(fields[2], 64)
	return la, nil
}

// ─── /proc/uptime ──────────────────────────────────────────────────────────

func readUptime() (int64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("unexpected /proc/uptime format")
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

// ─── /sys/class/thermal — CPU temperature ─────────────────────────────────

func readCPUTemperature() (float64, error) {
	// Try common thermal zone paths.
	patterns := []string{
		"/sys/class/thermal/thermal_zone*/temp",
		"/sys/class/hwmon/hwmon*/temp1_input",
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			continue
		}
		data, err := os.ReadFile(matches[0])
		if err != nil {
			continue
		}
		milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			continue
		}
		return float64(milliC) / 1000.0, nil
	}
	return 0, nil
}

// ─── Disk usage via /proc/mounts + statfs workaround ─────────────────────

// getDiskUsage returns disk usage for the given mount point.
// We read /proc/diskstats is complex; instead we use a simple fallback using df.
// For a pure-Go solution without CGO or syscall.Statfs (which IS available on Linux):
func getDiskUsage(mount string) (*contract.DiskStats, error) {
	// Read from /sys/fs/... isn't reliable; use a stat-based approach.
	// syscall.Statfs is available on Linux without CGO.
	// We use it via the os package since Go 1.21+ wraps it.
	var stat statfsResult
	if err := statfs(mount, &stat); err != nil {
		return nil, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	var usedPct float64
	if total > 0 {
		usedPct = float64(used) / float64(total) * 100
	}
	return &contract.DiskStats{
		Mountpoint:  mount,
		Total:       total,
		Used:        used,
		Free:        free,
		UsedPercent: usedPct,
	}, nil
}

// ─── Process Collection (using gopsutil) ───────────────────────────────────

func (c *Collector) CollectProcesses() ([]contract.ProcessStats, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var result []contract.ProcessStats
	for _, p := range procs {
		name, _ := p.Name()
		if name == "" {
			continue
		}

		ppid, _ := p.Ppid()
		user, _ := p.Username()
		cpu, _ := p.CPUPercent()
		mem, _ := p.MemoryPercent()

		var rss uint64
		if memInfo, err := p.MemoryInfo(); err == nil {
			rss = memInfo.RSS
		}

		threads, _ := p.NumThreads()
		createTime, _ := p.CreateTime()
		cmd, _ := p.Cmdline()

		result = append(result, contract.ProcessStats{
			Pid:        int(p.Pid),
			Ppid:       int(ppid),
			Name:       name,
			User:       user,
			CpuPercent: cpu,
			MemPercent: float64(mem),
			MemRss:     rss,
			NumThreads: int(threads),
			CreateTime: createTime,
			Cmdline:    cmd,
		})
	}
	return result, nil
}

func (c *Collector) KillProcess(pid int32, force bool) error {
	if reason := protectedProcessReason(pid); reason != "" {
		return fmt.Errorf("%w: %s", ErrProtectedProcess, reason)
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}
	if force {
		return p.Kill()
	}
	return p.Terminate()
}
