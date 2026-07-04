package debuglog

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/haohanwu/showdown/internal/agent"
)

func TestWrapAskerPassthroughAndLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	inner := func(ctx context.Context, prompt string) (agent.Response, error) {
		return agent.Response{Text: "reply:" + prompt}, nil
	}
	resp, err := WrapAsker(inner, l)(context.Background(), "hello")
	if err != nil || resp.Text != "reply:hello" {
		t.Fatalf("passthrough = (%q, %v), want (reply:hello, nil)", resp.Text, err)
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
	if _, ok := evs[0]["tokens_in"]; ok {
		t.Error("agent_call has tokens_in field when Usage is nil")
	}
}

func TestWrapAskerLogsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	boom := errors.New("exec failed")
	inner := func(ctx context.Context, prompt string) (agent.Response, error) {
		return agent.Response{}, boom
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
	inner := func(ctx context.Context, prompt string) (agent.Response, error) {
		return agent.Response{Text: "ok"}, nil
	}
	resp, err := WrapAsker(inner, nil)(context.Background(), "p")
	if err != nil || resp.Text != "ok" {
		t.Fatalf("nil-logger passthrough = (%q, %v), want (ok, nil)", resp.Text, err)
	}
}

func TestWrapAskerLogsUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	inner := func(ctx context.Context, prompt string) (agent.Response, error) {
		return agent.Response{Text: "ok", Usage: &agent.Usage{
			InputTokens: 10, OutputTokens: 479, CacheRead: 10690, CacheWrite: 9039, CostUSD: 0.0215,
		}}, nil
	}
	if _, err := WrapAsker(inner, l)(context.Background(), "p"); err != nil {
		t.Fatal(err)
	}
	l.Close()
	evs := readEvents(t, path)
	// JSON numbers decode as float64
	if evs[0]["tokens_in"] != 10.0 || evs[0]["tokens_out"] != 479.0 ||
		evs[0]["cache_read"] != 10690.0 || evs[0]["cache_write"] != 9039.0 ||
		evs[0]["cost_usd"] != 0.0215 {
		t.Errorf("agent_call usage fields = %v", evs[0])
	}
}
