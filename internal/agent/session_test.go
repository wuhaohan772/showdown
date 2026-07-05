package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestHelperFakeClaude is not a test: it is the fake claude CLI, exec'd by
// startFakeSession as a subprocess (standard Go helper-process pattern).
// It speaks just enough of the stream-json protocol for Session:
// reads JSONL user messages, replies with an assistant event and a result
// event. FAKE_BEHAVIOR: "ok" (default), "die" (exit before replying),
// "hang" (never reply). total_cost_usd is cumulative on purpose.
func TestHelperFakeClaude(t *testing.T) {
	if os.Getenv("GO_FAKECLAUDE") != "1" {
		t.Skip("helper process")
	}
	behavior := os.Getenv("FAKE_BEHAVIOR")
	turn := 0
	sc := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		turn++
		switch behavior {
		case "die":
			os.Exit(1)
		case "hang":
			time.Sleep(time.Minute)
		case "error_result":
			_ = out.Encode(map[string]any{
				"type": "result", "subtype": "error_max_turns", "is_error": true,
				"result": "", "total_cost_usd": 0.0,
				"usage": map[string]any{
					"input_tokens": 0, "output_tokens": 0,
					"cache_read_input_tokens": 0, "cache_creation_input_tokens": 0,
				},
			})
			// process stays alive; session.kill() will reap it
			time.Sleep(time.Minute)
		}
		var in struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(sc.Bytes(), &in); err != nil {
			os.Exit(2)
		}
		cacheRead := 0
		if turn > 1 {
			cacheRead = 100
		}
		_ = out.Encode(map[string]any{"type": "assistant"})
		_ = out.Encode(map[string]any{
			"type":           "result",
			"subtype":        "success",
			"result":         fmt.Sprintf("turn%d:%s", turn, in.Message.Content[0].Text),
			"total_cost_usd": 0.01 * float64(turn),
			"usage": map[string]any{
				"input_tokens": 3, "output_tokens": 5,
				"cache_read_input_tokens":     cacheRead,
				"cache_creation_input_tokens": 10,
			},
		})
	}
	os.Exit(0)
}

func startFakeSession(t *testing.T, behavior string, timeout time.Duration) *Session {
	t.Helper()
	s, err := StartSession(os.Args[0], []string{"-test.run=TestHelperFakeClaude"},
		[]string{"GO_FAKECLAUDE=1", "FAKE_BEHAVIOR=" + behavior}, t.TempDir(), timeout)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestSessionTwoTurnsAndCostDelta(t *testing.T) {
	s := startFakeSession(t, "ok", 5*time.Second)
	r1, err := s.Ask(context.Background(), "hello")
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if r1.Text != "turn1:hello" {
		t.Errorf("turn 1 text = %q", r1.Text)
	}
	if r1.Usage == nil || r1.Usage.CostUSD < 0.009 || r1.Usage.CostUSD > 0.011 {
		t.Errorf("turn 1 cost = %+v, want ~0.01", r1.Usage)
	}
	r2, err := s.Ask(context.Background(), "again")
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if r2.Text != "turn2:again" {
		t.Errorf("turn 2 text = %q", r2.Text)
	}
	// cumulative 0.02 - 0.01 = per-turn 0.01
	if r2.Usage == nil || r2.Usage.CostUSD < 0.009 || r2.Usage.CostUSD > 0.011 {
		t.Errorf("turn 2 cost delta = %+v, want ~0.01", r2.Usage)
	}
	if r2.Usage.CacheRead != 100 {
		t.Errorf("turn 2 cache read = %d, want 100", r2.Usage.CacheRead)
	}
	if !s.Alive() {
		t.Error("session should be alive after clean turns")
	}
}

func TestSessionDiesOnProcessExit(t *testing.T) {
	s := startFakeSession(t, "die", 5*time.Second)
	if _, err := s.Ask(context.Background(), "x"); err == nil {
		t.Fatal("Ask should error when the process dies")
	}
	if s.Alive() {
		t.Error("session must be dead after process exit")
	}
	// dead session errors fast, does not hang
	if _, err := s.Ask(context.Background(), "y"); err == nil {
		t.Error("Ask on dead session should error")
	}
}

func TestSessionTimeoutKills(t *testing.T) {
	s := startFakeSession(t, "hang", 300*time.Millisecond)
	start := time.Now()
	_, err := s.Ask(context.Background(), "x")
	if err == nil {
		t.Fatal("Ask should time out")
	}
	if time.Since(start) > 3*time.Second {
		t.Error("timeout took too long — turn timeout not applied")
	}
	if s.Alive() {
		t.Error("session must be dead after timeout")
	}
}

func TestSessionPrimedFlagAndNilClose(t *testing.T) {
	s := startFakeSession(t, "ok", 5*time.Second)
	if s.Primed() {
		t.Error("new session must not be primed")
	}
	s.MarkPrimed()
	if !s.Primed() {
		t.Error("MarkPrimed did not stick")
	}
	var nilS *Session
	nilS.Close() // must not panic
	if nilS.Alive() {
		t.Error("nil session is not alive")
	}
}

func TestSessionErrorResultKills(t *testing.T) {
	s := startFakeSession(t, "error_result", 5*time.Second)
	_, err := s.Ask(context.Background(), "x")
	if err == nil {
		t.Fatal("Ask should error when the session returns an error result")
	}
	if s.Alive() {
		t.Error("session must be dead after error result")
	}
	// second Ask must also error fast, not hang
	if _, err := s.Ask(context.Background(), "y"); err == nil {
		t.Error("Ask on dead session should error")
	}
}

func TestSessionAskerSignature(t *testing.T) {
	s := startFakeSession(t, "ok", 5*time.Second)
	var ask Asker = s.Asker()
	r, err := ask(context.Background(), "sig")
	if err != nil || !strings.Contains(r.Text, "sig") {
		t.Errorf("Asker() adapter broken: %v %q", err, r.Text)
	}
}
