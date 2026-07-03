# Debug Transcript Log Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Opt-in `--debug` flag that writes a JSONL transcript (agent prompts/replies, engine actions and state, human keys) to `~/.showdown/debug-<timestamp>.jsonl` so one reproduction of a bug yields complete evidence.

**Architecture:** New `internal/debuglog` package with a nil-safe `Logger` (nil = no-op, so call sites carry no conditionals) and a `WrapAsker` decorator for agent I/O. `main.go` creates the logger and passes it into `tui.NewModel`; all TUI/engine hooks live in `internal/tui/app.go`, where every `Hand.Apply` call is funneled through one `m.apply` helper.

**Tech Stack:** Go stdlib only (`encoding/json`, `os`, `sync`, `time`). No new dependencies.

Spec: `docs/superpowers/specs/2026-07-03-debug-transcript-log-design.md`

## Global Constraints

- Logging must never prevent or crash play: creation failure → warn on stderr and run without logging; write/marshal errors dropped silently.
- A nil `*debuglog.Logger` is a valid no-op for every method and for `WrapAsker`.
- Log file: `~/.showdown/debug-<YYYYMMDD-HHMMSS>.jsonl`, one JSON object per line, each line `{"seq":N,"t":"<RFC3339Nano>","event":"<name>", ...fields}` with `seq` monotonically increasing from 1.
- Activation: `--debug` flag OR `SHOWDOWN_DEBUG=1` env var.
- No new module dependencies.

---

### Task 1: `internal/debuglog` — Logger core

**Files:**
- Create: `internal/debuglog/debuglog.go`
- Test: `internal/debuglog/debuglog_test.go`

**Interfaces:**
- Consumes: nothing (stdlib only).
- Produces:
  - `func New(path string) (*Logger, error)`
  - `func (l *Logger) Log(event string, fields map[string]any)` — nil-safe
  - `func (l *Logger) Close() error` — nil-safe
  - `func DefaultPath(now time.Time) string` — returns `~/.showdown/debug-<YYYYMMDD-HHMMSS>.jsonl`

- [ ] **Step 1: Write the failing tests**

Create `internal/debuglog/debuglog_test.go`:

```go
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
	l.Log("late", nil)         // must not panic
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/debuglog/`
Expected: FAIL — package does not exist / `New` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/debuglog/debuglog.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/debuglog/`
Expected: PASS (all 5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/debuglog/
git commit -m "feat: add debuglog JSONL logger (nil-safe, best-effort)"
```

---

### Task 2: `WrapAsker` — agent I/O logging decorator

**Files:**
- Create: `internal/debuglog/asker.go`
- Test: `internal/debuglog/asker_test.go`

**Interfaces:**
- Consumes: `agent.Asker` = `func(ctx context.Context, prompt string) (string, error)` (existing, `internal/agent/decide.go:18`); `*Logger` from Task 1.
- Produces: `func WrapAsker(ask agent.Asker, l *Logger) agent.Asker` — logs event `agent_call` with fields `prompt`, `raw`, `duration_ms`, and `error` (only on error); passes results through unchanged; nil logger returns `ask` as-is.

Note: `debuglog` imports `agent`; `agent` must NOT import `debuglog` (no cycle).

- [ ] **Step 1: Write the failing tests**

Create `internal/debuglog/asker_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/debuglog/`
Expected: FAIL — `WrapAsker` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/debuglog/asker.go`:

```go
package debuglog

import (
	"context"
	"time"

	"github.com/haohanwu/showdown/internal/agent"
)

// WrapAsker decorates ask so every agent CLI call is logged (event
// "agent_call") with the full prompt, raw output, error, and duration.
// Results pass through unchanged. A nil logger returns ask as-is.
func WrapAsker(ask agent.Asker, l *Logger) agent.Asker {
	if l == nil {
		return ask
	}
	return func(ctx context.Context, prompt string) (string, error) {
		start := time.Now()
		raw, err := ask(ctx, prompt)
		f := map[string]any{
			"prompt":      prompt,
			"raw":         raw,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err != nil {
			f["error"] = err.Error()
		}
		l.Log("agent_call", f)
		return raw, err
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/debuglog/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debuglog/asker.go internal/debuglog/asker_test.go
git commit -m "feat: WrapAsker decorator logs agent prompts and raw replies"
```

---

### Task 3: Plumbing — `--debug` flag, NewModel signature, `session_start`

**Files:**
- Modify: `main.go`
- Modify: `internal/tui/app.go` (`Model` struct ~line 43, `NewModel` ~line 70)
- Modify: `internal/tui/app_test.go` (two `NewModel` call sites, lines 17 and 170)
- Modify: `README.md` (flags paragraph)
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `debuglog.New`, `debuglog.DefaultPath`, `*debuglog.Logger` (Task 1).
- Produces:
  - `tui.NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir string, log *debuglog.Logger) Model` — NEW final param; nil disables logging.
  - `Model.log *debuglog.Logger` field, used by Task 4 hooks.
  - Event `session_start` fields: `agent_key`, `agent_name`, `quiet`, `dir`, `seed`.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/app_test.go` (also add `"github.com/haohanwu/showdown/internal/debuglog"`, `"bufio"`, `"encoding/json"`, `"os"`, `"path/filepath"` to imports):

```go
func readLogEvents(t *testing.T, path string) []map[string]any {
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
			t.Fatalf("invalid JSONL: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func TestSessionStartLogged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := debuglog.New(path)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", l)
	l.Close()

	evs := readLogEvents(t, path)
	if len(evs) != 1 || evs[0]["event"] != "session_start" {
		t.Fatalf("events = %v, want one session_start", evs)
	}
	if evs[0]["agent_key"] != "stub" || evs[0]["quiet"] != true {
		t.Errorf("session_start fields = %v", evs[0])
	}
	if _, ok := evs[0]["seed"]; !ok {
		t.Error("session_start missing seed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestSessionStartLogged`
Expected: FAIL to compile — `NewModel` takes 5 args, test passes 6.

- [ ] **Step 3: Change `NewModel` and `Model`**

In `internal/tui/app.go`, add import `"github.com/haohanwu/showdown/internal/debuglog"`.

Add field to `Model` struct (after `rng *rand.Rand`):

```go
	log *debuglog.Logger
```

Replace `NewModel`:

```go
func NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir string, log *debuglog.Logger) Model {
	in := textinput.New()
	in.CharLimit = 120
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	seed := time.Now().UnixNano()
	log.Log("session_start", map[string]any{
		"agent_key": opp.Key, "agent_name": opp.DisplayName,
		"quiet": quiet, "dir": dir, "seed": seed,
	})
	return Model{
		opp: opp, stats: st, statsPath: statsPath, quiet: quiet, dir: dir, log: log,
		rng:   rand.New(rand.NewSource(seed)),
		match: poker.NewMatch(), digest: agent.NewDigest(),
		input: in, spin: sp,
	}
}
```

Fix the two existing call sites in `internal/tui/app_test.go` by appending `nil`:

```go
	return NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", nil)
```

```go
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", nil)
```

- [ ] **Step 4: Wire up `main.go`**

Replace `main()` in `main.go` (imports gain `"time"` and `"github.com/haohanwu/showdown/internal/debuglog"`):

```go
func main() {
	agentFlag := flag.String("agent", "", "opponent agent key (claude, codex, gemini)")
	quiet := flag.Bool("quiet", false, "disable table talk")
	debug := flag.Bool("debug", false, "write a JSONL debug transcript to ~/.showdown/")
	flag.Parse()

	var dlog *debuglog.Logger
	var dlogPath string
	if *debug || os.Getenv("SHOWDOWN_DEBUG") == "1" {
		dlogPath = debuglog.DefaultPath(time.Now())
		l, err := debuglog.New(dlogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "debug log disabled: %v\n", err)
		} else {
			dlog = l
		}
	}

	st, err := stats.Load(stats.DefaultPath())
	if err != nil {
		st = stats.Stats{}
	}
	roster := agent.DetectRoster()

	var opp agent.Adapter
	if *agentFlag != "" {
		if len(roster) == 0 {
			fmt.Fprintln(os.Stderr, "no agent CLIs found on PATH (looked for: claude, codex, gemini)")
			os.Exit(1)
		}
		for _, a := range roster {
			if a.Key == *agentFlag {
				opp = a
			}
		}
		if opp.Key == "" {
			fmt.Fprintf(os.Stderr, "agent %q not found (have: %v)\n", *agentFlag, keys(roster))
			os.Exit(1)
		}
	} else {
		opp, err = tui.RunPicker(roster, st)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	dir, _ := os.Getwd()
	m := tui.NewModel(opp, st, stats.DefaultPath(), *quiet, dir, dlog)
	_, runErr := tea.NewProgram(m, tea.WithAltScreen()).Run()
	_ = dlog.Close()
	if dlog != nil {
		fmt.Fprintln(os.Stderr, "debug log:", dlogPath)
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}
```

(The alt screen hides stderr during play; the path prints after the TUI exits. Close before printing so the file is complete when the user copies it.)

- [ ] **Step 5: Update README**

In `README.md`, extend the flags sentence in the Play section:

```markdown
Detects `claude`, `codex`, and `gemini` on your PATH. Multiple found →
you choose your opponent. Force one with `--agent codex`. Silence the
table talk with `--quiet`. Debugging something? `--debug` (or
`SHOWDOWN_DEBUG=1`) writes a full JSONL transcript — prompts, replies,
every action — to `~/.showdown/`, path printed on exit.
```

- [ ] **Step 6: Run full test suite and build**

Run: `go build ./... && go test ./...`
Expected: build OK, all packages PASS (including `TestSessionStartLogged`).

- [ ] **Step 7: Commit**

```bash
git add main.go internal/tui/app.go internal/tui/app_test.go README.md
git commit -m "feat: --debug flag creates transcript logger, threads into TUI"
```

---

### Task 4: TUI hooks — apply funnel, hand/decision/key/end events

**Files:**
- Modify: `internal/tui/app.go` (`startHand`, `Update` decisionMsg case, `handleKey`, `humanAction`, `confirmInput`, `finishHand`; new helpers `apply`, `cardStrings`, `phaseName`)
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `Model.log` field and events from Task 3; existing `m.hand.Apply` semantics (`internal/poker/hand.go:163`).
- Produces (log events only, no new exported API):
  - `hand_start`: `hand`, `sb`, `bb`, `human_seat`, `hole_seat0`, `hole_seat1`, `stacks`
  - `agent_decision`: `action`, `to`, `say`, `fallback`
  - `apply`: `hand`, `actor_seat`, `action`, `to`, `street`, `pot`, `stacks`, `board`, `error` (only on error)
  - `human_key`: `key`, `phase`
  - `hand_end`: `hand`, `winner_seat`, `pot`, `showdown`, `split`, `desc`, `stacks`
- Internal helper: `func (m *Model) apply(a poker.Action) error` — ALL seven existing `m.hand.Apply(...)` call sites in app.go switch to `m.apply(...)`.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/app_test.go`:

```go
func TestDebugLogCapturesHandFlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := debuglog.New(path)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", l)

	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // human folds hand 1
	m = m2.(Model)
	l.Close()

	byEvent := map[string][]map[string]any{}
	for _, e := range readLogEvents(t, path) {
		name := e["event"].(string)
		byEvent[name] = append(byEvent[name], e)
	}

	if n := len(byEvent["hand_start"]); n != 1 {
		t.Fatalf("hand_start count = %d, want 1", n)
	}
	hs := byEvent["hand_start"][0]
	if hs["hand"] != float64(1) || hs["sb"] != float64(10) || hs["bb"] != float64(20) {
		t.Errorf("hand_start = %v", hs)
	}
	for _, k := range []string{"hole_seat0", "hole_seat1", "stacks", "human_seat"} {
		if _, ok := hs[k]; !ok {
			t.Errorf("hand_start missing %q", k)
		}
	}

	if n := len(byEvent["human_key"]); n != 1 {
		t.Fatalf("human_key count = %d, want 1", n)
	}
	hk := byEvent["human_key"][0]
	if hk["key"] != "f" || hk["phase"] != "human_turn" {
		t.Errorf("human_key = %v", hk)
	}

	if n := len(byEvent["apply"]); n != 1 {
		t.Fatalf("apply count = %d, want 1", n)
	}
	ap := byEvent["apply"][0]
	if ap["action"] != "fold" || ap["actor_seat"] != float64(0) {
		t.Errorf("apply = %v", ap)
	}
	for _, k := range []string{"street", "pot", "stacks", "board"} {
		if _, ok := ap[k]; !ok {
			t.Errorf("apply missing %q", k)
		}
	}
	if _, ok := ap["error"]; ok {
		t.Error("apply has error field on legal action")
	}

	if n := len(byEvent["hand_end"]); n != 1 {
		t.Fatalf("hand_end count = %d, want 1", n)
	}
	he := byEvent["hand_end"][0]
	if he["winner_seat"] != float64(1) || he["showdown"] != false {
		t.Errorf("hand_end = %v", he)
	}
}

func TestAgentDecisionLogged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := debuglog.New(path)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", l)

	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("c")) // human limps; agent's turn
	m = m2.(Model)
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}, say: "hm", fallback: true})
	m = m2.(Model)
	l.Close()

	var dec map[string]any
	for _, e := range readLogEvents(t, path) {
		if e["event"] == "agent_decision" {
			dec = e
		}
	}
	if dec == nil {
		t.Fatal("no agent_decision event logged")
	}
	if dec["action"] != "check" || dec["say"] != "hm" || dec["fallback"] != true {
		t.Errorf("agent_decision = %v", dec)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestDebugLogCapturesHandFlow|TestAgentDecisionLogged'`
Expected: FAIL — no events beyond `session_start` are written (counts = 0).

- [ ] **Step 3: Add helpers to `internal/tui/app.go`**

Add near `joinActions` (bottom of file, before styles):

```go
func cardStrings(cs []poker.Card) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.String()
	}
	return out
}

func phaseName(p phase) string {
	switch p {
	case phaseHumanTurn:
		return "human_turn"
	case phaseRaiseInput:
		return "raise_input"
	case phaseTalkInput:
		return "talk_input"
	case phaseAgentTurn:
		return "agent_turn"
	case phaseRunout:
		return "runout"
	case phaseHandEnd:
		return "hand_end"
	case phaseMatchOver:
		return "match_over"
	}
	return "unknown"
}

// apply funnels every Hand.Apply so the debug log records each action and
// the resulting engine state. Behavior is identical to calling
// m.hand.Apply directly.
func (m *Model) apply(a poker.Action) error {
	actor := m.hand.Actor
	err := m.hand.Apply(a)
	f := map[string]any{
		"hand": m.match.HandNum, "actor_seat": actor,
		"action": string(a.Type), "to": a.To,
		"street": m.hand.Street.String(),
		"pot":    m.hand.Pot,
		"stacks": []int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack},
		"board":  cardStrings(m.hand.Board),
	}
	if err != nil {
		f["error"] = err.Error()
	}
	m.log.Log("apply", f)
	return err
}
```

- [ ] **Step 4: Hook `startHand`, `finishHand`, `Update`, `handleKey`; swap Apply call sites**

In `startHand`, after `m.hand = poker.NewHand(...)`:

```go
	m.log.Log("hand_start", map[string]any{
		"hand": m.match.HandNum, "sb": sb, "bb": bb,
		"human_seat": m.humanSeat(),
		"hole_seat0": cardStrings(m.hand.Hole[0][:]),
		"hole_seat1": cardStrings(m.hand.Hole[1][:]),
		"stacks":     []int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack},
	})
```

In `finishHand`, after `r := m.hand.Result()`:

```go
	m.log.Log("hand_end", map[string]any{
		"hand": m.match.HandNum, "winner_seat": r.Winner,
		"pot": r.Pot, "showdown": r.Showdown, "split": r.Split, "desc": r.Desc,
		"stacks": []int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack},
	})
```

In `Update`, replace the `decisionMsg` case body's first lines:

```go
	case decisionMsg:
		m.log.Log("agent_decision", map[string]any{
			"action": string(msg.act.Type), "to": msg.act.To,
			"say": msg.say, "fallback": msg.fallback,
		})
		if err := m.apply(msg.act); err != nil {
			// GetDecision guarantees legality; a failure here is a bug — force fallback.
			_ = m.apply(agent.FallbackAction(m.hand.LegalActions()))
		}
```

In `handleKey`, after `k := msg.String()`:

```go
	m.log.Log("human_key", map[string]any{"key": k, "phase": phaseName(m.phase)})
```

Swap the remaining direct `m.hand.Apply` calls (all in `humanAction` and `confirmInput`):

- `_ = m.hand.Apply(poker.Action{Type: poker.Fold})` → `_ = m.apply(poker.Action{Type: poker.Fold})`
- `_ = m.hand.Apply(poker.Action{Type: poker.Call})` → `_ = m.apply(poker.Action{Type: poker.Call})`
- `_ = m.hand.Apply(poker.Action{Type: poker.Check})` → `_ = m.apply(poker.Action{Type: poker.Check})`
- `_ = m.hand.Apply(poker.Action{Type: t, To: m.hand.MaxRaiseTo()})` → `_ = m.apply(poker.Action{Type: t, To: m.hand.MaxRaiseTo()})`
- `if err := m.hand.Apply(poker.Action{Type: t, To: n}); err != nil {` → `if err := m.apply(poker.Action{Type: t, To: n}); err != nil {`

After this step `grep -n 'm\.hand\.Apply' internal/tui/app.go` must return nothing.

- [ ] **Step 5: Wrap the askers**

In `askAgentCmd`, change:

```go
	ask := m.opp.Asker(m.dir, decisionTimeout)
```

to:

```go
	ask := debuglog.WrapAsker(m.opp.Asker(m.dir, decisionTimeout), m.log)
```

Same one-line change in `finishHand` (the reaction asker).

- [ ] **Step 6: Run full test suite**

Run: `go build ./... && go test ./...`
Expected: PASS — new tests plus all existing tests (nil logger keeps old behavior; `m.apply` is behavior-identical to direct `Apply`).

- [ ] **Step 7: Manual smoke test**

Run: `SHOWDOWN_AGENT_CMD="./scripts/stub-agent.sh" go run . --debug --quiet`. Fold one hand, quit with `q`.
Expected: stderr prints `debug log: ~/.showdown/debug-....jsonl`; file contains `session_start`, `hand_start`, `human_key`, `apply`, `hand_end` lines (and `agent_call`/`agent_decision` if the agent acted).

- [ ] **Step 8: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: log hand/decision/key/apply events to debug transcript"
```
