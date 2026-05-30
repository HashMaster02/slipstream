package main

import (
	"container/heap"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/events"
)

func main() {

	tickers := []string{"AAPL", "MSFT", "GS"}
	h := &datastructures.ReaderHeap{}

	for _, ticker := range tickers {
		filepath := fmt.Sprintf("./data/firstrate/stock_update_month_1min_adjsplit/%s_month_1min_adjsplit.txt", ticker)
		reader, err := data.NewReader(filepath)
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
		heap.Push(h, &datastructures.ReaderEntry{Row: row, R: reader, Symbol: ticker})
	}

	// Demo: pop the next 20 bars in global timestmap order
	// and push a MarketEvent for each onto the event queue
	event_queue := datastructures.NewQueue()
	for i := 0; i < 20 && h.Len() > 0; i++ {
		e := heap.Pop(h).(*datastructures.ReaderEntry)

		event := events.MarketEvent{Type: events.MarketEventType, Symbol: e.Symbol, CreatedAt: time.Now()}
		event_queue.Push(event)

		next, err := e.R.Next()
		if err == io.EOF {
			e.R.CloseReader()
			continue
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		e.Row = next
		heap.Push(h, e)
	}

	// Print out each event in the event queue. We only have Market events for now
	for {
		event, succ := event_queue.Pop()
		if !succ {
			break
		}

		switch e := event.(type) {
			case events.MarketEvent: fmt.Printf("EVENT:: Type=%s, Symbol=%s, Timestamp=%v\n", e.Type, e.Symbol, e.CreatedAt)
		}
	}
	
}