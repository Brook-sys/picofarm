package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type jsonScanFunc func(...interface{}) error

func (f jsonScanFunc) Scan(dest ...interface{}) error { return f(dest...) }

func TestScanRowRawMessage(t *testing.T) {
	for _, tt := range []struct {
		name string
		src  interface{}
		want json.RawMessage
	}{
		{"null", nil, nil},
		{"text", `{"name":"test"}`, json.RawMessage(`{"name":"test"}`)},
		{"blob", []byte(`[1,true]`), json.RawMessage(`[1,true]`)},
		{"empty text", "", json.RawMessage{}},
		{"empty blob", []byte{}, json.RawMessage{}},
		{"nil blob", []byte(nil), nil},
		{"json null", "null", json.RawMessage("null")},
		{"unvalidated text", "not JSON", json.RawMessage("not JSON")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := json.RawMessage(`{"previous":true}`)
			row := jsonScanFunc(func(dest ...interface{}) error {
				return dest[0].(sql.Scanner).Scan(tt.src)
			})
			if err := scanRow(row, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestScanRowRawMessageCopiesDriverBytes(t *testing.T) {
	src := []byte(`{"value":1}`)
	// Even if the destination aliases the source before Scan, ownership must change.
	got := json.RawMessage(src)
	row := jsonScanFunc(func(dest ...interface{}) error {
		return dest[0].(sql.Scanner).Scan(src)
	})
	if err := scanRow(row, &got); err != nil {
		t.Fatal(err)
	}
	src[0] = 'x'
	if string(got) != `{"value":1}` {
		t.Fatalf("destination aliases driver bytes: %q", got)
	}
	got[1] = 'y'
	if src[1] != '"' {
		t.Fatal("mutating destination changed source")
	}
}

func TestScanRowRawMessageUnsupportedType(t *testing.T) {
	for _, src := range []interface{}{int64(1), float64(1), true, time.Time{}} {
		got := json.RawMessage(`{"previous":true}`)
		row := jsonScanFunc(func(dest ...interface{}) error {
			return dest[0].(sql.Scanner).Scan(src)
		})
		if err := scanRow(row, &got); err == nil || !strings.Contains(err.Error(), "json.RawMessage") {
			t.Fatalf("expected conversion error for %T, got %v", src, err)
		}
		if string(got) != `{"previous":true}` {
			t.Fatal("failed scan changed destination")
		}
	}
}

func TestScanRowRawMessagePreservesOtherDestinations(t *testing.T) {
	var raw json.RawMessage
	var stamp time.Time
	var nullable *time.Time
	var count int
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	row := jsonScanFunc(func(dest ...interface{}) error {
		if dest[3] != &count {
			t.Fatal("ordinary destination was wrapped")
		}
		for i, src := range []interface{}{`{}`, want.Format(time.RFC3339), want.Format(time.RFC3339)} {
			if err := dest[i].(sql.Scanner).Scan(src); err != nil {
				return err
			}
		}
		*dest[3].(*int) = 7
		return nil
	})
	if err := scanRow(row, &raw, &stamp, &nullable, &count); err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{}` || !stamp.Equal(want) || nullable == nil || !nullable.Equal(want) || count != 7 {
		t.Fatalf("unexpected values: %s %v %v %d", raw, stamp, nullable, count)
	}
}

func TestScanRowRawMessagePropagatesScanError(t *testing.T) {
	want := errors.New("scan failed")
	var raw json.RawMessage
	row := jsonScanFunc(func(...interface{}) error { return want })
	if err := scanRow(row, &raw); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}
