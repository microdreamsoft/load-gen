package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
)

// Config holds all tunable parameters for the load generator.
type Config struct {
	// CPUThreshold is the target system CPU usage percentage (0-100).
	// If actual CPU usage < CPUThreshold, CPU load is generated.
	CPUThreshold float64 `json:"cpu_threshold"`
	// MemThreshold is the target system memory usage percentage (0-100).
	MemThreshold float64 `json:"mem_threshold"`
	// Interval is the control-loop sampling interval in seconds.
	Interval float64 `json:"interval"`
	// ConfigFile path to a JSON config file, if any.
	ConfigFile string `json:"config_file"`
	// LogFile path to write verbose output to (falls back to stdout).
	LogFile string `json:"log_file"`
}

// DefaultConfig returns the built-in defaults.
func DefaultConfig() *Config {
	return &Config{
		CPUThreshold: 70,
		MemThreshold: 70,
		Interval:     1,
	}
}

// LoadConfig resolves configuration with precedence:
// defaults < config file < environment variables < command-line flags.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// 1. Parse flags to discover config file (and everything else).
	flagCPU := flag.Float64("cpu", 0, "target system CPU usage percentage (0-100)")
	flagMem := flag.Float64("mem", 0, "target system memory usage percentage (0-100)")
	flagInterval := flag.Duration("interval", 0, "control-loop sampling interval (e.g. 1s)")
	flagConfig := flag.String("config", "", "path to JSON config file")
	flagLog := flag.String("log", "", "path to log file (default: stdout)")
	flagV := flag.Bool("v", false, "verbose logging")
	flag.Parse()
	vFlag = *flagV

	// 2. Load config file (base layer).
	if *flagConfig != "" {
		if err := cfg.LoadFile(*flagConfig); err != nil {
			return nil, err
		}
	}

	// 3. Environment variables override the file.
	cfg.ApplyEnv()

	// 4. Flags override everything.
	if *flagLog != "" {
		cfg.LogFile = *flagLog
	}
	if *flagCPU > 0 {
		cfg.CPUThreshold = *flagCPU
	}
	if *flagMem > 0 {
		cfg.MemThreshold = *flagMem
	}
	if *flagInterval > 0 {
		cfg.Interval = flagInterval.Seconds()
	}

	return cfg, nil
}

// LoadFile reads and applies a JSON config file.
func (c *Config) LoadFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(raw, c); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	return nil
}

// ApplyEnv reads LOADGEN_* environment variables.
func (c *Config) ApplyEnv() {
	if v := os.Getenv("LOADGEN_CPU"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.CPUThreshold = f
		}
	}
	if v := os.Getenv("LOADGEN_MEM"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.MemThreshold = f
		}
	}
	if v := os.Getenv("LOADGEN_CONFIG"); v != "" {
		_ = c.LoadFile(v)
	}
	if v := os.Getenv("LOADGEN_LOG"); v != "" {
		c.LogFile = v
	}
}

// Validate clamps thresholds into [0, 100].
func (c *Config) Validate() {
	if c.CPUThreshold < 0 {
		c.CPUThreshold = 0
	}
	if c.CPUThreshold > 100 {
		c.CPUThreshold = 100
	}
	if c.MemThreshold < 0 {
		c.MemThreshold = 0
	}
	if c.MemThreshold > 100 {
		c.MemThreshold = 100
	}
	if c.Interval <= 0 {
		c.Interval = 1
	}
}
