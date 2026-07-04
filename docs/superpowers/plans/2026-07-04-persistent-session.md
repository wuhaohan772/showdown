# Persistent Opponent Session Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One long-lived `claude` stream-json process per match — priming turn carries the full prompt, each later decision is a small delta turn, match-end reaction rides the same session — per `docs/superpowers/specs/2026-07-04-persistent-session-design.md`.

**Architecture:** New `agent.Session` (owns the CLI process, speaks JSONL stream protocol, reports per-turn usage with cumulative-cost deltas). Claude's adapter gains a nilable `SessionArgs` capability. `GetDecision`/`GetReaction` gain prompt-explicit variants so the TUI can pass delta prompts. The TUI holds the session lazily, falls back to today's stateless spawn on any session failure (ADR-0001: a match never stalls), and lazily restarts a dead session with a full-digest priming message.

**Tech Stack:** Go, os/exec, bufio; tests use the Go helper-process pattern (no bash stub, no new deps).

## Global Constraints

- Session CLI invocation exactly: `claude -p --verbose --input-format stream-json --output-format stream-json --disallowedTools '*' --system-prompt <claudeSystemPrompt>` plus `--model <m>` when the model is non-empty; cwd = launch dir; env = `os.Environ()` + adapter Env (`MAX_THINKING_TOKENS=0`).
- stdin message framing exactly: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":<prompt>}]}}` + `\n`.
- A turn ends at the next stdout event with `"type":"result"`; its `usage` fields are per-turn, but `total_cost_usd` is cumulative — Session must report the delta as `Usage.CostUSD`.
- `Session.Ask` must satisfy the existing `agent.Asker` signature `func(ctx, prompt) (Response, error)` so `debuglog.WrapAsker` composes unchanged.
- Sessions are claude-only; codex/gemini/custom stateless paths must not change behavior.
- ADR-0001: no decision may stall a match — any session failure falls back to a stateless spawn within the same decision.
- Per-turn timeout = the TUI's existing `decisionTimeout` (45s), passed at session start.
- Run `gofmt -w` on every file you touch before committing.

---

### Task 1: Session core

**Files:**
- Create: `internal/agent/session.go`
- Create: `internal/agent/session_test.go`

**Interfaces:**
- Consumes: `Response`, `Usage` (response.go), `Asker` (decide.go:18).
- Produces (used by Tasks 2 and 4):
  - `func StartSession(bin string, args, env []string, dir string, timeout time.Duration) (*Session, error)`
  - `func (s *Session) Ask(ctx context.Context, prompt string) (Response, error)`
  - `func (s *Session) Asker() Asker` — method value adapter
  - `func (s *Session) Alive() bool`
  - `func (s *Session) Primed() bool` / `func (s *Session) MarkPrimed()`
  - `func (s *Session) Close()` — nil-safe

- [ ] **Step 1: Write the failing tests (with helper-process stub)**

`internal/agent/session_test.go`:

```go
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

func TestSessionAskerSignature(t *testing.T) {
	s := startFakeSession(t, "ok", 5*time.Second)
	var ask Asker = s.Asker()
	r, err := ask(context.Background(), "sig")
	if err != nil || !strings.Contains(r.Text, "sig") {
		t.Errorf("Asker() adapter broken: %v %q", err, r.Text)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run 'TestSession' -v`
Expected: compile error — `StartSession` undefined.

- [ ] **Step 3: Implement Session**

`internal/agent/session.go`:

```go
package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Session owns one long-lived agent CLI process speaking the stream-json
// protocol (spec: docs/superpowers/specs/2026-07-04-persistent-session-design.md).
// The conversation is the opponent's match memory: turn 1 carries the full
// prompt, later turns small deltas, so the API prompt cache stays warm
// (~$0.001/decision vs ~$0.015 stateless) and process spawn is paid once.
// The TUI's one-turn-at-a-time flow means Ask is never called concurrently.
type Session struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	results chan streamResult
	timeout time.Duration

	mu       sync.Mutex
	dead     bool
	primed   bool
	lastCost float64
}

type streamResult struct {
	Type         string  `json:"type"`
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

type streamUserMsg struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content []streamContent `json:"content"`
	} `json:"message"`
}

type streamContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// StartSession launches the CLI and its reader goroutine. timeout bounds
// each turn (the TUI passes its decisionTimeout).
func StartSession(bin string, args, env []string, dir string, timeout time.Duration) (*Session, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	s := &Session{cmd: cmd, stdin: stdin, results: make(chan streamResult, 1), timeout: timeout}
	go s.readLoop(stdout)
	return s, nil
}

// readLoop forwards result events; other event types (system, assistant,
// rate_limit_event) are skipped. Channel close = stdout EOF = process gone.
func (s *Session) readLoop(stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var r streamResult
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			continue
		}
		if r.Type == "result" {
			s.results <- r
		}
	}
	close(s.results)
}

// Ask sends one user message and blocks for its result event. Any failure
// (write error, EOF, timeout, malformed stream) kills the session — the
// caller's fallback path takes over and a later decision may start a fresh
// session.
func (s *Session) Ask(ctx context.Context, prompt string) (Response, error) {
	if !s.Alive() {
		return Response{}, fmt.Errorf("session dead")
	}
	var msg streamUserMsg
	msg.Type = "user"
	msg.Message.Role = "user"
	msg.Message.Content = []streamContent{{Type: "text", Text: prompt}}
	b, err := json.Marshal(msg)
	if err != nil {
		return Response{}, err
	}
	if _, err := s.stdin.Write(append(b, '\n')); err != nil {
		s.kill()
		return Response{}, fmt.Errorf("session write: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	select {
	case r, ok := <-s.results:
		if !ok {
			s.kill()
			return Response{}, fmt.Errorf("session closed stream")
		}
		s.mu.Lock()
		cost := r.TotalCostUSD - s.lastCost
		s.lastCost = r.TotalCostUSD
		s.mu.Unlock()
		return Response{Text: r.Result, Usage: &Usage{
			InputTokens:  r.Usage.InputTokens,
			OutputTokens: r.Usage.OutputTokens,
			CacheRead:    r.Usage.CacheReadInputTokens,
			CacheWrite:   r.Usage.CacheCreationInputTokens,
			CostUSD:      cost,
		}}, nil
	case <-tctx.Done():
		s.kill()
		return Response{}, fmt.Errorf("session turn: %w", tctx.Err())
	}
}

// Asker adapts the session to the plain Asker function type so
// debuglog.WrapAsker and GetDecision compose unchanged.
func (s *Session) Asker() Asker { return s.Ask }

func (s *Session) Alive() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.dead
}

func (s *Session) Primed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.primed
}

func (s *Session) MarkPrimed() {
	s.mu.Lock()
	s.primed = true
	s.mu.Unlock()
}

func (s *Session) kill() {
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return
	}
	s.dead = true
	s.mu.Unlock()
	_ = s.stdin.Close()
	_ = s.cmd.Process.Kill()
	go func() { _ = s.cmd.Wait() }() // reap; never block a decision on it
}

// Close ends the session gracefully: stdin EOF lets the CLI exit on its
// own, with a kill safety net. Nil-safe.
func (s *Session) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return
	}
	s.dead = true
	s.mu.Unlock()
	_ = s.stdin.Close()
	done := make(chan struct{})
	go func() { _ = s.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -run 'TestSession' -v`
Expected: PASS (5 tests; TestHelperFakeClaude self-skips).

- [ ] **Step 5: Full package + commit**

Run: `go test ./internal/agent/`
Expected: PASS.

```bash
gofmt -w internal/agent/session.go internal/agent/session_test.go
git add internal/agent/session.go internal/agent/session_test.go
git commit -m "feat: stream-json Session with per-turn usage and turn timeout"
```

---

### Task 2: Adapter session capability (claude only)

**Files:**
- Modify: `internal/agent/adapter.go`
- Modify: `internal/agent/adapter_test.go` (append)

**Interfaces:**
- Consumes: `StartSession` (Task 1), `claudeSystemPrompt` (adapter.go:29).
- Produces (used by Task 4):
  - `Adapter.SessionArgs func(model string) []string` — nil = no session support
  - `func (a Adapter) SupportsSession() bool`
  - `func (a Adapter) StartSession(dir string, timeout time.Duration) (*Session, error)` — errors if unsupported

- [ ] **Step 1: Write the failing tests**

Append to `internal/agent/adapter_test.go`:

```go
func TestClaudeSessionArgs(t *testing.T) {
	var claude Adapter
	for _, a := range knownAdapters {
		if a.Key == "claude" {
			claude = a
		} else if a.SessionArgs != nil {
			t.Errorf("%s must not support sessions", a.Key)
		}
	}
	if !claude.SupportsSession() {
		t.Fatal("claude must support sessions")
	}
	joined := strings.Join(claude.SessionArgs("sonnet"), " ")
	for _, w := range []string{
		"-p", "--verbose",
		"--input-format stream-json", "--output-format stream-json",
		"--disallowedTools *", "--system-prompt", "--model sonnet",
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("session args missing %q in %q", w, joined)
		}
	}
	if strings.Contains(strings.Join(claude.SessionArgs(""), " "), "--model") {
		t.Error("empty model must not add --model")
	}
}

func TestStartSessionUnsupportedAdapter(t *testing.T) {
	a := Adapter{Key: "codex", Bin: "true"}
	if _, err := a.StartSession(t.TempDir(), time.Second); err == nil {
		t.Error("StartSession on session-less adapter must error")
	}
}
```

(Note: `strings` and `time` are already imported by adapter_test.go; if not, add them.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run 'TestClaudeSessionArgs|TestStartSessionUnsupported' -v`
Expected: compile error — `SessionArgs` field undefined.

- [ ] **Step 3: Implement**

In `internal/agent/adapter.go`, add to the `Adapter` struct (after `Env []string`):

```go
	// SessionArgs builds the CLI args for a persistent stream-json session
	// (P2). nil = adapter is stateless-only.
	SessionArgs func(model string) []string
```

Add to the claude entry in `knownAdapters` (after `Env: ...`):

```go
		SessionArgs: func(m string) []string {
			args := []string{"-p", "--verbose",
				"--input-format", "stream-json", "--output-format", "stream-json",
				"--disallowedTools", "*",
				"--system-prompt", claudeSystemPrompt}
			if m != "" {
				args = append(args, "--model", m)
			}
			return args
		},
```

Add methods at the bottom of adapter.go:

```go
func (a Adapter) SupportsSession() bool { return a.SessionArgs != nil }

// StartSession launches a persistent session for this adapter with
// cwd=dir (ADR-0002) and the adapter's Env, mirroring Asker.
func (a Adapter) StartSession(dir string, timeout time.Duration) (*Session, error) {
	if a.SessionArgs == nil {
		return nil, fmt.Errorf("adapter %s does not support sessions", a.Key)
	}
	return StartSession(a.Bin, a.SessionArgs(a.Model), a.Env, dir, timeout)
}
```

(`fmt` needs adding to adapter.go imports.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/agent/adapter.go internal/agent/adapter_test.go
git add internal/agent/adapter.go internal/agent/adapter_test.go
git commit -m "feat: claude adapter session capability (SessionArgs, StartSession)"
```

---

### Task 3: Delta prompt + prompt-explicit decision/reaction seams

**Files:**
- Modify: `internal/agent/prompt.go` (append `RenderDelta`)
- Modify: `internal/agent/decide.go` (refactor seams)
- Modify: `internal/agent/prompt_test.go`, `internal/agent/decide_test.go` (append)

**Interfaces:**
- Consumes: `RequestData`, `RenderPrompt` (prompt.go), `GetDecision`/`GetReaction` internals (decide.go).
- Produces (used by Task 4):
  - `func RenderDelta(d RequestData, handResults []string) string`
  - `func GetDecisionPrompt(ctx context.Context, ask Asker, prompt string, data RequestData, legal []poker.ActionType) (poker.Action, string, bool, *Usage)`
  - `func GetReactionPrompt(ctx context.Context, ask Asker, prompt string) (string, *Usage)`
  - `GetDecision` / `GetReaction` keep their exact current signatures and behavior (they delegate).

- [ ] **Step 1: Write the failing tests**

Append to `internal/agent/prompt_test.go`:

```go
func TestRenderDelta(t *testing.T) {
	d := RequestData{
		HandState:    "STATE-MARKER",
		TalkLog:      "HUMAN: gl",
		LegalActions: "check, bet",
		MinRaise:     40, MaxAmount: 990,
	}
	out := RenderDelta(d, []string{"Hand 3: you won 220 (the human folded)."})
	for _, want := range []string{
		"Hand 3: you won 220",
		"=== CURRENT HAND ===", "STATE-MARKER",
		"=== TABLE TALK THIS HAND ===", "HUMAN: gl",
		"Legal actions: check, bet", "40", "990",
		"ONLY the JSON object",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("delta missing %q:\n%s", want, out)
		}
	}
	// no hand-result lines when none pending
	if strings.Contains(RenderDelta(d, nil), "Hand 3") {
		t.Error("delta with nil results must not carry hand lines")
	}
	// delta must NOT re-send the big static blocks
	for _, absent := range []string{"=== YOUR TABLE PERSONA ===", "=== THE MATCH ===", "Format rules:"} {
		if strings.Contains(out, absent) {
			t.Errorf("delta must not contain %q", absent)
		}
	}
}
```

Append to `internal/agent/decide_test.go`:

```go
func TestGetDecisionPromptUsesGivenPrompt(t *testing.T) {
	var seen []string
	ask := func(ctx context.Context, p string) (Response, error) {
		seen = append(seen, p)
		return Response{Text: `{"action":"check"}`}, nil
	}
	data := RequestData{LegalActions: "check"}
	act, _, fb, _ := GetDecisionPrompt(context.Background(), ask, "DELTA-PROMPT", data,
		[]poker.ActionType{poker.Check})
	if fb || act.Type != poker.Check {
		t.Fatalf("decision failed: %v fb=%v", act, fb)
	}
	if len(seen) != 1 || seen[0] != "DELTA-PROMPT" {
		t.Errorf("ask saw %q, want the given prompt verbatim", seen)
	}
}

func TestGetReactionPromptUsesGivenPrompt(t *testing.T) {
	ask := func(ctx context.Context, p string) (Response, error) {
		if p != "REACT-PROMPT" {
			t.Errorf("prompt = %q", p)
		}
		return Response{Text: "one line\nsecond"}, nil
	}
	line, _ := GetReactionPrompt(context.Background(), ask, "REACT-PROMPT")
	if line != "one line" {
		t.Errorf("line = %q", line)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run 'TestRenderDelta|TestGetDecisionPrompt|TestGetReactionPrompt' -v`
Expected: compile error — functions undefined.

- [ ] **Step 3: Implement**

Append to `internal/agent/prompt.go`:

```go
// RenderDelta is the per-decision message for an already-primed session:
// only what changed since the last turn. handResults are digest summary
// lines for hands finished since the previous agent turn. The static
// blocks (persona, match intro, format rules) live in the session's
// priming turn and must not be repeated here — the conversation carries
// them, which is what keeps the prompt cache warm.
func RenderDelta(d RequestData, handResults []string) string {
	var b strings.Builder
	for _, r := range handResults {
		b.WriteString(r + "\n")
	}
	b.WriteString("\n=== CURRENT HAND ===\n" + d.HandState + "\n")
	b.WriteString("\n=== TABLE TALK THIS HAND ===\n" + d.TalkLog + "\n")
	fmt.Fprintf(&b, "\n=== YOUR MOVE ===\nLegal actions: %s\n", d.LegalActions)
	fmt.Fprintf(&b, "Reply with ONLY the JSON object — same format and rules as before. "+
		"Minimum raise-to %d, maximum %d (all-in).\n", d.MinRaise, d.MaxAmount)
	return b.String()
}
```

In `internal/agent/decide.go`, replace `GetDecision` with the pair (keep the doc comment, adjust):

```go
// GetDecision asks, retries once on unusable output, and falls back to
// check/fold so a match can never stall (ADR-0001). Returns
// (action, say, fallbackUsed, usage) — usage summed across attempts,
// nil when the adapter reports none.
func GetDecision(ctx context.Context, ask Asker, data RequestData, legal []poker.ActionType) (poker.Action, string, bool, *Usage) {
	return GetDecisionPrompt(ctx, ask, RenderPrompt(data), data, legal)
}

// GetDecisionPrompt is GetDecision with the prompt supplied by the caller —
// the persistent-session path (P2) sends a delta instead of the full render.
func GetDecisionPrompt(ctx context.Context, ask Asker, prompt string, data RequestData, legal []poker.ActionType) (poker.Action, string, bool, *Usage) {
	var used *Usage
	for attempt := 0; attempt < 2; attempt++ {
		p := prompt
		if attempt == 1 {
			p += retryReminder
		}
		resp, err := ask(ctx, p)
		if resp.Usage != nil {
			if used == nil {
				used = &Usage{}
			}
			used.Add(resp.Usage)
		}
		if err != nil {
			break // exec error / timeout: no retry, straight to fallback
		}
		d, err := ExtractDecision(resp.Text)
		if err != nil {
			continue
		}
		a, err := ValidateDecision(d, legal, data.MinRaise, data.MaxAmount)
		if err != nil {
			continue
		}
		return a, d.Say, false, used
	}
	return FallbackAction(legal), "", true, used
}
```

Replace `GetReaction` with the pair:

```go
func GetReaction(ctx context.Context, ask Asker, agentName, personality, digest, summary string) (string, *Usage) {
	return GetReactionPrompt(ctx, ask, ReactionPrompt(agentName, personality, digest, summary))
}

// GetReactionPrompt is GetReaction with a caller-built prompt — the session
// path sends a short in-conversation line instead of the full standalone
// reaction prompt.
func GetReactionPrompt(ctx context.Context, ask Asker, prompt string) (string, *Usage) {
	resp, err := ask(ctx, prompt)
	if err != nil {
		return "", resp.Usage
	}
	line := strings.TrimSpace(resp.Text)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if r := []rune(line); len(r) > 120 {
		line = string(r[:120])
	}
	return line, resp.Usage
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/`
Expected: PASS (all existing GetDecision/GetReaction tests must pass unchanged — they exercise the delegating wrappers).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/agent/prompt.go internal/agent/decide.go internal/agent/prompt_test.go internal/agent/decide_test.go
git add internal/agent/prompt.go internal/agent/decide.go internal/agent/prompt_test.go internal/agent/decide_test.go
git commit -m "feat: delta prompt render and prompt-explicit decision/reaction seams"
```

---

### Task 4: TUI + main wiring (session lifecycle, fallback, reaction turn)

**Files:**
- Modify: `internal/tui/app.go` (Model fields ~line 48-78; `askAgentCmd` :137-155; `finishHand` :178; `settleAndNext` :235-249; add `ensureSession`/`CloseSession`)
- Modify: `main.go:79-83` (capture final model, close session)
- Modify: `internal/tui/app_test.go` (append)
- Modify: `README.md` (one sentence)

**Interfaces:**
- Consumes: `agent.Session` (`Alive/Primed/MarkPrimed/Asker/Close`), `Adapter.SupportsSession/StartSession(dir, timeout)`, `agent.RenderDelta(data, results)`, `agent.RenderPrompt(data)`, `agent.GetDecisionPrompt(ctx, ask, prompt, data, legal)`, `agent.GetReactionPrompt(ctx, ask, prompt)`, `agent.ReactionPrompt(...)` (all from Tasks 1-3).
- Produces: `func (m Model) CloseSession()` (called by main).

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/app_test.go`:

```go
// The stub adapter has no SessionArgs, so the session path must stay
// completely inert: no session created, decisions still work (covered by
// existing tests), CloseSession safe.
func TestNoSessionForStatelessAdapter(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	if m.session != nil {
		t.Error("stateless adapter must not get a session")
	}
	m.CloseSession() // nil session: must not panic
}

func TestPendingHandResultsAccumulateAndCarrySummaries(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // human folds, hand ends
	m = m2.(Model)
	if len(m.pendingResults) != 1 || !strings.Contains(m.pendingResults[0], "Hand 1:") {
		t.Errorf("pendingResults = %v, want the hand 1 summary", m.pendingResults)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestNoSessionForStateless|TestPendingHandResults' -v`
Expected: compile error — `session`/`pendingResults`/`CloseSession` undefined.

- [ ] **Step 3: Implement Model changes**

In `internal/tui/app.go`, add to the `Model` struct (after `finalHumanSeat int`):

```go
	// P2 persistent session (claude only; nil = stateless adapter or start
	// failed). pendingResults are digest hand-summary lines not yet conveyed
	// to the session — the next delta turn carries and clears them.
	session        *agent.Session
	pendingResults []string
```

Add after `askAgentCmd`:

```go
// ensureSession returns a live session, lazily (re)starting one for
// session-capable adapters. One start attempt per call; on failure the
// caller proceeds stateless and a later decision retries (spec: failure &
// restart policy).
func (m *Model) ensureSession() *agent.Session {
	if m.session.Alive() {
		return m.session
	}
	if !m.opp.SupportsSession() {
		return nil
	}
	s, err := m.opp.StartSession(m.dir, decisionTimeout)
	if err != nil {
		m.log.Log("session_start_failed", map[string]any{"error": err.Error()})
		m.session = nil
		return nil
	}
	m.log.Log("session_start", map[string]any{"restart": m.session != nil})
	m.session = s
	return s
}

// CloseSession releases the opponent process; main calls it after the tea
// program exits. Nil-safe.
func (m Model) CloseSession() { m.session.Close() }
```

Replace `askAgentCmd` (currently app.go:137-155) with:

```go
func (m *Model) askAgentCmd() tea.Cmd {
	sb, bb := m.match.Blinds()
	data := agent.RequestData{
		AgentName:    m.opp.DisplayName,
		Personality:  m.personality,
		MatchDigest:  m.digest.String(),
		HandState:    agent.BuildHandState(m.hand, m.agentSeat(), sb, bb),
		TalkLog:      m.digest.HandTalk(),
		LegalActions: joinActions(m.hand.LegalActions()),
		MinRaise:     m.hand.MinRaiseTo(),
		MaxAmount:    m.hand.MaxRaiseTo(),
	}
	legal := m.hand.LegalActions()
	full := agent.RenderPrompt(data)
	prompt := full
	ask := m.opp.Asker(m.dir, decisionTimeout)
	if s := m.ensureSession(); s != nil {
		if s.Primed() {
			prompt = agent.RenderDelta(data, m.pendingResults)
		} else {
			// priming turn: the full render (with digest) IS the catch-up
			s.MarkPrimed()
		}
		m.pendingResults = nil
		stateless := ask
		ask = func(ctx context.Context, p string) (agent.Response, error) {
			if !s.Alive() {
				return stateless(ctx, full)
			}
			resp, err := s.Ask(ctx, p)
			if err != nil {
				// session died mid-decision: same decision continues via a
				// cold spawn with the full prompt (ADR-0001 — never stall).
				return stateless(ctx, full)
			}
			return resp, nil
		}
	}
	wrapped := debuglog.WrapAsker(ask, m.log)
	return func() tea.Msg {
		act, say, fb, u := agent.GetDecisionPrompt(context.Background(), wrapped, prompt, data, legal)
		return decisionMsg{act: act, say: say, fallback: fb, usage: u}
	}
}
```

In `finishHand`, right after the existing `m.digest.EndHand(...)` line (app.go:178), add:

```go
	m.pendingResults = append(m.pendingResults, agentHandSummary(m.match.HandNum, r, r.Winner == m.agentSeat()))
```

(Note: `agentHandSummary` is called twice now — once for the digest, once for pendingResults. Assign it to a local `summary := agentHandSummary(...)` and use it for both `m.digest.EndHand(summary)` and the append.)

In `settleAndNext`, replace the `if !m.quiet { ... }` reaction block (app.go:235-248) with:

```go
		if !m.quiet {
			outcome := "MATCH OVER: you LOST the match to your human. They took every chip."
			if m.match.Winner() == 1 {
				outcome = "MATCH OVER: you WON the match. Your human is busted."
			}
			name, digest, persona := m.opp.DisplayName, m.digest.String(), m.personality
			fullPrompt := agent.ReactionPrompt(name, persona, digest, outcome)
			prompt := fullPrompt
			ask := m.opp.Asker(m.dir, decisionTimeout)
			if s := m.session; s.Alive() && s.Primed() {
				// the session already knows the match; one short turn does it
				prompt = outcome + "\nReact in ONE short line — gloat, whine, needle, whatever fits. Plain text only, no JSON, no quotes, one line."
				stateless := ask
				ask = func(ctx context.Context, p string) (agent.Response, error) {
					resp, err := s.Ask(ctx, p)
					if err != nil {
						return stateless(ctx, fullPrompt)
					}
					return resp, nil
				}
			}
			wrapped := debuglog.WrapAsker(ask, m.log)
			// Late usage from this call is folded into stats by the
			// reactionMsg handler (post-save re-save path).
			return func() tea.Msg {
				say, u := agent.GetReactionPrompt(context.Background(), wrapped, prompt)
				return reactionMsg{say: say, usage: u}
			}
		}
```

- [ ] **Step 4: Wire main.go**

In `main.go`, the program-run block (currently `_, runErr := tea.NewProgram(m, tea.WithAltScreen()).Run()`) becomes:

```go
	final, runErr := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if fm, ok := final.(tui.Model); ok {
		fm.CloseSession() // release the persistent opponent process, if any
	}
```

- [ ] **Step 5: README sentence**

In `README.md`, after the sentence about match cost on the match-over screen, add:

```
Claude opponents run as one persistent session per match (prompt-cache warm: ~10x cheaper and faster decisions than one-shot spawns); codex/gemini spawn per decision.
```

- [ ] **Step 6: Run the full suite + vet**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all packages PASS. Existing TUI tests (stub adapter, no SessionArgs) must pass unchanged — the session path is inert for them.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/tui/app.go internal/tui/app_test.go main.go
git add internal/tui/app.go internal/tui/app_test.go main.go README.md
git commit -m "feat: persistent session wiring — lazy start, delta turns, session reaction"
```

---

## Manual verification (after all tasks; per debug-transcript protocol)

`go run . --debug`, claude/sonnet/needler, play 3+ hands, quit at match end. In the transcript expect:
- `session_start` event once (no `session_start_failed`)
- `agent_call` #1: `cache_write` large, `cache_read` 0 (priming)
- `agent_call` #2+: `cache_read > 0`, cost ~$0.001-0.003, prompt field is the short delta
- match-end reaction present, its `agent_call` on the session (short prompt)
- decisions noticeably snappier after the first
