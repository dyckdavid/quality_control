// Package report builds CSV and simple PDF exports for QC records.
package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"
)

// WriteCSV encodes rows as UTF-8 CSV with a header line.
func WriteCSV(header []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	// UTF-8 BOM helps Excel open the file correctly.
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FormatTime formats a time for CSV cells.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// FormatPtrTime formats an optional time.
func FormatPtrTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return FormatTime(*t)
}

// FormatFloat formats a float for CSV.
func FormatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// FormatPtrFloat formats an optional float.
func FormatPtrFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return FormatFloat(*v)
}

// FormatBool formats a bool for CSV.
func FormatBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// FormatInt formats an int for CSV.
func FormatInt(v int) string {
	return strconv.Itoa(v)
}

// Filename stamps a report basename with UTC date.
func Filename(prefix, ext string) string {
	return fmt.Sprintf("%s-%s.%s", prefix, time.Now().UTC().Format("20060102"), ext)
}
