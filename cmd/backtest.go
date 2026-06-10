package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/metrics"
	"github.com/HashMaster02/slipstream/internal/portfolio"
	"github.com/HashMaster02/slipstream/pkg/strategy"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type EngineState struct {
	mu         sync.Mutex
	rowHeap    datastructures.RowHeap
	eventQueue datastructures.Queue
}

var engineState EngineState = EngineState{
	rowHeap:    datastructures.RowHeap{},
	eventQueue: datastructures.NewQueue(),
}

// TODO: Move this somewhere else at some point
var latestBar map[string]*data.Row = make(map[string]*data.Row)

func ProcessChannels(c <-chan data.MarketDataPacket, engine *EngineState) {
	// Both channels are unbuffered, which guarantees that we
	// push data onto the rowHeap BEFORE we push a corresponding
	// MarketEvent onto the eventQueue (which we want). If we ever
	// buffer either of the channels, we will break this inherent sequence.
	for c != nil {
		packet, ok := <-c
		if !ok {
			c = nil
			continue
		}

		engine.mu.Lock()
		engine.rowHeap.PushEntry(&packet.Row)
		engine.eventQueue.Push(packet.Event)
		engine.mu.Unlock()
	}
}

func main() {

	tickers := []string{"AAPL", "GS", "MSFT", "NVDA", "META", "GOOG", "T", "JPM"}

	marketPacketChannel := make(chan data.MarketDataPacket)

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

	portfolio := &portfolio.Portfolio{
		Positions: make(map[string]*portfolio.Position),
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
		var head *data.Row = engineState.rowHeap.Peek()
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

			var nextBar *data.Row = engineState.rowHeap.PopEntry()
			engineState.mu.Unlock()
			latestBar[nextBar.Symbol] = nextBar

			// Update latest market prices for portfolio
			// ! Edge Case: What if our portfolio has positions for securities we are not currently processing?
			portfolio.UpdatePrice(*nextBar)
		}

		// TODO: Process OrderBook

		// Calculate signals from strategies
		// TODO: run in go routine for each active strategy ==========
		engineState.mu.Lock()
		orders, succ := strategy.CalculateSignals(&latestBar)
		if succ {
			for _, order := range orders {
				portfolio.UpdatePosition(order)
			}
		}
		engineState.mu.Unlock()
		// TODO: =====================================================

		nav := metrics.NetAssetValue(portfolio)
		fmt.Printf("\033[2J\033[3J\033[H%s", nav)

		time.Sleep(500 * time.Millisecond) // so we can watch the numbers on the terminal
	}
}
