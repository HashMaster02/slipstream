package main

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/HashMaster02/slipstream/internal/core"
	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/metrics"
	"github.com/HashMaster02/slipstream/pkg/strategy"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type EngineState struct {
	mu      sync.Mutex
	rowHeap core.CandleHeap
}

var engineState EngineState = EngineState{
	rowHeap: core.CandleHeap{},
}

// TODO: Move this somewhere else at some point
var latestBar map[string]*data.Candle = make(map[string]*data.Candle)

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

		bar, succ := latestBar[order.Symbol]
		if !succ {
			continue
		}

		switch order.Type {
		case types.Market:
			{
				fill, err := types.NewFill(
					order.Symbol,
					order.Side,
					order.Type,
					order.Quantity,
					bar.Close,
				)
				if err != nil {
					fmt.Print(fmt.Errorf("%s", err))
					continue
				}
				port.UpdatePosition(&fill)
				pendingOrders.Remove(e)
			}
		case types.Limit:
			{
				if ((order.Side == types.Buy) && (bar.Close <= order.Price)) || ((order.Side == types.Sell) && (bar.Close >= order.Price)) {
					fill, err := types.NewFill(
						order.Symbol,
						order.Side,
						order.Type,
						order.Quantity,
						bar.Close,
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

func ProcessChannels(c <-chan data.Candle, engine *EngineState) {
	// Both channels are unbuffered, which guarantees that we
	// push data onto the rowHeap BEFORE we push a corresponding
	// MarketEvent onto the eventQueue (which we want). If we ever
	// buffer either of the channels, we will break this inherent sequence.
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

func main() {

	tickers := []string{"AAPL", "GS", "MSFT", "NVDA", "META", "GOOG", "T", "JPM"}

	marketPacketChannel := make(chan data.Candle)

	entryPrices := make(map[string]types.Price)
	for _, symbol := range tickers {
		entryPrices[symbol] = 0
	}

	go ProcessChannels(marketPacketChannel, &engineState)

	for _, ticker := range tickers {
		filepath := fmt.Sprintf("./_data/firstrate/stock_update_month_1min_adjsplit/%s_month_1min_adjsplit.txt", ticker)
		reader, err := data.NewReader(filepath, ticker)
		if err != nil {
			fmt.Print(fmt.Errorf("%s", err))
			os.Exit(-1)
		}

		go data.ReadData(reader, marketPacketChannel)
	}

	// =========== Main engine loop ===============
	METRIC_OUTPUT_FILE, err := os.OpenFile("./_output/metrics.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
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
		var head *data.Candle = engineState.rowHeap.Peek()
		engineState.mu.Unlock()
		if head == nil {
			// Heap is empty (readers haven't produced data yet, or we've
			// drained everything). Yield and retry rather than busy-spinning.
			time.Sleep(time.Millisecond)
			continue
		}

		var currentTick time.Time = head.Timestamp

		// The engine updates all bars for securities
		for {

			engineState.mu.Lock()
			next := engineState.rowHeap.Peek()
			if next == nil || !next.Timestamp.Equal(currentTick) {
				engineState.mu.Unlock()
				break
			}

			var nextBar *data.Candle = engineState.rowHeap.PopEntry()
			engineState.mu.Unlock()
			latestBar[nextBar.Symbol] = nextBar

			// Update latest market prices for portfolio
			// ! Edge Case: What if our portfolio has positions for securities we are not currently processing?
			portfolio.UpdatePrice(*nextBar)
		}

		ProcessOrders(portfolio)

		// Calculate signals from strategies
		// TODO: run in go routine for each active strategy ==========
		engineState.mu.Lock()
		newOrders := strategy.CalculateSignals(&latestBar, portfolio)
		engineState.mu.Unlock()
		// TODO: =====================================================

		// TODO: Have CalculateSignals append to this directly somehow
		for _, o := range newOrders {
			pendingOrders.PushBack(&o)
		}

		fmt.Printf("\033[2J\033[3J\033[H=====Info=====\n\x1b[1;33mNAV\x1b[0m:: $%s\n\n=====Positions=====\n%s", portfolio.NAV, metrics.CurrentPositions(portfolio))

		time.Sleep(1000 * time.Millisecond) // so we can watch the numbers on the terminal
	}
}
