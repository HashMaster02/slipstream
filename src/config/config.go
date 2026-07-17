// Package config defines the tunable settings for the backtest engine and
// loads them from an external JSON file. Every setting has a baked-in default
// (see Default), so a missing config file runs with the historical hardcoded
// values and a partial file only overrides the keys it specifies.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// DefaultPath is where the engine looks for a config file when none is given.
const DefaultPath = "./config.json"

// tickerPlaceholder is substituted with each symbol when building a quote path.
const tickerPlaceholder = "{ticker}"

// Latency configures the simulated order-submission delay. Delays are measured
// in ticks (bars), not wall-clock time: an order emitted on tick T is not added
// to the pending book until tick T + delay.
type Latency struct {
	// BaseOrderDelayTicks is the minimum number of bars every order waits.
	BaseOrderDelayTicks uint64 `json:"baseOrderDelayTicks"`
	// OrderDelayJitterTicks adds a random 0..N-1 bars on top of the base delay.
	// Set to 0 to disable jitter (every order waits exactly the base delay).
	OrderDelayJitterTicks uint64 `json:"orderDelayJitterTicks"`
	// RNGSeed seeds the jitter RNG. A fixed seed makes a run reproducible.
	RNGSeed int64 `json:"rngSeed"`
}

// Engine configures the main loop's timing.
type Engine struct {
	// RenderThrottleMs pauses the loop after each tick so the terminal output
	// is watchable. Set to 0 to run flat out (recommended for real backtests).
	RenderThrottleMs int64 `json:"renderThrottleMs"`
	// IdlePollMs is how long the loop sleeps when the heap is empty (waiting on
	// the readers) before checking again.
	IdlePollMs int64 `json:"idlePollMs"`
}

// Data configures the market universe and file locations.
type Data struct {
	// Tickers is the set of symbols the engine trades.
	Tickers []string `json:"tickers"`
	// BasePath is the root directory for input data files.
	BasePath string `json:"basePath"`
	// QuoteFilePattern is the per-symbol quote file path, relative to BasePath.
	// It must contain the "{ticker}" placeholder, which is replaced with each
	// symbol.
	QuoteFilePattern string `json:"quoteFilePattern"`
	// MetricsOutputPath is where run metrics are written.
	MetricsOutputPath string `json:"metricsOutputPath"`
	// MetricsAppend, when true, appends to MetricsOutputPath across runs;
	// when false, the file is truncated at the start of each run.
	MetricsAppend bool `json:"metricsAppend"`
}

// Config is the full set of engine settings.
type Config struct {
	Latency Latency `json:"latency"`
	Engine  Engine  `json:"engine"`
	Data    Data    `json:"data"`
}

// Default returns the settings matching the engine's original hardcoded values.
func Default() Config {
	return Config{
		Latency: Latency{
			BaseOrderDelayTicks:   2,
			OrderDelayJitterTicks: 3,
			RNGSeed:               1,
		},
		Engine: Engine{
			RenderThrottleMs: 100,
			IdlePollMs:       1,
		},
		Data: Data{
			Tickers:           []string{"AAPL"},
			BasePath:          "./_data/firstrate",
			QuoteFilePattern:  "stock_update_month_1min_quote/" + tickerPlaceholder + "_month_1min_quote.txt",
			MetricsOutputPath: "./_output/metrics.txt",
			MetricsAppend:     true,
		},
	}
}

// Load reads settings from the JSON file at path, layered over Default. A
// non-existent file is not an error: the defaults are returned unchanged. A
// file that exists but is malformed or fails validation returns an error.
func Load(path string) (Config, error) {
	cfg := Default()

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return cfg, fmt.Errorf("invalid config %s: %w", path, err)
	}

	return cfg, nil
}

func (c Config) validate() error {
	if len(c.Data.Tickers) == 0 {
		return fmt.Errorf("data.tickers must list at least one symbol")
	}
	if !strings.Contains(c.Data.QuoteFilePattern, tickerPlaceholder) {
		return fmt.Errorf("data.quoteFilePattern must contain the %s placeholder", tickerPlaceholder)
	}
	if c.Data.MetricsOutputPath == "" {
		return fmt.Errorf("data.metricsOutputPath must not be empty")
	}
	if c.Engine.RenderThrottleMs < 0 {
		return fmt.Errorf("engine.renderThrottleMs must be >= 0")
	}
	if c.Engine.IdlePollMs < 0 {
		return fmt.Errorf("engine.idlePollMs must be >= 0")
	}
	return nil
}

// QuotePath returns the quote data file path for a given ticker.
func (c Config) QuotePath(ticker string) string {
	rel := strings.ReplaceAll(c.Data.QuoteFilePattern, tickerPlaceholder, ticker)
	return c.Data.BasePath + "/" + rel
}

// RenderThrottle is the per-tick display pause as a time.Duration.
func (c Config) RenderThrottle() time.Duration {
	return time.Duration(c.Engine.RenderThrottleMs) * time.Millisecond
}

// IdlePoll is the empty-heap poll interval as a time.Duration.
func (c Config) IdlePoll() time.Duration {
	return time.Duration(c.Engine.IdlePollMs) * time.Millisecond
}
