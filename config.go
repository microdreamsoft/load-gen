package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all tunable parameters for the load generator.
type Config struct {
	// CPULow / CPUHigh define the acceptable system CPU band (0-100).
	// If actual CPU < CPULow, CPU load is generated; if > CPUHigh, it is released.
	CPULow  float64 `json:"cpu_low"`
	CPUHigh float64 `json:"cpu_high"`
	// MemLow / MemHigh define the acceptable system memory band (0-100).
	MemLow  float64 `json:"mem_low"`
	MemHigh float64 `json:"mem_high"`
	// Interval is the control-loop sampling interval in seconds.
	Interval float64 `json:"interval"`
	// ConfigFile path to a JSON config file, if any.
	ConfigFile string `json:"config_file"`
	// LogFile path to write verbose output to (falls back to stdout).
	LogFile string `json:"log_file"`
}

// configFile mirrors the on-disk JSON schema, using range strings for cpu/mem.
type configFile struct {
	CPU      string  `json:"cpu"`
	Mem      string  `json:"mem"`
	Interval float64 `json:"interval"`
	LogFile  string  `json:"log_file"`
}

// DefaultConfig returns the built-in defaults.
func DefaultConfig() *Config {
	return &Config{
		CPULow:  50,
		CPUHigh: 80,
		MemLow:  40,
		MemHigh: 80,
		Interval: 1,
	}
}

// parseRange parses "min-max" (e.g. "50-70") or "value" (e.g. "70"),
// returning (low, high). A bare value yields low == high.
func parseRange(s string) (float64, float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("empty range")
	}
	parts := strings.Split(s, "-")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("invalid range %q (expected \"min-max\")", s)
	}
	parse := func(v string) (float64, error) {
		if v == "" {
			return 0, fmt.Errorf("invalid range %q", s)
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid range %q: %v", s, err)
		}
		return f, nil
	}
	low, err := parse(parts[0])
	if err != nil {
		return 0, 0, err
	}
	high := low
	if len(parts) == 2 {
		high, err = parse(parts[1])
		if err != nil {
			return 0, 0, err
		}
	}
	if low > high {
		return 0, 0, fmt.Errorf("invalid range %q: low > high", s)
	}
	return low, high, nil
}

// setRange applies a parsed range string to the given low/high fields.
func (c *Config) setRange(s, field string) error {
	low, high, err := parseRange(s)
	if err != nil {
		return err
	}
	switch field {
	case "cpu":
		c.CPULow, c.CPUHigh = low, high
	case "mem":
		c.MemLow, c.MemHigh = low, high
	}
	return nil
}

// LoadConfig resolves configuration with precedence:
// defaults < config file < environment variables < command-line flags.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// 1. Parse flags (cpu/mem accepted as range strings).
	flagCPU := flag.String("cpu", "", "target system CPU usage band, e.g. \"50-70\"")
	flagMem := flag.String("mem", "", "target system memory usage band, e.g. \"40-70\"")
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
	if *flagCPU != "" {
		if err := cfg.setRange(*flagCPU, "cpu"); err != nil {
			return nil, err
		}
	}
	if *flagMem != "" {
		if err := cfg.setRange(*flagMem, "mem"); err != nil {
			return nil, err
		}
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
	var cf configFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	if cf.CPU != "" {
		if err := c.setRange(cf.CPU, "cpu"); err != nil {
			return fmt.Errorf("config file %s: %w", path, err)
		}
	}
	if cf.Mem != "" {
		if err := c.setRange(cf.Mem, "mem"); err != nil {
			return fmt.Errorf("config file %s: %w", path, err)
		}
	}
	if cf.Interval > 0 {
		c.Interval = cf.Interval
	}
	if cf.LogFile != "" {
		c.LogFile = cf.LogFile
	}
	return nil
}

// ApplyEnv reads LOADGEN_* environment variables (cpu/mem as ranges).
func (c *Config) ApplyEnv() {
	if v := os.Getenv("LOADGEN_CPU"); v != "" {
		_ = c.setRange(v, "cpu")
	}
	if v := os.Getenv("LOADGEN_MEM"); v != "" {
		_ = c.setRange(v, "mem")
	}
	if v := os.Getenv("LOADGEN_CONFIG"); v != "" {
		_ = c.LoadFile(v)
	}
	if v := os.Getenv("LOADGEN_LOG"); v != "" {
		c.LogFile = v
	}
}

// Validate clamps the band boundaries into [0, 100] and normalizes the interval.
func (c *Config) Validate() {
	clamp := func(v *float64) {
		if *v < 0 {
			*v = 0
		}
		if *v > 100 {
			*v = 100
		}
	}
	clamp(&c.CPULow)
	clamp(&c.CPUHigh)
	clamp(&c.MemLow)
	clamp(&c.MemHigh)
	if c.Interval <= 0 {
		c.Interval = 1
	}
}
