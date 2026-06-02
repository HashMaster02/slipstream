package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/events"
)

type MarketDataPacket struct {
	row data.Row
	event events.Event
}
var rowHeap datastructures.RowHeap = datastructures.RowHeap{}  // This stores the data from the market data source
var eventQueue datastructures.Queue = datastructures.NewQueue()

func ReadData(reader *data.Reader, channel chan <- MarketDataPacket) {
	for range 5 {
		data, err := reader.Next()
		if err == io.EOF {
			reader.CloseReader()
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		e := events.MarketEvent{Type: events.MarketEventType, CreatedAt: time.Now()}
		channel <- MarketDataPacket{row: data, event: e}
	}
}


func ProcessChannels(c <- chan MarketDataPacket) {
	// Both channels are unbuffered, which guarantees that we 
	// push data onto the rowHeap BEFORE we push a corresponding
	// MarketEvent onto the eventQueue (which we want). If we ever 
	// buffer either of the channels, we will break this inherent sequence.
	for c != nil {
		packet, ok := <- c
		if !ok {c = nil; continue}
		rowHeap.PushEntry(&packet.row)
		eventQueue.Push(packet.event)
	}
}


func main() {

	tickers := []string{"AAPL", "MSFT", "GS"}

	marketPacketChannel := make(chan MarketDataPacket)

	go ProcessChannels(marketPacketChannel)

	for _, ticker := range tickers {
		filepath := fmt.Sprintf("./data/firstrate/stock_update_month_1min_adjsplit/%s_month_1min_adjsplit.txt", ticker)
		reader, err := data.NewReader(filepath, ticker)
		if err != nil {
			fmt.Print(fmt.Errorf("%s", err))
			os.Exit(-1)
		}

		row, err := reader.Next()
		if err == io.EOF {
			reader.CloseReader()
			continue
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rowHeap.PushEntry(&row)
		go ReadData(reader, marketPacketChannel)
	}

	for {
		event, succ := eventQueue.Pop()
		if !succ {
			break
		}

		switch e := event.(type) {
			case events.MarketEvent: {
				fmt.Printf("EVENT:: Type=%s, EngineTimestamp: %v\n", e.Type, e.CreatedAt)
				row := rowHeap.PopEntry()
				fmt.Printf("Symbol: %s, Timestamp: %v\n", row.Symbol, row.Timestamp)
			}

		}
	}

	close(marketPacketChannel)
	
}
