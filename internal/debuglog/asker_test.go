package debuglog

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestWrapAskerPassthroughAndLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	inner := func(ctx context.Context, prompt string) (string, error) {
		return "reply:" + prompt, nil
	}
	raw, err := WrapAsker(inner, l)(context.Background(), "hello")
	if err != nil || raw != "reply:hello" {
		t.Fatalf("passthrough = (%q, %v), want (reply:hello, nil)", raw, err)
	}
	l.Close()

	evs := readEvents(t, path)
	if len(evs) != 1 || evs[0]["event"] != "agent_call" {
		t.Fatalf("events = %v, want one agent_call", evs)
	}
	if evs[0]["prompt"] != "hello" || evs[0]["raw"] != "reply:hello" {
		t.Errorf("agent_call fields = %v", evs[0])
	}
	if _, ok := evs[0]["duration_ms"]; !ok {
		t.Error("agent_call missing duration_ms")
	}
	if _, ok := evs[0]["error"]; ok {
		t.Error("agent_call has error field on success")
	}
}

func TestWrapAskerLogsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	boom := errors.New("exec failed")
	inner := func(ctx context.Context, prompt string) (string, error) {
		return "", boom
	}
	_, err = WrapAsker(inner, l)(context.Background(), "p")
	if !errors.Is(err, boom) {
		t.Fatalf("error not passed through: %v", err)
	}
	l.Close()

	evs := readEvents(t, path)
	if len(evs) != 1 || evs[0]["error"] != "exec failed" {
		t.Fatalf("events = %v, want one agent_call with error", evs)
	}
}

func TestWrapAskerNilLogger(t *testing.T) {
	inner := func(ctx context.Context, prompt string) (string, error) {
		return "ok", nil
	}
	raw, err := WrapAsker(inner, nil)(context.Background(), "p")
	if err != nil || raw != "ok" {
		t.Fatalf("nil-logger passthrough = (%q, %v), want (ok, nil)", raw, err)
	}
}
