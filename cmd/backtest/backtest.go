package main

import (
	"container/list"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/HashMaster02/slipstream/src/config"
	"github.com/HashMaster02/slipstream/src/core"
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/metrics"
	"github.com/HashMaster02/slipstream/src/strategy"
	"github.com/HashMaster02/slipstream/src/types"
)

type EngineState struct {
	mu      sync.Mutex
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

// Consider implementing a custom Doubly Linked List typed to an Order
var pendingOrders *list.List = list.New()

func ProcessOrders(port *core.Portfolio) {
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
						fmt.Print(fmt.Errorf("%s", err))
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
						fmt.Print(fmt.Errorf("%s", err))
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
						fmt.Print(fmt.Errorf("%s", err))
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
						fmt.Print(fmt.Errorf("%s", err))
						continue
					}
					port.UpdatePosition(&fill)
					pendingOrders.Remove(e)
				}
			}
		}
	}
}

func ProcessChannels(c <-chan data.Quote, engine *EngineState) {
	for c != nil {
		row, ok := <-c
		if !ok {
			c = nil
			continue
		}

		engine.mu.Lock()
		engine.rowHeap.PushEntry(&row)
		engine.mu.Unlock()
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
		fmt.Println(fmt.Errorf("%w", err))
		os.Exit(1)
	}

	baseOrderDelayTicks = cfg.Latency.BaseOrderDelayTicks
	orderDelayJitterTicks = cfg.Latency.OrderDelayJitterTicks
	delayRNG = rand.New(rand.NewSource(cfg.Latency.RNGSeed))

	tickers := cfg.Data.Tickers

	marketPacketChannel := make(chan data.Quote)

	entryPrices := make(map[string]types.Price)
	for _, symbol := range tickers {
		entryPrices[symbol] = 0
	}

	go ProcessChannels(marketPacketChannel, &engineState)

	READER_COUNT := 0
	for _, ticker := range tickers {
		var QUOTE_DATA_PATH = cfg.QuotePath(ticker)
		reader, err := data.NewReader(QUOTE_DATA_PATH, ticker)
		if err != nil {
			fmt.Print(fmt.Errorf("%s", err))
			continue
		}
		READER_COUNT++

		go data.ReadData(reader, marketPacketChannel)
	}
	if READER_COUNT == 0 {
		fmt.Println("All data files failed to open. Quitting program.")
		os.Exit(-1)
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
		fmt.Print(fmt.Errorf("%s", err))
	}
	defer METRIC_OUTPUT_FILE.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	portfolio := &core.Portfolio{
		Positions: make(map[string]*core.Position),
	}
	strategy := strategy.NewTakeProfit(tickers, types.PriceFromFloat(0.50), 100)

	for {
		// Break out of the loop when Ctrl+C (SIGINT) cancels the context.
		// The default case keeps this non-blocking so the loop keeps
		// draining events when no signal has arrived.
		select {
		case <-ctx.Done():
			fmt.Println("\nShutting down engine...")
			return
		default:
		}

		// Get latest timestamp (tick)
		engineState.mu.Lock()
		var head *data.Quote = engineState.rowHeap.Peek()
		engineState.mu.Unlock()
		if head == nil {
			// Heap is empty (readers haven't produced data yet, or we've
			// drained everything). Yield and retry rather than busy-spinning.
			time.Sleep(cfg.IdlePoll())
			continue
		}

		var currentTick time.Time = head.Timestamp
		tickCount++

		// The engine updates all bars for securities
		for {

			engineState.mu.Lock()
			next := engineState.rowHeap.Peek()
			if next == nil || !next.Timestamp.Equal(currentTick) {
				engineState.mu.Unlock()
				break
			}

			var nextBar *data.Quote = engineState.rowHeap.PopEntry()
			engineState.mu.Unlock()
			latestQuote[nextBar.Symbol] = nextBar

			// Update latest market prices for portfolio
			// ! Edge Case: What if our portfolio has positions for securities we are not currently processing?
			portfolio.UpdatePrice(*nextBar)
		}
		ReleaseDelayedOrders()

		ProcessOrders(portfolio)

		// Calculate signals from strategies
		// TODO: run in go routine for each active strategy ==========
		engineState.mu.Lock()
		newOrders := strategy.CalculateSignals(&latestQuote, portfolio)
		engineState.mu.Unlock()
		// TODO: =====================================================

		// TODO: Have CalculateSignals append to this directly somehow
		for _, o := range newOrders {
			SubmitOrder(o)
		}

		fmt.Printf("\033[2J\033[3J\033[H=====Info=====\n\x1b[1;33mNAV\x1b[0m:: $%s\n\n=====Positions=====\n%s", portfolio.NAV, metrics.CurrentPositions(portfolio))

		time.Sleep(cfg.RenderThrottle()) // so we can watch the numbers on the terminal
	}
}
