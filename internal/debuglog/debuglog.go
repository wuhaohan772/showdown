// Package debuglog writes an opt-in JSONL transcript of a showdown session
// (spec: docs/superpowers/specs/2026-07-03-debug-transcript-log-design.md).
// A nil *Logger is a no-op everywhere, so call sites need no enabled-checks.
package debuglog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu     sync.Mutex
	f      *os.File
	seq    int
	closed bool
}

// DefaultPath returns ~/.showdown/debug-<YYYYMMDD-HHMMSS>.jsonl.
func DefaultPath(now time.Time) string {
	name := "debug-" + now.Format("20060102-150405") + ".jsonl"
	home, err := os.UserHomeDir()
	if err != nil {
		return name
	}
	return filepath.Join(home, ".showdown", name)
}

func New(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f}, nil
}

// Log writes one line: {"seq":N,"t":"...","event":name, ...fields}.
// Best-effort: marshal/write errors are dropped so logging can never
// stall or crash the game.
func (l *Logger) Log(event string, fields map[string]any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.seq++
	rec := make(map[string]any, len(fields)+3)
	for k, v := range fields {
		rec[k] = v
	}
	rec["seq"] = l.seq
	rec["t"] = time.Now().Format(time.RFC3339Nano)
	rec["event"] = event
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_, _ = l.f.Write(append(b, '\n'))
}

func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	return l.f.Close()
}
