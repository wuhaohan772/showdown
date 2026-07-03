package debuglog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", sc.Text(), err)
		}
		out = append(out, m)
	}
	return out
}

func TestLogWritesJSONLWithSeq(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Log("first", map[string]any{"k": "v"})
	l.Log("second", nil)
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	evs := readEvents(t, path)
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2", len(evs))
	}
	if evs[0]["event"] != "first" || evs[0]["k"] != "v" {
		t.Errorf("event 0 = %v", evs[0])
	}
	if evs[0]["seq"] != float64(1) || evs[1]["seq"] != float64(2) {
		t.Errorf("seq = %v, %v; want 1, 2", evs[0]["seq"], evs[1]["seq"])
	}
	ts, _ := evs[0]["t"].(string)
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("t %q not RFC3339Nano: %v", ts, err)
	}
}

func TestNewCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New with missing parent dir: %v", err)
	}
	l.Close()
}

func TestNilLoggerIsNoop(t *testing.T) {
	var l *Logger
	l.Log("x", map[string]any{"k": 1}) // must not panic
	if err := l.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
}

func TestLogAfterCloseIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Close()
	l.Log("late", nil)                // must not panic
	if err := l.Close(); err != nil { // double close must not error
		t.Errorf("double Close: %v", err)
	}
	if evs := readEvents(t, path); len(evs) != 0 {
		t.Errorf("got %d events after close, want 0", len(evs))
	}
}

func TestDefaultPath(t *testing.T) {
	now := time.Date(2026, 7, 3, 14, 5, 6, 0, time.UTC)
	p := DefaultPath(now)
	if !strings.HasSuffix(p, filepath.Join(".showdown", "debug-20260703-140506.jsonl")) {
		t.Errorf("DefaultPath = %q", p)
	}
}
