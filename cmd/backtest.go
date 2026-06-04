package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/events"
	"github.com/HashMaster02/slipstream/pkg/strategy"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type MarketDataPacket struct {
	row data.Row
	event events.Event
}

type EngineState struct {
	mu sync.Mutex
	rowHeap datastructures.RowHeap
	eventQueue datastructures.Queue
}

var engineState EngineState = EngineState{
	rowHeap: datastructures.RowHeap{},
	eventQueue: datastructures.NewQueue(),
}


func ReadData(reader *data.Reader, channel chan <- MarketDataPacket) {
	for {
		data, err := reader.Next()
		if err == io.EOF {
			reader.CloseReader()
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}

		e := events.MarketEvent{Type: events.MarketEventType, CreatedAt: time.Now()}
		channel <- MarketDataPacket{row: data, event: e}
	}
}


func ProcessChannels(c <- chan MarketDataPacket, engine *EngineState) {
	// Both channels are unbuffered, which guarantees that we 
	// push data onto the rowHeap BEFORE we push a corresponding
	// MarketEvent onto the eventQueue (which we want). If we ever 
	// buffer either of the channels, we will break this inherent sequence.
	for c != nil {
		packet, ok := <- c
		if !ok {c = nil; continue}

		engine.mu.Lock()
		engine.rowHeap.PushEntry(&packet.row)
		engine.eventQueue.Push(packet.event)
		engine.mu.Unlock()
	}
}


func main() {

	tickers := []string{"AAPL"}

	marketPacketChannel := make(chan MarketDataPacket)

	historicData := make(map[string][]*data.Row)

	go ProcessChannels(marketPacketChannel, &engineState)

	for _, ticker := range tickers {
		filepath := fmt.Sprintf("./data/firstrate/stock_update_month_1min_adjsplit/%s_month_1min_adjsplit.txt", ticker)
		reader, err := data.NewReader(filepath, ticker)
		if err != nil {
			fmt.Print(fmt.Errorf("%s", err))
			os.Exit(-1)
		}

		go ReadData(reader, marketPacketChannel)
	}


	// Main engine loop. Perpetually runs until quit with Ctrl+C
	// TODO: Shutdown engine gracefully on KILL signal (Ctrl+C)
	strategy := strategy.NewBuyAndHoldOnClose(tickers, types.PriceFromFloat(269.00), types.PriceFromFloat(0.50), 100)
	for {
		// The engine waits for an event
		engineState.mu.Lock()
		event, succ := engineState.eventQueue.Pop()
		engineState.mu.Unlock()
		if !succ {
			continue
		}

		switch e := event.(type) {
			case events.MarketEvent: {
				
				engineState.mu.Lock()
				// ======= run in go routine for each active strategy ========
				order, succ := strategy.CalculateSignals(&engineState.rowHeap, &historicData)
				if succ {
					engineState.eventQueue.Push(order)
				}
				// ===========================================================

				row := engineState.rowHeap.PopEntry()
				engineState.mu.Unlock()
				historicData[row.Symbol] = append(historicData[row.Symbol], row)
			}
			case events.OrderEvent: {
				fmt.Printf("BarTimestamp: %v, OrderType: %s, Price: %s, Symbol: %s\n", e.BarTimestamp, e.OrderType.String(), e.Price.String(), e.Symbol)
			}
		}
	}
	
}
