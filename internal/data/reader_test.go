package data_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/pkg/types"
)

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestNewReader(t *testing.T) {
	t.Run("opens existing file", func(t *testing.T) {
		path := writeTempCSV(t, "")
		r, err := data.NewReader(path)
		if err != nil {
			t.Fatalf("NewReader(%q) unexpected error: %v", path, err)
		}
		if r == nil {
			t.Fatal("NewReader returned nil reader")
		}
		r.CloseReader()
	})

	t.Run("missing file returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does-not-exist.csv")
		r, err := data.NewReader(path)
		if err == nil {
			r.CloseReader()
			t.Fatalf("NewReader(%q) = %v, nil; want error", path, r)
		}
	})
}

func TestReader_Next(t *testing.T) {
	t.Run("parses a valid row", func(t *testing.T) {
		path := writeTempCSV(t, "2024-01-02 03:04:05,1.2345,2.3456,0.5,1.0001,42\n")
		r, err := data.NewReader(path)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		defer r.CloseReader()

		row, err := r.Next()
		if err != nil {
			t.Fatalf("Next() unexpected error: %v", err)
		}

		wantTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		if !row.Timestamp.Equal(wantTime) {
			t.Errorf("Timestamp = %v, want %v", row.Timestamp, wantTime)
		}
		if row.Open != types.Price(12345) {
			t.Errorf("Open = %d, want %d", row.Open, 12345)
		}
		if row.High != types.Price(23456) {
			t.Errorf("High = %d, want %d", row.High, 23456)
		}
		if row.Low != types.Price(5000) {
			t.Errorf("Low = %d, want %d", row.Low, 5000)
		}
		if row.Close != types.Price(10001) {
			t.Errorf("Close = %d, want %d", row.Close, 10001)
		}
		if row.Volume != 42 {
			t.Errorf("Volume = %d, want %d", row.Volume, 42)
		}
	})

	t.Run("iterates multiple rows then EOF", func(t *testing.T) {
		content := "2024-01-02 03:04:05,1,2,3,4,10\n" +
			"2024-01-02 03:04:06,5,6,7,8,20\n" +
			"2024-01-02 03:04:07,9,10,11,12,30\n"
		path := writeTempCSV(t, content)
		r, err := data.NewReader(path)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		defer r.CloseReader()

		wantVolumes := []int64{10, 20, 30}
		for i, wantVol := range wantVolumes {
			row, err := r.Next()
			if err != nil {
				t.Fatalf("Next() row %d unexpected error: %v", i, err)
			}
			if row.Volume != wantVol {
				t.Errorf("row %d Volume = %d, want %d", i, row.Volume, wantVol)
			}
		}

		if _, err := r.Next(); !errors.Is(err, io.EOF) {
			t.Errorf("Next() at end = %v, want io.EOF", err)
		}
	})

	t.Run("empty file returns EOF immediately", func(t *testing.T) {
		path := writeTempCSV(t, "")
		r, err := data.NewReader(path)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		defer r.CloseReader()

		if _, err := r.Next(); !errors.Is(err, io.EOF) {
			t.Errorf("Next() on empty file = %v, want io.EOF", err)
		}
	})

	t.Run("malformed rows return error", func(t *testing.T) {
		tests := []struct {
			name string
			line string
		}{
			{"too few columns", "2024-01-02 03:04:05,1,2,3,4\n"},
			{"too many columns", "2024-01-02 03:04:05,1,2,3,4,5,6\n"},
			{"bad timestamp", "not-a-time,1,2,3,4,5\n"},
			{"bad open", "2024-01-02 03:04:05,abc,2,3,4,5\n"},
			{"bad high", "2024-01-02 03:04:05,1,abc,3,4,5\n"},
			{"bad low", "2024-01-02 03:04:05,1,2,abc,4,5\n"},
			{"bad close", "2024-01-02 03:04:05,1,2,3,abc,5\n"},
			{"bad volume", "2024-01-02 03:04:05,1,2,3,4,abc\n"},
			{"fractional volume", "2024-01-02 03:04:05,1,2,3,4,1.5\n"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				path := writeTempCSV(t, tc.line)
				r, err := data.NewReader(path)
				if err != nil {
					t.Fatalf("NewReader: %v", err)
				}
				defer r.CloseReader()

				if _, err := r.Next(); err == nil {
					t.Errorf("Next() on %q = nil; want error", tc.line)
				}
			})
		}
	})

	t.Run("error on bad row does not stop iteration", func(t *testing.T) {
		content := "2024-01-02 03:04:05,1,2,3,4,10\n" +
			"bad,row,here,really,bad,nope\n" +
			"2024-01-02 03:04:07,5,6,7,8,30\n"
		path := writeTempCSV(t, content)
		r, err := data.NewReader(path)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		defer r.CloseReader()

		if _, err := r.Next(); err != nil {
			t.Fatalf("Next() row 0 unexpected error: %v", err)
		}
		if _, err := r.Next(); err == nil {
			t.Error("Next() row 1: want error on malformed row")
		}
		row, err := r.Next()
		if err != nil {
			t.Fatalf("Next() row 2 unexpected error: %v", err)
		}
		if row.Volume != 30 {
			t.Errorf("row 2 Volume = %d, want 30", row.Volume)
		}
	})
}

func TestReader_CloseReader(t *testing.T) {
	path := writeTempCSV(t, "2024-01-02 03:04:05,1,2,3,4,10\n")
	r, err := data.NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	r.CloseReader()
}
