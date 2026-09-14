package main

import (
	"fmt"
	"os"
	"runtime"
)

var verbose = false

var logOut *os.File

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		fatalf("configuration error: %v", err)
	}
	cfg.Validate()

	// Enable verbose logging via the -v flag (registered in LoadConfig).
	verbose = vFlag

	// Redirect verbose output to a log file when requested.
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fatalf("open log file %s: %v", cfg.LogFile, err)
		}
		logOut = f
	}

	printf("load-gen v0.2 | cpu_band=[%.0f%%-%.0f%%] mem_band=[%.0f%%-%.0f%%] interval=%.1fs physical_cores=%d logical=%d (verbose=%v)",
		cfg.CPULow, cfg.CPUHigh, cfg.MemLow, cfg.MemHigh, cfg.Interval, physicalCoreCount(), runtime.NumCPU(), verbose)

	Run(cfg)
}

var vFlag = false

func sprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}

func printf(format string, a ...interface{}) {
	writeStdout(format, a...)
}

// writeStdout writes synchronously so buffered redirection is flushed promptly.
func writeStdout(format string, a ...interface{}) {
	b := []byte(fmt.Sprintf(format, a...))
	if logOut != nil {
		logOut.Write(b)
		return
	}
	os.Stdout.Write(b)
}

func fatalf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}
