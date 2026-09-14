package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// Metrics is a snapshot of system CPU and memory usage.
type Metrics struct {
	CPUPercent float64 // overall system CPU usage 0-100
	MemPercent float64 // overall system memory usage 0-100
	MemTotal   uint64  // total memory in bytes
}

// sampleMetrics collects current system usage.
func sampleMetrics() (*Metrics, error) {
	m := &Metrics{}

	if percents, err := cpu.Percent(0, false); err != nil {
		return nil, err
	} else if len(percents) > 0 {
		m.CPUPercent = percents[0]
	}

	if vm, err := mem.VirtualMemory(); err != nil {
		return nil, err
	} else {
		m.MemPercent = vm.UsedPercent
		m.MemTotal = vm.Total
	}

	return m, nil
}

// Run drives the control loop: sample system usage, then adjust the amount of
// generated load so that system usage stays near the configured thresholds.
func Run(cfg *Config) {
	cpuGen := &CPUGenerator{}
	memGen := &MemGenerator{}
	defer cpuGen.Stop()
	defer memGen.Stop()

	// Release load and exit on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cpuGen.Stop()
		memGen.Stop()
		os.Exit(0)
	}()

	interval := time.Duration(cfg.Interval * float64(time.Second))
	tick := time.NewTicker(interval)
	defer tick.Stop()

	cpuCores := physicalCoreCount()
	currentWorkers := 0
	targetMemBytes := int64(0)

	// Pin the Go runtime to physical cores. On Windows (hyperthreading),
	// GOMAXPROCS defaults to logical CPUs; leaving it high spreads each worker
	// across half-cores and undersaturates.
	runtime.GOMAXPROCS(cpuCores)

	var (
		smoothedCPU  float64
		prevWorkers  int
		prevSmoothed float64
		first        = true
	)

	for {
		<-tick.C
		metrics, err := sampleMetrics()
		if err != nil {
			tracef("sampling failed: %v", err)
			continue
		}

		// ---- CPU control (band with hysteresis, smoothed + self-stalling) ----
		// Below CPULow: ramp workers up. Above CPUHigh: release workers. Within
		// the band: hold current load.
		sysCPU := metrics.CPUPercent
		if first {
			smoothedCPU = sysCPU
			first = false
		}
		prevSmoothed = smoothedCPU
		smoothedCPU = smoothedCPU*0.4 + sysCPU*0.6

		switch {
		case smoothedCPU < cfg.CPULow:
			// Ramp up one worker at a time. If the last added worker did NOT
			// raise measured CPU (scheduler saturation / hyperthread collapse),
			// stop increasing — adding more burns cycles and can lower throughput.
			saturated := currentWorkers > prevWorkers && smoothedCPU < prevSmoothed-4
			if !saturated && currentWorkers < cpuCores {
				currentWorkers++
				cpuGen.Set(currentWorkers)
				tracef("cpu: sys=%.1f%% below band [%.0f-%.0f] -> workers=%d", sysCPU, cfg.CPULow, cfg.CPUHigh, currentWorkers)
			}
		case smoothedCPU > cfg.CPUHigh:
			// Release one worker until back inside the band.
			if currentWorkers > 0 {
				currentWorkers--
				cpuGen.Set(currentWorkers)
				tracef("cpu: sys=%.1f%% above band [%.0f-%.0f] -> workers=%d", sysCPU, cfg.CPULow, cfg.CPUHigh, currentWorkers)
			}
		default:
			tracef("cpu: sys=%.1f%% within band [%.0f-%.0f] hold workers=%d", sysCPU, cfg.CPULow, cfg.CPUHigh, currentWorkers)
		}
		prevWorkers = currentWorkers

		// ---- Memory control (band with hysteresis) ----
		// Below MemLow: allocate so system memory reaches MemLow. Above MemHigh:
		// release so it drops to MemHigh. Within the band: hold.
		sysUsed := float64(int64(metrics.MemTotal)) * metrics.MemPercent / 100.0
		baseline := sysUsed - float64(targetMemBytes)
		lowBytes := float64(int64(metrics.MemTotal)) * cfg.MemLow / 100.0
		highBytes := float64(int64(metrics.MemTotal)) * cfg.MemHigh / 100.0

		var newTarget int64
		switch {
		case sysUsed < lowBytes:
			hold := lowBytes - baseline
			if hold < 0 {
				hold = 0
			}
			newTarget = int64(hold)
		case sysUsed > highBytes:
			hold := highBytes - baseline
			if hold < 0 {
				hold = 0
			}
			newTarget = int64(hold)
		default:
			newTarget = targetMemBytes // within band: hold
		}

		if newTarget != targetMemBytes {
			memGen.Set(newTarget)
			targetMemBytes = newTarget
			tracef("mem: sys=%.1f%% band [%.0f-%.0f] hold=%d MiB", metrics.MemPercent, cfg.MemLow, cfg.MemHigh, targetMemBytes>>20)
		}

		if currentWorkers == 0 && targetMemBytes == 0 {
			tracef("idle: cpu=%.1f%% mem=%.1f%% (within band)", metrics.CPUPercent, metrics.MemPercent)
		}
	}
}

func tracef(format string, a ...interface{}) {
	if verbose {
		printf(format+"\n", a...)
	}
}

// physicalCoreCount returns the number of physical cores (not logical
// hyperthreaded processors). Falls back to the logical count when the OS
// cannot report physical cores.
func physicalCoreCount() int {
	if n, err := cpu.Counts(false); err == nil && n > 0 {
		return n
	}
	return runtime.NumCPU()
}
