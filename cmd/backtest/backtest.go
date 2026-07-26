package main

import (
	"container/list"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/HashMaster02/slipstream/src/config"
	"github.com/HashMaster02/slipstream/src/core"
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/metrics"
	"github.com/HashMaster02/slipstream/src/strategy"
	"github.com/HashMaster02/slipstream/src/types"
)

type EngineState struct {
	rowHeap core.QuoteHeap
}

var engineState EngineState = EngineState{
	rowHeap: core.QuoteHeap{},
}

// Simulated Network Delay parameters, measured in ticks (bars) rather than
// wall-clock time. An order emitted by a strategy on tick T is not added to
// the `pendingOrders` until tick (T + delay)
var (
	baseOrderDelayTicks   uint64 // each order waits at least X bars
	orderDelayJitterTicks uint64 // add a random 0..X-1 bars on top of the base
)
var delayRNG *rand.Rand

// Monotonic count of distinct market ticks (bars) processed by the engine
var tickCount uint64

type delayedOrder struct {
	order          types.Order
	activationTick uint64
}
var delayedOrders []delayedOrder

// TODO: Move this somewhere else at some point
var latestQuote map[string]*data.Quote = make(map[string]*data.Quote)

// Orders do not fill against a quote older than this. Positions stay marked at it.
var maxQuoteStaleness time.Duration
var staleSymbols map[string]bool = make(map[string]bool)

// Consider implementing a custom Doubly Linked List typed to an Order
var pendingOrders *list.List = list.New()

func ProcessOrders(port *core.Portfolio, now time.Time) {
	// Keep track of 'next' node in case of current Order deletion due to Fill
	var next *list.Element

	for e := pendingOrders.Front(); e != nil; e = next {
		next = e.Next()

		order, ok := e.Value.(*types.Order)
		if !ok {
			continue
		}

		bar, succ := latestQuote[order.Symbol]
		if !succ {
			continue
		}

		// A stale quote is not a market we can trade against, so the order rests
		// until fresh data arrives for that symbol.
		if maxQuoteStaleness > 0 && now.Sub(bar.Timestamp) > maxQuoteStaleness {
			staleSymbols[order.Symbol] = true
			continue
		}

		// TODO: This switch statement duplicates a lot of logic. Simplify it.
		switch order.Type {
		case types.Market:
			{
				if (order.Side == types.Buy) {
					fill, err := types.NewFill(
						order.Symbol,
						order.Side,
						order.Type,
						order.Quantity,
						bar.Ask,
					)
					if err != nil {
						fmt.Fprintf(os.Stderr, "%v\n", err)
						continue
					}
					port.UpdatePosition(&fill)
					pendingOrders.Remove(e)
				} else if (order.Side == types.Sell) {  // being explicit here on purpose
					fill, err := types.NewFill(
						order.Symbol,
						order.Side,
						order.Type,
						order.Quantity,
						bar.Bid,
					)
					if err != nil {
						fmt.Fprintf(os.Stderr, "%v\n", err)
						continue
					}
					port.UpdatePosition(&fill)
					pendingOrders.Remove(e)
				}
			}
		case types.Limit:
			{
				if ((order.Side == types.Buy) && (bar.Ask <= order.Price)) {
					fill, err := types.NewFill(
						order.Symbol,
						order.Side,
						order.Type,
						order.Quantity,
						bar.Ask,
					)
					if err != nil {
						fmt.Fprintf(os.Stderr, "%v\n", err)
						continue
					}
					port.UpdatePosition(&fill)
					pendingOrders.Remove(e)
				} else if ((order.Side == types.Sell) && (bar.Bid >= order.Price)) {
					fill, err := types.NewFill(
						order.Symbol,
						order.Side,
						order.Type,
						order.Quantity,
						bar.Bid,
					)
					if err != nil {
						fmt.Fprintf(os.Stderr, "%v\n", err)
						continue
					}
					port.UpdatePosition(&fill)
					pendingOrders.Remove(e)
				}
			}
		}
	}
}

func SubmitOrder(order types.Order) {
	delay := baseOrderDelayTicks
	if orderDelayJitterTicks > 0 {
		delay += uint64(delayRNG.Int63n(int64(orderDelayJitterTicks)))
	}
	delayedOrders = append(delayedOrders, delayedOrder{
		order:          order,
		activationTick: tickCount + delay,
	})
}

func ReleaseDelayedOrders() {
	remaining := delayedOrders[:0]
	for _, d := range delayedOrders {
		if d.activationTick > tickCount {
			remaining = append(remaining, d)
			continue
		}
		o := d.order
		pendingOrders.PushBack(&o)
	}
	delayedOrders = remaining
}

func main() {

	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	baseOrderDelayTicks = cfg.Latency.BaseOrderDelayTicks
	orderDelayJitterTicks = cfg.Latency.OrderDelayJitterTicks
	delayRNG = rand.New(rand.NewSource(cfg.Latency.RNGSeed))
	maxQuoteStaleness = cfg.MaxQuoteStaleness()
	renderThrottle := cfg.RenderThrottle()

	tickers := cfg.Data.Tickers

	QUOTE_COUNT := 0
	for _, ticker := range tickers {
		var QUOTE_DATA_PATH = cfg.QuotePath(ticker)
		reader, err := data.NewReader(QUOTE_DATA_PATH, ticker)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}

		quotes, err := data.LoadAll(reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\nBad market data. Quitting program.\n", ticker, err)
			os.Exit(1)
		}
		QUOTE_COUNT += len(quotes)

		for i := range quotes {
			engineState.rowHeap.PushEntry(&quotes[i])
		}
	}
	if QUOTE_COUNT == 0 {
		fmt.Fprintln(os.Stderr, "No market data was loaded. Quitting program.")
		os.Exit(1)
	}

	// =========== Main engine loop ===============
	metricFlags := os.O_WRONLY | os.O_CREATE
	if cfg.Data.MetricsAppend {
		metricFlags |= os.O_APPEND
	} else {
		metricFlags |= os.O_TRUNC
	}
	METRIC_OUTPUT_FILE, err := os.OpenFile(cfg.Data.MetricsOutputPath, metricFlags, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	} else {
		defer METRIC_OUTPUT_FILE.Close()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	startingCash := types.PriceFromFloat(cfg.Portfolio.StartingCash)
	portfolio := &core.Portfolio{
		Positions: make(map[string]*core.Position),
		Cash:      startingCash,
		NAV:       startingCash,
	}
	strategy := strategy.NewTakeProfit(tickers, types.PriceFromFloat(0.50), 100)

engineLoop:
	for {
		// Break out of the loop when Ctrl+C (SIGINT) cancels the context.
		// The default case keeps this non-blocking so the loop keeps
		// draining events when no signal has arrived.
		select {
		case <-ctx.Done():
			fmt.Println("\nShutting down engine...")
			break engineLoop
		default:
		}

		// Get latest timestamp (tick)
		var head *data.Quote = engineState.rowHeap.Peek()
		if head == nil {
			break
		}

		var currentTick time.Time = head.Timestamp
		tickCount++

		// The engine updates all bars for securities
		for {

			next := engineState.rowHeap.Peek()
			if next == nil || !next.Timestamp.Equal(currentTick) {
				break
			}

			var nextBar *data.Quote = engineState.rowHeap.PopEntry()
			latestQuote[nextBar.Symbol] = nextBar

			// Update latest market prices for portfolio
			// ! Edge Case: What if our portfolio has positions for securities we are not currently processing?
			portfolio.UpdatePrice(*nextBar)
		}
		ReleaseDelayedOrders()

		ProcessOrders(portfolio, currentTick)

		// Calculate signals from strategies
		// TODO: run in go routine for each active strategy ==========
		newOrders := strategy.CalculateSignals(&latestQuote, portfolio)
		// TODO: =====================================================

		// TODO: Have CalculateSignals append to this directly somehow
		for _, o := range newOrders {
			SubmitOrder(o)
		}

		// The live view clears the screen every tick, so it is only drawn when the
		// throttle is slow enough to watch. Otherwise errors would be wiped too.
		if renderThrottle > 0 {
			fmt.Printf("\033[2J\033[3J\033[H=====Info=====\n\x1b[1;33mNAV\x1b[0m:: $%s\n\x1b[1;33mCash\x1b[0m:: $%s\n\n=====Positions=====\n%s", portfolio.NAV, portfolio.Cash, metrics.CurrentPositions(portfolio))
			time.Sleep(renderThrottle)
		}
	}

	totalReturn := 0.0
	if startingCash > 0 {
		totalReturn = (portfolio.NAV.Float()/startingCash.Float() - 1) * 100
	}
	var summary strings.Builder
	fmt.Fprintf(&summary, "\n=====Backtest Complete=====\nTickers: %s\nTicks processed: %d\nStarting Cash: $%s\nFinal NAV: $%s\nTotal Return: %0.2f%%\n", strings.Join(tickers, ", "), tickCount, startingCash, portfolio.NAV, totalReturn)

	if len(staleSymbols) > 0 {
		stale := make([]string, 0, len(staleSymbols))
		for symbol := range staleSymbols {
			stale = append(stale, symbol)
		}
		sort.Strings(stale)
		fmt.Fprintf(&summary, "Stale data (orders held, positions marked at last price): %s\n", strings.Join(stale, ", "))
	}

	fmt.Print(summary.String())

	if METRIC_OUTPUT_FILE != nil {
		if _, err := METRIC_OUTPUT_FILE.WriteString(summary.String()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}
}
