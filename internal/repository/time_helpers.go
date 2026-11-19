package repository

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// scannable is implemented by *sql.Row and *sql.Rows.
type scannable interface {
	Scan(dest ...interface{}) error
}

// scanRow wraps Scan to handle SQLite time and JSON values.
// Time destinations parse stored strings; JSON destinations accept TEXT, BLOB,
// and NULL while owning their bytes independently of the driver's buffers.
func scanRow(s scannable, dest ...interface{}) error {
	wrapped := make([]interface{}, len(dest))
	for i, d := range dest {
		switch v := d.(type) {
		case *time.Time:
			wrapped[i] = &sqliteTime{dest: v}
		case **time.Time:
			wrapped[i] = &sqliteNullTime{dest: v}
		case *json.RawMessage:
			wrapped[i] = &sqliteJSON{dest: v}
		default:
			wrapped[i] = d
		}
	}
	return s.Scan(wrapped...)
}

// sqliteJSON scans SQLite TEXT, BLOB, or NULL without validating the JSON.
// Driver-owned byte slices must be copied before the next Scan.
type sqliteJSON struct {
	dest *json.RawMessage
}

func (s *sqliteJSON) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		*s.dest = nil
	case string:
		*s.dest = json.RawMessage(v)
	case []byte:
		if v == nil {
			*s.dest = nil
		} else {
			data := make(json.RawMessage, len(v))
			copy(data, v)
			*s.dest = data
		}
	default:
		return fmt.Errorf("cannot scan %T into json.RawMessage", src)
	}
	return nil
}

// sqliteTime scans a SQLite TEXT value into a time.Time.
type sqliteTime struct {
	dest *time.Time
}

func (s *sqliteTime) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*s.dest = v
		return nil
	case string:
		t, err := parseTime(v)
		if err != nil {
			return err
		}
		*s.dest = t
		return nil
	default:
		return fmt.Errorf("cannot scan %T into time.Time", src)
	}
}

// sqliteNullTime scans a SQLite TEXT value into a *time.Time (nullable).
type sqliteNullTime struct {
	dest **time.Time
}

func (s *sqliteNullTime) Scan(src interface{}) error {
	if src == nil {
		*s.dest = nil
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*s.dest = &v
		return nil
	case string:
		if v == "" {
			*s.dest = nil
			return nil
		}
		t, err := parseTime(v)
		if err != nil {
			return err
		}
		*s.dest = &t
		return nil
	default:
		return fmt.Errorf("cannot scan %T into *time.Time", src)
	}
}

// parseTime attempts to parse a time string using multiple formats.
var timeFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseTime(v string) (time.Time, error) {
	// Strip Go's monotonic clock reading (e.g., " m=+0.123456789")
	if idx := strings.Index(v, " m="); idx > 0 {
		v = v[:idx]
	}
	// Strip timezone name if present (e.g., " MST" before the offset)
	// Format: "2026-01-26 21:57:31.12551 -0700 MST"
	// We want: "2026-01-26 21:57:31.12551 -0700"

	for _, format := range timeFormats {
		if t, err := time.Parse(format, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %q", v)
}
