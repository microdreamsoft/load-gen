package main

import (
	"sync"
)

// CPUWorker is a spinning goroutine that burns CPU cycles.
type CPUWorker struct {
	stop chan struct{}
}

// Stop signals the worker to exit.
func (w *CPUWorker) Stop() {
	close(w.stop)
}

// pump busy-loops until stopped.
func (w *CPUWorker) pump() {
	for {
		select {
		case <-w.stop:
			return
		default:
			// pure computation to consume a full core
		}
	}
}

// CPUGenerator manages a set of CPU-burning workers.
type CPUGenerator struct {
	mu      sync.Mutex
	workers []*CPUWorker
}

// Set adjusts the number of active workers to `n`. Increasing spins up new
// workers, decreasing stops the excess and releases the cores.
func (g *CPUGenerator) Set(n int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// stop excess workers
	for len(g.workers) > n {
		w := g.workers[len(g.workers)-1]
		g.workers = g.workers[:len(g.workers)-1]
		w.Stop()
	}

	// start new workers
	for len(g.workers) < n {
		w := &CPUWorker{stop: make(chan struct{})}
		g.workers = append(g.workers, w)
		go w.pump()
	}
}

// Stop releases all CPU load.
func (g *CPUGenerator) Stop() {
	g.Set(0)
}

// MemBlock is an allocated chunk of memory held to consume RAM.
type MemBlock struct {
	data []byte
}

// MemGenerator allocates and releases memory to consume RAM.
type MemGenerator struct {
	mu     sync.Mutex
	blocks []*MemBlock
}

// Set target total memory in bytes. Allocates or releases blocks to approach
// the target, holding allocations so the OS reports them as in use.
func (g *MemGenerator) Set(targetBytes int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	const chunk = int64(16 << 20) // 16 MiB per block

	total := int64(0)
	for _, b := range g.blocks {
		total += int64(len(b.data))
	}

	// allocate more until we reach (or overshoot) the target
	for total < targetBytes {
		size := chunk
		if remain := targetBytes - total; remain < size {
			size = remain
		}
		b := &MemBlock{data: make([]byte, size)}
		// touch pages so they are actually committed by the OS
		for i := 0; i < len(b.data); i += 4096 {
			b.data[i] = 1
		}
		g.blocks = append(g.blocks, b)
		total += int64(size)
	}

	// release blocks that overshoot the target
	for total > targetBytes && len(g.blocks) > 0 {
		g.blocks = g.blocks[:len(g.blocks)-1]
		total -= chunk
	}
}

// Total returns the currently held memory in bytes.
func (g *MemGenerator) Total() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	total := int64(0)
	for _, b := range g.blocks {
		total += int64(len(b.data))
	}
	return total
}

// Stop releases all memory.
func (g *MemGenerator) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.blocks = nil
}
