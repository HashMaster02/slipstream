# Slipstream — Technical Design

> Companion to `PRD.md`. This doc describes *how* the system is built, with explicit tradeoffs called out.

**Status:** Draft v1
**Read order:** Read the PRD first.

---

## 1. Architecture at a glance

```
                          ┌──────────────────┐
                          │  Config (YAML)   │
                          └────────┬─────────┘
                                   │
                                   ▼
  ┌─────────────────┐   ┌────────────────────────────────────┐   ┌────────────────┐
  │ Market Data     │──▶│           Simulator                │──▶│ Output Writer  │
  │ Source          │   │                                    │   │ (Parquet+JSON) │
  │ (Parquet)       │   │  ┌──────────────────────────────┐  │   └────────────────┘
  └─────────────────┘   │  │   Event Loop                 │  │
                        │  │   ┌──────────────────────┐   │  │
                        │  │   │  Event Queue (heap)  │   │  │
                        │  │   └──────────┬───────────┘   │  │
                        │  │              │               │  │
                        │  │              ▼               │  │
                        │  │   ┌────────────────────┐     │  │
                        │  │   │  Latency Model     │     │  │
                        │  │   └────────────────────┘     │  │
                        │  │              │               │  │
                        │  │   ┌──────────┴───────────┐   │  │
                        │  │   ▼                      ▼   │  │
                        │  │ ┌──────────┐      ┌────────┐ │  │
                        │  │ │ Strategy │◀────▶│ Broker │ │  │
                        │  │ └──────────┘      └───┬────┘ │  │
                        │  │                       │      │  │
                        │  │                       ▼      │  │
                        │  │             ┌────────────────┐│  │
                        │  │             │ Matching Engine││  │
                        │  │             │   + Order Book ││  │
                        │  │             └────────────────┘│  │
                        │  └──────────────────────────────┘  │
                        └────────────────────────────────────┘

                        Sweep harness (separate concern):
                                ┌─────────────────┐
                                │ Sweep Coordinator│
                                └────────┬────────┘
                                         │ fan out
                          ┌──────────────┼──────────────┐
                          ▼              ▼              ▼
                     ┌────────┐    ┌────────┐    ┌────────┐
                     │Worker N│    │Worker N│ …  │Worker N│   one Simulator each
                     └────────┘    └────────┘    └────────┘
```

The simulator is **single-threaded by design**. The sweep harness runs many simulators in parallel.

---

## 2. Package layout

```
slipstream/
├── cmd/
│   ├── backtest/        # single-run CLI entry point
│   └── sweep/           # sweep CLI entry point
├── internal/
│   ├── orderbook/       # limit order book data structure
│   ├── matching/        # matching engine (consumes events, produces fills)
│   ├── simulator/       # the event loop, time, broker
│   ├── latency/         # latency model implementations
│   ├── data/            # parquet reader, schema validation
│   ├── metrics/         # Sharpe, drawdown, attribution
│   ├── output/          # parquet writer, HTML report generator
│   └── sweep/           # sweep coordinator and worker pool
├── pkg/
│   ├── strategy/        # Strategy interface (public — strategies depend on it)
│   ├── types/           # Order, Fill, Quote, Trade, Price (public)
│   └── slipstream/      # high-level facade for embedded use
├── strategies/          # example strategies (each its own subpackage)
├── docs/
│   ├── PRD.md
│   ├── DESIGN.md        # this file
│   └── SCHEDULE.md
├── testdata/            # fixtures
├── go.mod
└── README.md
```

**Why `internal/` vs `pkg/`:** Anything under `internal/` is unimportable from outside the module (Go enforces this). The strategy author only needs the `Strategy` interface, the `types`, and possibly the facade — everything else is implementation detail and lives in `internal/`.

---

## 3. Core types

These are the load-bearing types. Get them right early.

### 3.1 Price

```go
// Price is a fixed-point monetary value in 1/10000ths of a unit (1 pip = 0.0001).
// Stored as int64 to avoid floating-point error.
// MaxPrice ≈ 9.2 × 10^14 / 10000 = 9.2 × 10^10 — far above any reasonable equity price.
type Price int64

const TicksPerUnit Price = 10000

func PriceFromFloat(f float64) Price { return Price(math.Round(f * float64(TicksPerUnit))) }
func (p Price) Float() float64        { return float64(p) / float64(TicksPerUnit) }
func (p Price) String() string        { return fmt.Sprintf("%.4f", p.Float()) }
```

**Tradeoff:** A hand-rolled fixed-point type is faster and forces you to think about precision. The cost is you write a little arithmetic yourself. If multiplication becomes painful (PnL × shares × price ratio), revisit and consider `shopspring/decimal`.

### 3.2 Order, Fill, Quote, Trade

```go
type OrderID uint64

type Side int8
const ( Buy Side = 1; Sell Side = -1 )

type TIF int8
const ( Day TIF = iota; IOC; GTC )

type OrderType int8
const ( Limit OrderType = iota; Market )

type Order struct {
    ID            OrderID
    Symbol        string
    Side          Side
    Type          OrderType
    TIF           TIF
    Quantity      int64    // shares
    Price         Price    // limit price (0 if Market)
    SubmittedAt   time.Time
}

type Fill struct {
    OrderID   OrderID
    FillID    uint64
    Symbol    string
    Side      Side
    Quantity  int64
    Price     Price
    Timestamp time.Time
}

type Quote struct {
    Symbol    string
    Bid       Price
    Ask       Price
    BidSize   int64
    AskSize   int64
    Timestamp time.Time
}

type Trade struct {
    Symbol    string
    Price     Price
    Quantity  int64
    Side      Side  // aggressor side; may be Unknown if data lacks it
    Timestamp time.Time
}
```

All times are `time.Time` in UTC. Never store local time; convert at the I/O boundary if needed.

### 3.3 Event

The event queue holds heterogeneous events keyed on timestamp.

```go
type Event interface {
    EventTime() time.Time
    EventKind() EventKind
}

type EventKind int8
const (
    KindQuote EventKind = iota
    KindTrade
    KindOrderSubmit
    KindOrderCancel
    KindOrderAck
    KindFill
    KindClockTick
)
```

Concrete types embed a base struct. The event queue is a `container/heap` of `Event` ordered by `(EventTime, EventKind, sequenceNumber)` — the tiebreakers are what make ordering deterministic.

---

## 4. The event loop

```go
func (s *Simulator) Run(ctx context.Context) error {
    s.strategy.OnStart(ctx)
    defer s.strategy.OnEnd(ctx)

    for !s.queue.Empty() {
        if err := ctx.Err(); err != nil { return err }

        ev := s.queue.Pop()  // earliest-timestamped event
        s.clock.Advance(ev.EventTime())

        switch e := ev.(type) {
        case QuoteEvent:
            s.book.ApplyQuote(e.Quote)
            // schedule strategy delivery with market data latency
            s.queue.Push(StrategyQuoteDelivery{...})
        case StrategyQuoteDelivery:
            s.strategy.OnQuote(e.Quote)
        case OrderSubmitEvent:
            // submission was scheduled when strategy called the broker;
            // now it lands at the exchange.
            fills := s.engine.Submit(e.Order)
            for _, f := range fills {
                s.queue.Push(FillDeliveryEvent{f, e.EventTime().Add(s.latency.AckLatency())})
            }
        // ... etc
        }
    }
    return nil
}
```

**Key invariant:** The strategy's view of time and the matching engine's view of time can differ by the configured latency. The strategy never sees a quote at time T; it sees the version that was emitted at T - market_data_latency.

**Determinism:** The event queue's ordering uses `(timestamp, kind, sequence)`. The sequence number is a monotonic counter assigned at push time. A given run with a given seed produces an identical sequence of pops.

---

## 5. Order book

### 5.1 Data structure

```go
type Book struct {
    bids *PriceLevels  // sorted descending by price
    asks *PriceLevels  // sorted ascending by price
}

type PriceLevels struct {
    levels map[Price]*Level
    sorted []Price  // maintained sorted; binary search on insert
}

type Level struct {
    Price  Price
    Orders *list.List  // doubly linked list of *RestingOrder (FIFO)
    Volume int64       // sum of remaining qty across orders
}

type RestingOrder struct {
    Order
    Remaining int64
}
```

**Tradeoff:** A sorted slice + map for price levels is O(log N) on insert and O(1) on best-price lookup. The book rarely has more than ~100 active price levels per side in practice, so this is well within budget. A red-black tree would be marginally faster and dramatically more code.

The FIFO queue at each level is a `container/list` (doubly linked) so cancels are O(1) given a pointer to the order. Maintain a `map[OrderID]*list.Element` on the book for O(1) cancel lookup.

### 5.2 Queue position — the load-bearing detail

When a strategy's limit order arrives at a price level that already has resting volume:

1. The order is appended to the tail of the level's order list.
2. Its `QueueAhead` field is set to the current level volume *at submission time*.
3. As incoming aggressor volume hits the level, the order's `QueueAhead` decreases.
4. The order only starts filling when `QueueAhead == 0`.

This is the single feature that makes Slipstream not a toy. In an interview: *"Without modeling queue position, my fill rate on a passive strategy was overstated by 40%, and the apparent Sharpe was 0.6 higher than realistic."*

### 5.3 Matching rules

When an aggressor order arrives (a marketable order — market order, or a limit that crosses the spread):

1. Walk price levels from best to worst.
2. At each level, consume orders FIFO. Partial fills mark remaining qty.
3. Stop when the aggressor is exhausted or no more crossing levels exist.
4. Emit a `Fill` event for each consumption.

When a quote update arrives from the data feed:

This is the conceptually subtle one. **In Slipstream, the order book is driven by the data feed, not by the matching engine.** The book has two kinds of liquidity:

- **External liquidity** — quoted size from the feed. This is what your orders trade against when they cross the spread.
- **Internal liquidity** — your own resting orders. These compete with external liquidity at their price level.

When an external trade prints from the feed (a `KindTrade` event), the simulator must decide whether your resting orders at the touch would have participated. Two simple models:

- **Optimistic:** Your order fills any external volume that hit your level. (Overstates fills.)
- **Pessimistic:** Your order only fills if external volume at your level exceeds the volume ahead of you in the queue. (Default — more realistic.)

Document which model is used in the run config.

---

## 6. Latency model

```go
type LatencyModel interface {
    SubmissionLatency() time.Duration
    AckLatency() time.Duration
    MarketDataLatency() time.Duration
}
```

Implementations:

| Implementation | Notes |
| --- | --- |
| `ZeroLatency` | All zero. For correctness debugging only — never realistic. |
| `ConstantLatency{d}` | Same delay for all three legs. |
| `NormalLatency{mean, stddev}` | Gaussian, floored at zero. |
| `LognormalLatency{mu, sigma}` | Heavy-tailed; matches the "tail wags" of real exchanges. **Default for realistic runs.** |
| `EmpiricalLatency{cdf}` | Samples from an empirical CDF you provide. Stretch goal. |

All randomness goes through a single `*rand.Rand` seeded from the config. The rand source is **not** shared across goroutines; the sweep harness gives each worker its own seeded source derived from a master seed.

---

## 7. Concurrency model

### 7.1 The simulator is single-threaded

Inside one simulation, everything happens on one goroutine. The event loop pops events, dispatches them to handlers, and pushes new events. No channels, no mutexes. This is the *only* way to get deterministic replay cheaply.

### 7.2 The sweep harness is parallel

```go
type Sweep struct {
    Configs []RunConfig
    Workers int
}

func (s *Sweep) Run(ctx context.Context) ([]Result, error) {
    work := make(chan RunConfig)
    results := make(chan Result, len(s.Configs))

    var wg sync.WaitGroup
    for i := 0; i < s.Workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for cfg := range work {
                r, err := runOne(ctx, cfg)
                results <- Result{cfg, r, err}
            }
        }()
    }

    go func() {
        defer close(work)
        for _, c := range s.Configs {
            select {
            case work <- c:
            case <-ctx.Done():
                return
            }
        }
    }()

    wg.Wait()
    close(results)

    out := make([]Result, 0, len(s.Configs))
    for r := range results { out = append(out, r) }
    sort.Slice(out, func(i, j int) bool { return out[i].ConfigIndex < out[j].ConfigIndex })
    return out, nil
}
```

**Determinism in parallel:** Each `runOne` is fully deterministic given its config and seed. The order in which the harness completes variants is non-deterministic, but the final sorted result is identical run-to-run.

**Backpressure:** Unbuffered `work` channel means workers pull as they finish — natural load balancing.

**Cancellation:** `ctx` is honored throughout. Ctrl-C cleanly cancels in-flight runs.

---

## 8. Data formats

### 8.1 Input (market data)

Parquet, one file per symbol per month:

```
data/quotes/{SYMBOL}/{YYYY-MM}.parquet
```

Schema:

| Column | Type | Notes |
| --- | --- | --- |
| timestamp | timestamp(ns, UTC) | Strictly ascending |
| symbol | string | Constant per file |
| event_type | string | "quote" or "trade" |
| bid | decimal(18,4) | NULL for trade |
| ask | decimal(18,4) | NULL for trade |
| bid_size | int64 | NULL for trade |
| ask_size | int64 | NULL for trade |
| trade_price | decimal(18,4) | NULL for quote |
| trade_size | int64 | NULL for quote |
| aggressor | string | "buy", "sell", or "unknown" |
| sequence | int64 | Tiebreaker for same-ns events |

### 8.2 Output

Per-run directory:

```
runs/{run_id}/
├── config.yaml         # the exact config used
├── version.json        # git SHA, build time
├── trades.parquet
├── orders.parquet
├── equity.parquet
├── metrics.json
└── report.html
```

`run_id` is a deterministic hash of `(config, data version, git SHA)` so re-runs with the same inputs reuse the same directory and you can hash-compare outputs.

---

## 9. Testing strategy

### 9.1 Unit tests

The matching engine and order book get the most love. Every scenario should be a separate test function with a descriptive name:

```go
func TestBook_LimitBuy_AddsToBidSide(t *testing.T) { ... }
func TestBook_MarketSell_SweepsTopOfBookFIFO(t *testing.T) { ... }
func TestBook_PartialFill_LeavesRestingOrder(t *testing.T) { ... }
func TestBook_Cancel_RemovesFromQueue_PreservesOthers(t *testing.T) { ... }
func TestBook_QueuePosition_PassiveOrderDoesNotFillUntilAhead(t *testing.T) { ... }
// ... at least 30 scenarios before week 2 ends.
```

### 9.2 Property-based tests

For the matching engine: random sequences of orders should preserve invariants. Use `gopter` or hand-rolled with `testing/quick`.

Invariants:

- Total volume in == total volume out + total resting volume.
- Best bid < best ask at all times (no crossed book).
- Fill prices respect submission limits (no buy above limit, no sell below limit).

### 9.3 Reproduction test

One end-to-end test: load a fixed dataset, run a fixed strategy with a fixed seed, assert specific output metrics within tolerance. This is your regression net — if anything fundamental breaks, this test catches it.

### 9.4 What not to test

- `cmd/` entry points (smoke-test via shell, not Go test)
- Generated HTML output (eyeball it)
- `output/parquet.go` write paths (test the metrics, trust the library)

---

## 10. Explicit tradeoffs you'll be asked about

Memorize these. Each is a 60-second answer you should rehearse.

| Decision | Alternative | Why this way |
| --- | --- | --- |
| Event-driven, not vectorized | Pandas-style vectorized backtester | Vectorized hides microstructure; event-driven matches real trading systems and forces you to model latency and queue position. |
| Single-threaded simulation | Parallel inside one run | Determinism. Parallelism inside one backtest creates ordering ambiguity that destroys reproducibility. Parallelism *across* backtests is fine. |
| Fixed-point `Price` int64 | float64 | float64 has 15-16 significant digits; a price × shares × time accumulator drifts. Fixed point is exact. |
| FIFO queue position | Random fill probability | Real exchanges use price-time priority. Random is "easier to implement" but unrealistic. |
| Pessimistic external-fill model | Optimistic | Optimistic overstates fill rates for passive strategies. The whole point of microstructure realism. |
| Strategies as Go code | YAML/JSON DSL | A DSL is a project unto itself. Go interfaces are perfectly expressive and let strategies import any library they want. |
| Parquet output | CSV / JSON / SQLite | Columnar, typed, fast, plays well with pandas/polars for downstream analysis. |
| `internal/` for engine, `pkg/` for strategy interface | Everything public | Strategies should depend on a stable narrow surface. Implementation should be free to change. |

---

## 11. What's not designed yet (and intentionally so)

- The exact HTML report layout — punt to week 4
- Whether to support multi-strategy runs in one backtest — defer; one strategy per run for now
- Sweep config schema — defer to week 3 when you have a strategy to sweep
- Backfilling the order book at start-of-day (cold start problem) — for now, start the book empty and let the first ~100 events warm it

If any of these surprises you in week 3, write a brief design note in `docs/` rather than letting it derail you.

---

## 12. Reading list (curated, don't go down rabbit holes)

For the interview, you should be able to gesture at having heard of these. You don't need to read all of them.

- **"Trades, Quotes and Prices" by Bouchaud, Bonart, Donier, Gould (2018)** — the standard text on market microstructure. Read chapters 1, 4, and 6 if anything.
- **CME Globex order entry/matching docs** — public PDF, ~30 pages, describes how a real matching engine works. Skim for vocabulary.
- **"The Bias of Time-Series Backtests" — short blog posts by Marcos López de Prado** — particularly the "deflated Sharpe" essay. Useful caveat to keep in your back pocket.
- **Go's `container/heap` and `container/list` docs** — read once, you'll use both heavily.
- **Effective Go + the language spec on `select` and channels** — to handle the sweep harness correctly.

Don't read more than this. The point is to ship, not to study.
