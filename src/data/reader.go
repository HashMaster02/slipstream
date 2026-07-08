package data

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/HashMaster02/slipstream/src/types"
)

type Quote struct {
	Timestamp time.Time
	Symbol    string
	Bid 	  types.Price
	Ask       types.Price
	Last      types.Price
}

type Reader struct {
	file       *os.File
	symbol     string
	scanner    *bufio.Scanner
	currCandle uint64
}

func NewReader(path string, sym string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file %s: %w", path, err)
	}

	scanner := bufio.NewScanner(file)

	return &Reader{file: file, symbol: sym, scanner: scanner, currCandle: 0}, nil
}

func (r *Reader) CloseReader() {
	r.file.Close()
}

func (r *Reader) Next() (Quote, error) {

	const TIME_LAYOUT string = "2006-01-02 15:04:05"
	const ELEMENTS_PER_ROW = 4

	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return Quote{}, fmt.Errorf("scan: %w", err)
		}
		return Quote{}, io.EOF
	}

	r.currCandle++
	line := r.scanner.Text()
	data := strings.Split(line, ",")

	if len(data) != ELEMENTS_PER_ROW {
		return Quote{}, fmt.Errorf("Error on row %d. The row does not have enough items. Expected %d, got %d.", r.currCandle, ELEMENTS_PER_ROW, len(data))
	}

	time, err := time.Parse(TIME_LAYOUT, strings.TrimSpace(data[0]))
	if err != nil {
		return Quote{}, fmt.Errorf("error parsing time string %s: %w", data[0], err)
	}

	bid, err := types.ParsePrice(strings.TrimSpace(data[1]))
	if err != nil {
		return Quote{}, fmt.Errorf("error parsing numeric value %s: %w", data[1], err)
	}

	ask, err := types.ParsePrice(strings.TrimSpace(data[2]))
	if err != nil {
		return Quote{}, fmt.Errorf("error parsing numeric value %s: %w", data[2], err)
	}

	last, err := types.ParsePrice(strings.TrimSpace(data[3]))
	if err != nil {
		return Quote{}, fmt.Errorf("error parsing numeric value %s: %w", data[3], err)
	}

	row := Quote{
		Timestamp: time,
		Symbol:    r.symbol,
		Bid:       bid,
		Ask:       ask,
		Last:      last,
	}

	return row, nil
}

func ReadData(reader *Reader, channel chan<- Quote) {
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

		channel <- data
	}
}
