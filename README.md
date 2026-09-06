# Slipstream

> **Not for professional use.** Slipstream was built as a study project to learn about basic stock market
> functionality and to learn Go. It is not audited, not battle-tested, and its fill model is a deliberate
> simplification of how real markets work. Do not use it to make trading decisions or to manage real money.

A backtesting engine for equity strategies, written in Go. It replays historical 1-minute
bid/ask/last quotes through a simulated venue with order latency, cash constraints, and stale-data handling,
then reports what the portfolio did.

## AI Notice

The production code was **not** written with AI. However, AI was used to write the tests and to help find some bugs.

## Known simplifications

There are gaps between this and a real simulator, so it's worth being explicit:

- No order book or depth. Orders match against a single top-of-book quote and always fill in full — no partial
  fills, no queue position, no market impact.
- Bid/ask are _estimated_ from OHLC candles, not observed. Using the bar's high as the ask and low as the bid
  overstates the spread badly.
- Time-in-force is stored on the order but not enforced; nothing expires.
- No shorting, margin, fees, or slippage beyond the spread.
- No corporate action handling beyond whatever the source data already adjusted for.

## Running it

```sh
go run ./cmd/backtest
```

Data files are not in the repo. The engine expects one quote file per ticker at
`{basePath}/{quoteFilePattern}`, with `{ticker}` substituted in, and each line formatted as:

```
2026-04-27 04:00:00, 269.212800, 270.300000, 269.740000
```

that is: `timestamp, bid, ask, last`. If you only have OHLC candles
(`timestamp,open,high,low,close,volume` — the FirstRate Data format), `cmd/genbidask` will synthesize quote
files from them; the ticker and paths are constants at the top of its `main`.

## Configuration

Settings live in [config.json](config.json), layered over the defaults in
[src/config/config.go](src/config/config.go). A missing file is fine — the defaults run. A malformed one is an
error.

```json
{
  "latency": {
    "baseOrderDelayTicks": 2,
    "orderDelayJitterTicks": 3,
    "rngSeed": 1
  },
  "engine": {
    "renderThrottleMs": 0,
    "idlePollMs": 1
  },
  "data": {
    "tickers": ["AAPL"],
    "basePath": "./_data/firstrate",
    "quoteFilePattern": "stock_update_month_1min_quote/{ticker}_month_1min_quote.txt",
    "metricsOutputPath": "./_output/metrics.txt",
    "metricsAppend": true,
    "maxQuoteStalenessMinutes": 60
  },
  "portfolio": {
    "startingCash": 100000
  }
}
```

`renderThrottleMs` is the one worth calling out: at `0` (the default) the engine runs flat out and prints only
the final summary. Set it to something watchable (say `100`) to get the live NAV/positions view, which redraws
and pauses every tick.

The run summary is printed to the console and written to `metricsOutputPath`.

## Tests

```sh
go test ./...
```
