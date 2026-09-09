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

	cpuCores := runtime.NumCPU()
	currentWorkers := 0
	targetMemBytes := int64(0)

	for {
		<-tick.C
		metrics, err := sampleMetrics()
		if err != nil {
			tracef("sampling failed: %v", err)
			continue
		}

		// ---- CPU control ----
		// If observed system CPU is below threshold, spin up workers; adjust in
		// steps proportional to the error so we converge and overshoot minimally.
		delta := int((cfg.CPUThreshold - metrics.CPUPercent) / 10)
		next := currentWorkers + delta
		if next < 0 {
			next = 0
		}
		if next > cpuCores {
			next = cpuCores
		}
		if next != currentWorkers {
			cpuGen.Set(next)
			currentWorkers = next
			tracef("cpu: sys=%.1f%% thr=%.1f%% workers=%d", metrics.CPUPercent, cfg.CPUThreshold, currentWorkers)
		}

		// ---- Memory control ----
		// Allocate so that total used memory approaches the threshold. Baseline
		// used memory is approximated as current total minus what we hold.
		sysUsed := float64(int64(metrics.MemTotal)) * metrics.MemPercent / 100.0
		baseline := sysUsed - float64(targetMemBytes)
		thresholdBytes := float64(int64(metrics.MemTotal)) * cfg.MemThreshold / 100.0
		desiredHold := thresholdBytes - baseline
		if desiredHold < 0 {
			desiredHold = 0
		}
		newTarget := int64(desiredHold) / (1 << 20) * (1 << 20) // round to MiB
		// Damp overshoot-churn: only reallocate when moving down or by a big gap.
		if newTarget < targetMemBytes || newTarget-targetMemBytes > (24<<20) {
			memGen.Set(newTarget)
			targetMemBytes = newTarget
			tracef("mem: sys=%.1f%% thr=%.1f%% hold=%d MiB", metrics.MemPercent, cfg.MemThreshold, targetMemBytes>>20)
		}

		if currentWorkers == 0 && targetMemBytes == 0 {
			tracef("idle: cpu=%.1f%% mem=%.1f%% (at/above thresholds)", metrics.CPUPercent, metrics.MemPercent)
		}
	}
}

func tracef(format string, a ...interface{}) {
	if verbose {
		printf(format+"\n", a...)
	}
}
