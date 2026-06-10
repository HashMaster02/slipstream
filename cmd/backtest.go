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
	"github.com/HashMaster02/slipstream/internal/events"
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

	historicData := make(map[string][]*data.Row)

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
	strategy := strategy.NewTakeProfit(tickers, types.PriceFromFloat(2.50), 100)

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

		// The engine waits for an event
		engineState.mu.Lock()
		event, succ := engineState.eventQueue.Pop()
		engineState.mu.Unlock()
		if !succ {
			continue
		}

		switch e := event.(type) {
		case events.MarketEvent:
			{

				engineState.mu.Lock()
				// TODO: run in go routine for each active strategy ==========
				order, succ := strategy.CalculateSignals(&engineState.rowHeap, &historicData)
				if succ {
					engineState.eventQueue.Push(order)
				}
				// TODO: =====================================================

				row := engineState.rowHeap.PopEntry()
				portfolio.UpdatePrice(*row)
				engineState.mu.Unlock()
				historicData[row.Symbol] = append(historicData[row.Symbol], row)
			}
		case events.OrderEvent:
			{
				portfolio.UpdatePosition(e)
				nav := metrics.NetAssetValue(portfolio)
				fmt.Printf("\033[2J\033[3J\033[H%s", nav)

				time.Sleep(1 * time.Second)
				// ticker, mVal := metrics.LargestPosition(portfolio)
				// succ := data.WriteToText(*METRIC_OUTPUT_FILE, fmt.Sprintf("LargestPosition:: Ticker: %s, MarketValue: %d\n", ticker, mVal))
				// if !succ {
				// 	fmt.Println("Failed to write to file.")
				// }

			}
		}
	}
}
