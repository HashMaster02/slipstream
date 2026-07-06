package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HashMaster02/slipstream/pkg/generate"
)

func rowToCandle(line string, CURRCANDLE uint64) (generate.Candle, error) {
	const TIME_LAYOUT string = "2006-01-02 15:04:05"
	ELEMENTS_PER_ROW := 6

	data := strings.Split(line, ",")

	if len(data) != ELEMENTS_PER_ROW {
		return generate.Candle{}, fmt.Errorf("Error on row %d. The row does not have enough items. Expected %d, got %d.", CURRCANDLE, ELEMENTS_PER_ROW, len(data))
	}

	time, err := time.Parse(TIME_LAYOUT, data[0])
	if err != nil {
		return generate.Candle{}, fmt.Errorf("error parsing time string %s: %w", data[0], err)
	}
	open, err := strconv.ParseFloat(data[1], 64)
	if err != nil {
		return generate.Candle{}, fmt.Errorf("error parsing numeric value %s: %w", data[1], 64, err)
	}

	high, err := strconv.ParseFloat(data[2], 64)
	if err != nil {
		return generate.Candle{}, fmt.Errorf("error parsing numeric value %s: %w", data[2], 64, err)
	}

	low, err := strconv.ParseFloat(data[3], 64)
	if err != nil {
		return generate.Candle{}, fmt.Errorf("error parsing numeric value %s: %w", data[3], 64, err)
	}

	close, err := strconv.ParseFloat(data[4], 64)
	if err != nil {
		return generate.Candle{}, fmt.Errorf("error parsing numeric value %s: %w", data[4], err)
	}

	volume, err := strconv.ParseInt(data[5], 10, 64)
	if err != nil {
		return generate.Candle{}, fmt.Errorf("error parsing numeric value %s: %w", data[5], err)
	}

	row := generate.Candle{
			Timestamp: time,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		}
	
	return row, nil

}

func twoCandleWindow(scanner *bufio.Scanner, CURRCANDLE uint64) (generate.Candle, generate.Candle, error){
	candle1, err := rowToCandle(scanner.Text(), CURRCANDLE)
	if err != nil {
		return generate.Candle{}, generate.Candle{}, fmt.Errorf("ERROR while reading single candle: %w", err)
	}

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return candle1, generate.Candle{}, fmt.Errorf("error while reading file: %w", err)
		}
		// Clean end of file: candle1 has no partner, so no quote is produced.
		return candle1, generate.Candle{}, io.EOF
	}

	candle2, err := rowToCandle(scanner.Text(), CURRCANDLE+1)
	if err != nil {
		return candle1, generate.Candle{}, fmt.Errorf("ERROR while reading single candle: %w", err)
	}

	return candle1, candle2, nil
}

func main() {

	const TIME_LAYOUT string = "2006-01-02 15:04:05"
	const BASE_PATH = "./_data/firstrate"
	const TICKER = "DTSQR"
	const OHLC_DATA_PATH = BASE_PATH + "/stock_update_month_1min_adjsplit/" + TICKER + "_month_1min_adjsplit.txt"
	const QUOTE_DATA_PATH = BASE_PATH + "/stock_update_month_1min_quote/" + TICKER + "_month_1min_quote.txt"
	var CURRCANDLE uint64 = 0


	output_file, err := os.Create(QUOTE_DATA_PATH)
	if err != nil {
		fmt.Println(fmt.Errorf("could not open file %s: %w", QUOTE_DATA_PATH, err))
		os.Exit(1)
	}
	defer output_file.Close()
	writer := bufio.NewWriter(output_file)
	defer func() {
		if err := writer.Flush(); err != nil {
			fmt.Println(fmt.Errorf("error flushing output: %w", err))
		}
	}()

	input_file, err := os.Open(OHLC_DATA_PATH)
	if err != nil {
		fmt.Println(fmt.Errorf("could not open file %s: %w", OHLC_DATA_PATH, err))
		os.Exit(1)
	}
	defer input_file.Close()

	scanner := bufio.NewScanner(input_file)


	for {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Println(fmt.Errorf("error while reading file: %w", err))
				break
			}
			break
		}
		CURRCANDLE++
		candle, err := rowToCandle(scanner.Text(), CURRCANDLE)

		var quote generate.Quote = generate.HighLowAsAskBid(candle)

		_, err = writer.WriteString(fmt.Sprintf("%s, %f, %f, %f\n", quote.Timestamp.Format(TIME_LAYOUT), quote.Bid, quote.Ask, quote.Last))
		if err != nil {
			fmt.Println(fmt.Errorf("ERROR while writing quote: %w", err))
			continue
		}
	}
	
}