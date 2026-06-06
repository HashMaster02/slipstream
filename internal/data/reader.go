package data

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HashMaster02/slipstream/pkg/types"
)

type Row struct {
	Timestamp time.Time
	Symbol    string
	Open      types.Price
	High      types.Price
	Low       types.Price
	Close     types.Price
	Volume    int64
}

type Reader struct {
	file    *os.File
	symbol  string
	scanner *bufio.Scanner
	currRow uint64
}

func NewReader(path string, sym string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file %s: %w", path, err)
	}

	scanner := bufio.NewScanner(file)

	return &Reader{file: file, symbol: sym, scanner: scanner, currRow: 0}, nil
}

func (r *Reader) CloseReader() {
	r.file.Close()
}

func (r *Reader) Next() (Row, error) {

	const TIME_LAYOUT string = "2006-01-02 15:04:05"
	const ELEMENTS_PER_ROW = 6

	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return Row{}, fmt.Errorf("scan: %w", err)
		}
		return Row{}, io.EOF
	}

	r.currRow++
	line := r.scanner.Text()
	data := strings.Split(line, ",")

	if len(data) != ELEMENTS_PER_ROW {
		return Row{}, fmt.Errorf("Error on row %d. The row does not have enough items. Expected %d, got %d.", r.currRow, ELEMENTS_PER_ROW, len(data))
	}

	time, err := time.Parse(TIME_LAYOUT, data[0])
	if err != nil {
		return Row{}, fmt.Errorf("error parsing time string %s: %w", data[0], err)
	}

	open, err := types.ParsePrice(data[1])
	if err != nil {
		return Row{}, fmt.Errorf("error parsing numeric value %s: %w", data[1], err)
	}

	high, err := types.ParsePrice(data[2])
	if err != nil {
		return Row{}, fmt.Errorf("error parsing numeric value %s: %w", data[2], err)
	}

	low, err := types.ParsePrice(data[3])
	if err != nil {
		return Row{}, fmt.Errorf("error parsing numeric value %s: %w", data[3], err)
	}

	close, err := types.ParsePrice(data[4])
	if err != nil {
		return Row{}, fmt.Errorf("error parsing numeric value %s: %w", data[4], err)
	}

	volume, err := strconv.ParseInt(data[5], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("error parsing numeric value %s: %w", data[5], err)
	}

	row := Row{
		Timestamp: time,
		Symbol:    r.symbol,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}

	return row, nil
}
