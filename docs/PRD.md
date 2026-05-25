# Slipstream — Product Requirements Document

> An event-driven backtesting engine with realistic order book microstructure, written in Go.

**Status:** Draft v1
**Owner:** [you]
**Target completion:** 4 weeks from start (plus ~1 weekend of Go onboarding)

---

## 1. Background

### 1.1 What this is

Slipstream is an event-driven backtesting and simulation engine for single-asset equity strategies. It consumes a historical market data feed, simulates an exchange (limit order book, matching engine, latency model), runs a user-supplied strategy against the simulated venue, and produces performance metrics plus per-trade attribution.

### 1.2 Why it exists

This is a **portfolio project for a Software Engineer interview at a quantitative trading firm**. The job description emphasizes:

- High-performance backtesting and simulation infrastructure
- Workflows for rapid experimentation
- Realism and correctness in simulations — explicitly: market microstructure, latency, data quality
- Tools to analyze performance and explain behavior
- The full lifecycle: idea → backtest → analysis → production

The project is designed to hit every one of these bullets directly, in a way that survives a 45-minute deep-dive in an interview.

The secondary goal is to learn Go in a context that actually exercises its strengths (event loops, goroutine pools, deterministic concurrency), not in toy exercises.

### 1.3 Who reads this

| Audience | What they care about |
| --- | --- |
| You (builder) | Knowing what is in scope and what is not |
| Future you (interviewer) | Being able to articulate goals, tradeoffs, what was cut and why |
| The hiring manager | Whether the project demonstrates judgment, not just code |

---

## 2. Goals and non-goals

### 2.1 Goals

**G1.** Build a backtesting engine that models order book microstructure realistically enough that fill rates and slippage estimates are defensible.

**G2.** Model order submission latency and market data latency as first-class, configurable parameters.

**G3.** Support deterministic replay: same input + same seed + same code = byte-identical output.

**G4.** Support parameter sweeps that run N strategy variants concurrently across CPU cores.

**G5.** Reproduce one published or otherwise verifiable result within a stated tolerance, to validate correctness end-to-end.

**G6.** Produce a written technical post-mortem (~2,000 words) that explains the design, the tradeoffs, and the lessons learned.

**G7.** Demonstrate idiomatic Go: package layout, interfaces, error handling, concurrency primitives, the standard testing package.

### 2.2 Non-goals (explicitly cut)

These are tempting and will eat your timeline. They are deferred indefinitely.

- **Live trading.** No broker integrations, no exchange connections, no paper trading.
- **Multi-asset support.** Single equity at a time. No portfolios, no options, no futures, no FX.
- **Multi-venue routing.** One simulated venue. No SOR, no dark pools.
- **A web UI.** Output goes to Parquet and a static HTML report. That is enough.
- **A strategy DSL or scripting language.** Strategies are Go code implementing an interface. No YAML/JSON/Python config.
- **Production observability.** No Prometheus, no OpenTelemetry, no structured log aggregation. `log/slog` to stdout is enough.
- **Distributed execution.** Single-machine only. The sweep is goroutine-parallel, not multi-host.
- **Real-time data ingestion.** Historical data only, loaded once at start.
- **A general-purpose data pipeline.** Slipstream is a *simulator*, not a *data platform*. Crucible/Meridian already cover the latter.

---

## 3. Success criteria

A reviewer (you, four weeks from now, plus the eventual interviewer) judges the project against these:

| # | Criterion | How it's measured |
| --- | --- | --- |
| S1 | Order book matching engine handles a defined set of scenarios correctly | Passes all unit-test scenarios in `orderbook_test.go` (>30 cases) |
| S2 | Engine models FIFO queue position for resting orders | Resting orders fill in submission order at a price level; documented in a focused test |
| S3 | Latency model is configurable and affects fill behavior measurably | Same strategy under zero-latency vs. realistic-latency produces different and explainable fill rates |
| S4 | Determinism: identical runs produce identical outputs | Hash of output Parquet file is stable across 10 runs of the same config |
| S5 | Sweep harness runs N variants across goroutines and reports completion | Demonstrably saturates available CPU on a sweep of ≥100 variants |
| S6 | Reproduces one known result | A documented strategy reproduces a target metric (e.g., Sharpe within 0.3) of a published or independently-computed reference |
| S7 | A written post-mortem exists and is published | Blog post or repo `WRITEUP.md`, ~2,000 words, covers architecture, three explicit tradeoffs, one failure mode and how it was found |
| S8 | Idiomatic Go | Passes `go vet`, `staticcheck`, `golangci-lint` clean. No `interface{}` in hot paths. Errors wrapped with context. |

If S1–S4 are met but S5–S7 are not, the project is half-finished. **Do not start S5 until S1–S4 are solid.**

---

## 4. Users and use cases

There is one user: a strategy researcher (in this case, you).

### 4.1 Primary user stories

**U1.** As a researcher, I want to run a single strategy over a historical date range and see the equity curve, drawdown, and trade list, so I can judge if the idea has merit.

**U2.** As a researcher, I want to sweep one or more strategy parameters across a grid and rank the results by Sharpe ratio, so I can find sensible parameter regions without manual reruns.

**U3.** As a researcher, I want to compare the same strategy under different latency assumptions, so I can understand how sensitive my idea is to real-world execution friction.

**U4.** As a researcher, I want to re-run a backtest from a saved config and get exactly the same numbers, so I can investigate a result months later and trust it.

**U5.** As a researcher, I want to inspect *why* a specific trade had bad slippage, so I can understand whether the model or the strategy is at fault.

### 4.2 Out of scope user stories (cut, with reasons)

- Hot-reloading strategy code (use `go run`, this is a Go project)
- Walking through a backtest tick-by-tick in a debugger UI (use logs)
- Collaborative result sharing (it's a portfolio project, not a team tool)

---

## 5. Functional requirements

### 5.1 Market data input

**F1.** Accept historical market data in Parquet format with a documented schema (timestamp, symbol, bid, ask, bid_size, ask_size, last, last_size). Trade ticks and quote ticks are differentiated by an `event_type` column.

**F2.** Replay events in strict timestamp order. Ties are broken by event_type (quote before trade) then by sequence number.

**F3.** Tolerate small data gaps (define small) by logging a warning. Tolerate nothing larger; abort with a clear error.

### 5.2 Order book and matching engine

**F4.** Implement a price-time priority limit order book with arbitrary number of price levels.

**F5.** Support order types: `LIMIT`, `MARKET`. Time-in-force: `DAY`, `IOC` (immediate-or-cancel), `GTC`. Side: `BUY`, `SELL`.

**F6.** Model FIFO queue position at each price level. When a resting order rests behind shares already at a price level, those shares must consume incoming market volume first.

**F7.** Support partial fills. Track remaining quantity on each resting order.

**F8.** Support cancellations. Cancels are subject to latency (see F11).

### 5.3 Strategy interface

**F9.** Expose a `Strategy` interface with at minimum:
- `OnStart(ctx)` — one-time setup
- `OnQuote(quote)` — top-of-book update
- `OnTrade(trade)` — a trade printed on the tape
- `OnFill(fill)` — one of your own orders filled
- `OnEnd(ctx)` — teardown / final analytics

**F10.** Strategies submit orders by calling a `Broker` interface. The broker enforces:
- Position limits (configurable max long/short)
- Order-rate limits (configurable max orders/sec — to catch runaway strategies)

### 5.4 Latency model

**F11.** A `LatencyModel` interface with at least:
- `ConstantLatency(d time.Duration)` — flat delay
- `NormalLatency(mean, stddev time.Duration)` — Gaussian
- `LognormalLatency(mu, sigma float64)` — heavy-tailed, more realistic

**F12.** Latency applies separately to:
- Order submission (strategy decides at T, exchange sees it at T + Δ₁)
- Order acknowledgment (fill at T₂, strategy sees it at T₂ + Δ₂)
- Market data (event happens at T, strategy sees it at T + Δ₃)

### 5.5 Output and metrics

**F13.** Per-run output (Parquet):
- `trades.parquet` — every fill with timestamp, side, qty, price, slippage vs. arrival mid
- `equity.parquet` — equity curve sampled at end-of-bar
- `orders.parquet` — every order submitted with its outcome (filled / cancelled / expired)

**F14.** Computed metrics (in JSON sidecar):
- Total return, annualized return
- Sharpe ratio (assume risk-free rate = 0; document)
- Sortino ratio
- Max drawdown (and date)
- Win rate, average win, average loss, profit factor
- Turnover (volume traded / starting equity)
- Average slippage in basis points

**F15.** A static HTML report (`report.html`) generated per run with the equity curve, drawdown plot, and trade markers.

### 5.6 Sweep harness

**F16.** Accept a sweep config (YAML or JSON) describing a parameter grid.

**F17.** Run all variants concurrently using a goroutine worker pool (size = `runtime.NumCPU()` by default).

**F18.** Output a `sweep_results.parquet` with one row per variant: parameter values + key metrics. Sortable / queryable downstream.

### 5.7 Reproducibility

**F19.** All randomness is seeded. A `seed` is part of every run config. Two runs with the same config and same data produce byte-identical output.

**F20.** The exact version of the binary (git SHA, embedded at build time) is written into every run's output directory.

---

## 6. Non-functional requirements

### 6.1 Performance

**N1.** A single backtest over 1 year of 1-second quote data (~6M events) for one symbol should complete in **under 30 seconds** on a modern laptop. (Stretch: under 10 seconds.)

**N2.** The sweep harness should process N independent backtests in approximately N / cores wall time, modulo startup overhead.

### 6.2 Correctness

**N3.** No floating-point math for prices, quantities, or PnL. Use a fixed-point type (cents or a `decimal.Decimal` equivalent — you may pick an existing Go library here, e.g., `shopspring/decimal`, or build a thin integer-based "Price" type).

**N4.** All tests pass under `-race`. The matching engine, in particular, must be single-threaded by design — but the harness running it must be race-clean.

### 6.3 Code quality

**N5.** Clean output from `go vet`, `staticcheck`, and `golangci-lint` with a reasonable config. Document any disabled rules in `.golangci.yml` with a comment.

**N6.** Unit test coverage ≥ 70% on `internal/orderbook`, `internal/matching`, `internal/metrics`. Other packages: best-effort.

### 6.4 Documentation

**N7.** Every exported type and function in `pkg/` has a doc comment. `go doc ./...` produces useful output.

**N8.** A top-level `README.md` explains how to run, what assumptions are made, and links to the writeup.

---

## 7. Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
| --- | --- | --- | --- |
| Go learning curve eats week 1 | Medium | High | Pre-week onboarding before week 1 starts |
| Matching engine has a subtle correctness bug | High | High | Heavy unit testing in week 2; never move on until tests are exhaustive |
| Realistic data is hard to get cheaply | Medium | Medium | Start with daily bars or 1-minute bars (free from Yahoo / Stooq); upgrade to tick data only if time allows |
| Scope creep — adding options or multi-asset | High | Critical | Re-read this PRD weekly; cut, don't add |
| Reproducibility breaks with goroutine ordering | Medium | High | Each backtest is single-threaded; only the sweep harness is parallel |
| Write-up gets cut at the end | High | Critical | Week 4 is *explicitly* for write-up, not for new features |

---

## 8. Open questions

These should be resolved by the end of pre-week:

1. What is the minimum-viable historical dataset? (Free 1-minute bars for one liquid symbol over a year is the recommended floor.)
2. Decimal library or hand-rolled fixed-point? (Default recommendation: hand-rolled integer "Price" type as cents or as 1/10000 of a dollar — it's simpler, faster, and a good learning exercise. Use `shopspring/decimal` only if you decide the hand-roll is taking real time.)
3. Charting library for the HTML report? (Default: dump to JSON, embed in a static template that uses Plotly via CDN. No Go-side plotting library.)

---

## 9. Appendix: what an "interview-ready" version looks like

When you're done, you should be able to answer these questions in 60 seconds each, in an interview, without slides:

1. *"Walk me through what happens when a strategy submits a limit order."*
2. *"How do you model queue position, and why does it matter?"*
3. *"How do you guarantee deterministic replay when the sweep is parallel?"*
4. *"What did you assume that turned out to be wrong, and how did you fix it?"*
5. *"What would you build next if you had another month?"*

If any of those questions makes you sweat, that part of the project isn't done — regardless of what the code looks like.
