# Token/Cost Measurement + Model Choice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Measure real token usage and dollar cost per agent call (P0), and let the player pick which model the opponent agent runs (P1).

**Architecture:** The claude CLI is switched to `claude -p <prompt> --output-format json`, whose envelope carries real `usage` and `total_cost_usd`. A new `agent.Response{Text, Usage}` replaces the raw string an `Asker` returns; adapters that can't report usage (codex, gemini, custom) return `Usage == nil`. Usage flows three ways: into the debug log (`agent_call` event), into a per-session accumulator on the TUI `Model` (shown on the match-over screen), and into `stats.Record` (lifetime per-agent totals, shown in the picker). Model choice is a new `Model` field on `Adapter`, injected into CLI args, settable via a `--model` flag or a second picker prompt.

**Tech Stack:** Go, bubbletea TUI, `go test ./...`. No new dependencies.

## Verified facts (do not re-derive)

`claude --model haiku -p 'reply with the single word: ok' --output-format json` was run on 2026-07-03 and returned (abridged):

```json
{"type":"result","subtype":"success","is_error":false,"duration_ms":8951,
 "result":"ok","total_cost_usd":0.021552,
 "usage":{"input_tokens":10,"cache_creation_input_tokens":9039,
          "cache_read_input_tokens":10690,"output_tokens":479}}
```

- The model's text lives in top-level `result` (string). The decision JSON the game needs is *inside* that string.
- `--model` accepts aliases `haiku`, `sonnet`, `opus`.
- codex model flag: `codex exec -m <model> <prompt>`. gemini model flag: `gemini -m <model> -p <prompt>`. Neither gets JSON usage parsing in this plan (their envelopes are unverified) — they return `Usage == nil`.

## Global Constraints

- All tests green: `go test ./...` after every task.
- Formatting: `gofmt -l .` must print nothing before each commit.
- Commit style (from git history): `feat: …`, `fix: …`, `docs: …`, final graph commit `graph: incremental update for token-cost + model-choice features`.
- Match code style of the file being edited (small files, comments only for non-obvious constraints).
- ADR-0001 invariant must survive: a match can never stall — every parse failure falls back to retry → check/fold.
- Old `~/.showdown/stats.json` files (only `wins`/`losses`) must still load. New fields must default to zero.

---

### Task 1: `Usage`, `Response`, `ParseClaudeJSON`

**Files:**
- Create: `internal/agent/response.go`
- Test: `internal/agent/response_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `type Usage struct{ InputTokens, OutputTokens, CacheRead, CacheWrite int; CostUSD float64 }` with methods `(u *Usage) Add(v *Usage)` and `(u Usage) TotalIn() int`; `type Response struct{ Text string; Usage *Usage }`; `func ParseClaudeJSON(raw string) Response`. Later tasks use these exact names.

- [ ] **Step 1: Write the failing test**

```go
// internal/agent/response_test.go
package agent

import "testing"

const sampleEnvelope = `{"type":"result","subtype":"success","is_error":false,
"duration_ms":8951,"result":"{\"action\": \"call\"}","total_cost_usd":0.021552,
"usage":{"input_tokens":10,"cache_creation_input_tokens":9039,
"cache_read_input_tokens":10690,"output_tokens":479}}`

func TestParseClaudeJSON(t *testing.T) {
	r := ParseClaudeJSON(sampleEnvelope)
	if r.Text != `{"action": "call"}` {
		t.Errorf("Text = %q", r.Text)
	}
	if r.Usage == nil {
		t.Fatal("Usage = nil, want populated")
	}
	u := *r.Usage
	if u.InputTokens != 10 || u.OutputTokens != 479 || u.CacheRead != 10690 || u.CacheWrite != 9039 {
		t.Errorf("Usage = %+v", u)
	}
	if u.CostUSD != 0.021552 {
		t.Errorf("CostUSD = %v", u.CostUSD)
	}
	if u.TotalIn() != 10+10690+9039 {
		t.Errorf("TotalIn = %d", u.TotalIn())
	}
}

func TestParseClaudeJSONMalformedFallsBackToRaw(t *testing.T) {
	for _, raw := range []string{"plain text reply", `{"no_result_key":1}`, ""} {
		r := ParseClaudeJSON(raw)
		if r.Text != raw {
			t.Errorf("ParseClaudeJSON(%q).Text = %q, want raw passthrough", raw, r.Text)
		}
		if r.Usage != nil {
			t.Errorf("ParseClaudeJSON(%q).Usage != nil", raw)
		}
	}
}

func TestUsageAdd(t *testing.T) {
	u := Usage{InputTokens: 1, OutputTokens: 2, CacheRead: 3, CacheWrite: 4, CostUSD: 0.5}
	u.Add(&Usage{InputTokens: 10, OutputTokens: 20, CacheRead: 30, CacheWrite: 40, CostUSD: 0.25})
	want := Usage{InputTokens: 11, OutputTokens: 22, CacheRead: 33, CacheWrite: 44, CostUSD: 0.75}
	if u != want {
		t.Errorf("got %+v, want %+v", u, want)
	}
	u.Add(nil) // must be a no-op, not a panic
	if u != want {
		t.Errorf("Add(nil) changed value: %+v", u)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run 'TestParseClaudeJSON|TestUsageAdd' -v`
Expected: FAIL — `undefined: ParseClaudeJSON`, `undefined: Usage`

- [ ] **Step 3: Write the implementation**

```go
// internal/agent/response.go
package agent

import "encoding/json"

// Usage is token/cost accounting for one or more agent CLI calls.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheRead    int     `json:"cache_read_input_tokens"`
	CacheWrite   int     `json:"cache_creation_input_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// Add accumulates v into u; a nil v is a no-op.
func (u *Usage) Add(v *Usage) {
	if v == nil {
		return
	}
	u.InputTokens += v.InputTokens
	u.OutputTokens += v.OutputTokens
	u.CacheRead += v.CacheRead
	u.CacheWrite += v.CacheWrite
	u.CostUSD += v.CostUSD
}

// TotalIn is everything sent to the model: fresh input plus cache reads and
// cache writes. This is the honest "what went into this" number.
func (u Usage) TotalIn() int { return u.InputTokens + u.CacheRead + u.CacheWrite }

// Response is one agent CLI reply. Usage is nil when the CLI doesn't report it.
type Response struct {
	Text  string
	Usage *Usage
}

type claudeEnvelope struct {
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// ParseClaudeJSON unwraps the `claude -p --output-format json` envelope.
// Anything that doesn't look like the envelope (including an empty result,
// e.g. from a future CLI format drift) falls back to raw passthrough so a
// decision can still be extracted or retried (ADR-0001).
func ParseClaudeJSON(raw string) Response {
	var env claudeEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil || env.Result == "" {
		return Response{Text: raw}
	}
	return Response{Text: env.Result, Usage: &Usage{
		InputTokens:  env.Usage.InputTokens,
		OutputTokens: env.Usage.OutputTokens,
		CacheRead:    env.Usage.CacheReadInputTokens,
		CacheWrite:   env.Usage.CacheCreationInputTokens,
		CostUSD:      env.TotalCostUSD,
	}}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -run 'TestParseClaudeJSON|TestUsageAdd' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
gofmt -l . && git add internal/agent/response.go internal/agent/response_test.go
git commit -m "feat: Usage/Response types + claude JSON envelope parser"
```

---

### Task 2: `Asker` returns `Response`; claude adapter switches to JSON output

One atomic signature change: `Asker func(ctx, prompt) (string, error)` → `(Response, error)`. Everything that compiles against `Asker` updates in this task; behavior of codex/gemini/custom adapters is unchanged (plain text in `.Text`, nil `Usage`).

**Files:**
- Modify: `internal/agent/decide.go` (Asker type at line 18; `GetDecision` line 97; `GetReaction` line 134)
- Modify: `internal/agent/adapter.go` (Adapter struct line 12; `Asker` method line 29; claude entry in `knownAdapters` line 20)
- Modify: `internal/debuglog/asker.go` (WrapAsker)
- Test: `internal/agent/adapter_test.go`, `internal/agent/decide_test.go`, `internal/debuglog/asker_test.go`

**Interfaces:**
- Consumes: `Response`, `ParseClaudeJSON` from Task 1.
- Produces: `type Asker func(ctx context.Context, prompt string) (Response, error)`; `Adapter` gains field `Parse func(raw string) Response` (nil = plain text). Claude adapter args become `["-p", prompt, "--output-format", "json"]`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/agent/adapter_test.go`:

```go
func TestClaudeAdapterUsesJSONOutput(t *testing.T) {
	for _, a := range knownAdapters {
		if a.Key != "claude" {
			continue
		}
		args := strings.Join(a.Args("PROMPT"), " ")
		if !strings.Contains(args, "--output-format json") {
			t.Errorf("claude args = %q, want --output-format json", args)
		}
		if a.Parse == nil {
			t.Error("claude adapter must set Parse")
		}
		return
	}
	t.Fatal("no claude adapter in knownAdapters")
}

func TestAskerAppliesParse(t *testing.T) {
	a := Adapter{Key: "x", Bin: "echo",
		Args:  func(p string) []string { return []string{sampleEnvelope} },
		Parse: ParseClaudeJSON}
	resp, err := a.Asker(".", 10*time.Second)(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if resp.Text != `{"action": "call"}` {
		t.Errorf("Text = %q, want unwrapped result", resp.Text)
	}
	if resp.Usage == nil || resp.Usage.OutputTokens != 479 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}
```

(`sampleEnvelope` is the const from `response_test.go`, same package.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run 'TestClaudeAdapter|TestAskerAppliesParse' -v`
Expected: FAIL — `a.Parse undefined`, and/or `--output-format json` missing

- [ ] **Step 3: Implement**

`internal/agent/decide.go` — change the Asker type:

```go
type Asker func(ctx context.Context, prompt string) (Response, error)
```

In `GetDecision`, replace the ask block:

```go
		resp, err := ask(ctx, p)
		if err != nil {
			break // exec error / timeout: no retry, straight to fallback
		}
		d, err := ExtractDecision(resp.Text)
```

In `GetReaction`:

```go
	resp, err := ask(ctx, ReactionPrompt(agentName, digest, handSummary))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(resp.Text)
```

`internal/agent/adapter.go` — struct gains `Parse`, method applies it, claude entry updated:

```go
type Adapter struct {
	Key         string
	DisplayName string
	Bin         string
	Args        func(prompt string) []string
	Parse       func(raw string) Response // nil = plain-text output
}
```

```go
	{Key: "claude", DisplayName: "Claude Code", Bin: "claude",
		Args:  func(p string) []string { return []string{"-p", p, "--output-format", "json"} },
		Parse: ParseClaudeJSON},
```

`Asker` method tail:

```go
		out, err := cmd.Output()
		if err != nil {
			return Response{}, err
		}
		if a.Parse != nil {
			return a.Parse(string(out)), nil
		}
		return Response{Text: string(out)}, nil
```

`internal/debuglog/asker.go` — pass `Response` through, log `.Text` as `raw`:

```go
	return func(ctx context.Context, prompt string) (agent.Response, error) {
		start := time.Now()
		resp, err := ask(ctx, prompt)
		f := map[string]any{
			"prompt":      prompt,
			"raw":         resp.Text,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err != nil {
			f["error"] = err.Error()
		}
		l.Log("agent_call", f)
		return resp, err
	}
```

- [ ] **Step 4: Mechanically update existing tests to the new signature**

- `internal/agent/decide_test.go`: every fake asker `return "…", nil` → `return Response{Text: "…"}, nil`; `return "", errors.New("timeout")` → `return Response{}, errors.New("timeout")`.
- `internal/agent/adapter_test.go` `TestCustomAdapterViaEnv`: `out, err := ask(…)` → check `out.Text` contains `"action": "call"` and `out.Usage == nil` (custom adapters report no usage).
- `internal/debuglog/asker_test.go`: inner askers return `agent.Response{Text: "reply:" + prompt}`; assertions read `resp.Text`.

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS (all packages — tui compiles unchanged because `GetDecision`/`GetReaction` signatures haven't changed yet)

- [ ] **Step 6: Commit**

```bash
gofmt -l . && git add -A
git commit -m "feat: Asker returns Response; claude adapter uses --output-format json"
```

---

### Task 3: Usage flows to the TUI — `GetDecision`/`GetReaction` return it, `Model` accumulates it

**Files:**
- Modify: `internal/agent/decide.go` (`GetDecision`, `GetReaction`)
- Modify: `internal/tui/app.go` (`decisionMsg` line 36, `reactionMsg` line 41, `askAgentCmd` line 129, `finishHand` line 148, `Update` cases line 237/262, `Model` struct line 44)
- Test: `internal/agent/decide_test.go`, `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `Usage`, `Response` (Task 1), new `Asker` (Task 2).
- Produces: `GetDecision(ctx, ask, data, legal) (poker.Action, string, bool, *Usage)` — usage summed across the retry; `GetReaction(ctx, ask, name, digest, summary) (string, *Usage)`; TUI `Model` field `sessionUsage agent.Usage`; `decisionMsg` field `usage *agent.Usage`; `reactionMsg` becomes `struct{ say string; usage *agent.Usage }`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/agent/decide_test.go`:

```go
func TestGetDecisionSumsUsageAcrossRetry(t *testing.T) {
	calls := 0
	flaky := func(ctx context.Context, prompt string) (Response, error) {
		calls++
		u := &Usage{InputTokens: 100, OutputTokens: 10, CostUSD: 0.01}
		if calls == 1 {
			return Response{Text: "hmm let me think", Usage: u}, nil
		}
		return Response{Text: `{"action": "call"}`, Usage: u}, nil
	}
	_, _, _, used := GetDecision(context.Background(), flaky, RequestData{}, legalFCR())
	if used == nil || used.InputTokens != 200 || used.OutputTokens != 20 {
		t.Errorf("used = %+v, want summed over 2 calls", used)
	}
}

func TestGetDecisionNilUsageStaysNil(t *testing.T) {
	plain := func(ctx context.Context, prompt string) (Response, error) {
		return Response{Text: `{"action": "call"}`}, nil
	}
	_, _, _, used := GetDecision(context.Background(), plain, RequestData{}, legalFCR())
	if used != nil {
		t.Errorf("used = %+v, want nil for usage-less adapter", used)
	}
}
```

Append to `internal/tui/app_test.go`:

```go
func TestSessionUsageAccumulates(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check},
		usage: &agent.Usage{InputTokens: 100, OutputTokens: 5, CostUSD: 0.02}})
	m = m2.(Model)
	m2, _ = m.Update(reactionMsg{say: "gg",
		usage: &agent.Usage{InputTokens: 50, OutputTokens: 3, CostUSD: 0.01}})
	m = m2.(Model)
	if m.sessionUsage.InputTokens != 150 || m.sessionUsage.CostUSD != 0.03 {
		t.Errorf("sessionUsage = %+v", m.sessionUsage)
	}
}
```

Note: `TestSessionUsageAccumulates` sends `decisionMsg` when it may not be the agent's turn; `apply` failing is fine — the fallback path in `Update` still runs and usage must still accumulate **before** the apply. Put `m.sessionUsage.Add(msg.usage)` first in the case.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ ./internal/tui/ -run 'SumsUsage|NilUsage|SessionUsage' -v`
Expected: FAIL — wrong number of return values / undefined fields

- [ ] **Step 3: Implement**

`internal/agent/decide.go`:

```go
// GetDecision asks, retries once on unusable output, and falls back to
// check/fold so a match can never stall (ADR-0001). Returns
// (action, say, fallbackUsed, usage) — usage summed across attempts,
// nil when the adapter reports none.
func GetDecision(ctx context.Context, ask Asker, data RequestData, legal []poker.ActionType) (poker.Action, string, bool, *Usage) {
	prompt := RenderPrompt(data)
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

`GetReaction` returns `(string, *Usage)`: on error `return "", resp.Usage`; on success `return line, resp.Usage`.

`internal/tui/app.go`:

```go
type decisionMsg struct {
	act      poker.Action
	say      string
	fallback bool
	usage    *agent.Usage
}
type reactionMsg struct {
	say   string
	usage *agent.Usage
}
```

`Model` gains `sessionUsage agent.Usage` (next to `saved bool`).

`askAgentCmd` closure:

```go
	return func() tea.Msg {
		act, say, fb, u := agent.GetDecision(context.Background(), ask, data, legal)
		return decisionMsg{act: act, say: say, fallback: fb, usage: u}
	}
```

`finishHand` reaction closure:

```go
		cmds = append(cmds, func() tea.Msg {
			say, u := agent.GetReaction(context.Background(), ask, name, digest, summary)
			return reactionMsg{say: say, usage: u}
		})
```

`Update` — first line of `case decisionMsg:` becomes `m.sessionUsage.Add(msg.usage)`; `case reactionMsg:` becomes:

```go
	case reactionMsg:
		m.sessionUsage.Add(msg.usage)
		if msg.say != "" && !m.quiet {
			m.agentSay = msg.say
		}
		return m, nil
```

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS (fix any remaining 3-value destructurings of `GetDecision` in tests: add `, _`)

- [ ] **Step 5: Commit**

```bash
gofmt -l . && git add -A
git commit -m "feat: thread agent usage into TUI session accumulator"
```

---

### Task 4: Debug log records usage per `agent_call`

**Files:**
- Modify: `internal/debuglog/asker.go`
- Test: `internal/debuglog/asker_test.go`

**Interfaces:**
- Consumes: `agent.Response`, `agent.Usage` (Tasks 1–2).
- Produces: `agent_call` events gain, when `Usage != nil`: `tokens_in` (fresh input), `tokens_out`, `cache_read`, `cache_write`, `cost_usd`. Absent entirely when nil.

- [ ] **Step 1: Write the failing test**

Append to `internal/debuglog/asker_test.go`:

```go
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
```

Also extend `TestWrapAskerPassthroughAndLog`: assert `_, ok := evs[0]["tokens_in"]; !ok` (no usage fields when Usage is nil).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/debuglog/ -run TestWrapAskerLogsUsage -v`
Expected: FAIL — usage fields missing

- [ ] **Step 3: Implement**

In `WrapAsker`, after building `f`:

```go
		if resp.Usage != nil {
			u := resp.Usage
			f["tokens_in"] = u.InputTokens
			f["tokens_out"] = u.OutputTokens
			f["cache_read"] = u.CacheRead
			f["cache_write"] = u.CacheWrite
			f["cost_usd"] = u.CostUSD
		}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/debuglog/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
gofmt -l . && git add internal/debuglog/
git commit -m "feat: agent_call debug events include token usage and cost"
```

---

### Task 5: Lifetime token/cost totals in `stats.Record`

**Files:**
- Modify: `internal/stats/stats.go`
- Test: `internal/stats/stats_test.go`

**Interfaces:**
- Consumes: nothing new (plain ints/float; stats must not import agent).
- Produces: `Record` gains `TokensIn int64 `json:"tokens_in,omitempty"``, `TokensOut int64 `json:"tokens_out,omitempty"``, `CostUSD float64 `json:"cost_usd,omitempty"``. `Line()` appends `` · $X.XX`` when `CostUSD > 0`.

- [ ] **Step 1: Write the failing test**

Append to `internal/stats/stats_test.go`:

```go
func TestRecordUsageRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stats.json")
	s := Stats{"claude": {Wins: 1, TokensIn: 50000, TokensOut: 2000, CostUSD: 0.43}}
	if err := s.Save(p); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got["claude"] != s["claude"] {
		t.Errorf("got %+v", got["claude"])
	}
}

func TestLineIncludesCost(t *testing.T) {
	s := Stats{"claude": {Wins: 2, Losses: 1, CostUSD: 0.43}}
	if got := s.Line("claude"); got != "vs claude: 2–1 · $0.43" {
		t.Errorf("Line = %q", got)
	}
	// zero-cost records keep the old format exactly
	s2 := Stats{"codex": {Wins: 3, Losses: 1}}
	if got := s2.Line("codex"); got != "vs codex: 3–1" {
		t.Errorf("Line = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stats/ -v`
Expected: FAIL — unknown fields `TokensIn`, etc.

- [ ] **Step 3: Implement**

```go
type Record struct {
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	TokensIn  int64   `json:"tokens_in,omitempty"`
	TokensOut int64   `json:"tokens_out,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
}
```

```go
func (s Stats) Line(key string) string {
	r := s[key]
	line := fmt.Sprintf("vs %s: %d–%d", key, r.Wins, r.Losses)
	if r.CostUSD > 0 {
		line += fmt.Sprintf(" · $%.2f", r.CostUSD)
	}
	return line
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/stats/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
gofmt -l . && git add internal/stats/
git commit -m "feat: stats track lifetime tokens and cost per agent"
```

---

### Task 6: Persist session usage at match end; show it on the match-over screen

**Files:**
- Modify: `internal/tui/app.go` (`settleAndNext` line 206, `View` match-over branch line 567)
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `Model.sessionUsage` (Task 3), `stats.Record` usage fields (Task 5), `Usage.TotalIn()` (Task 1).
- Produces: on match over, session usage is added to the agent's `stats.Record` before `Save`; match-over view shows `tokens this match: <TotalIn()> in / <OutputTokens> out · $<CostUSD>` when usage is non-zero.

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/app_test.go` (mirrors `TestMatchOverShowsCorrectStacks`):

```go
func TestMatchOverSavesAndShowsUsage(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m.sessionUsage = agent.Usage{InputTokens: 1000, OutputTokens: 200, CacheRead: 500, CostUSD: 0.09}

	hs, as := m.humanSeat(), m.agentSeat()
	m.hand.Seats[hs].Stack = 3000
	m.hand.Seats[as].Stack = 0
	m.settleAndNext()

	if m.phase != phaseMatchOver {
		t.Fatalf("phase = %v, want phaseMatchOver", m.phase)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "tokens this match: 1500 in / 200 out · $0.09") {
		t.Errorf("view missing usage line, got:\n%s", view)
	}
	saved, err := stats.Load(m.statsPath)
	if err != nil {
		t.Fatalf("load stats: %v", err)
	}
	r := saved["stub"]
	if r.TokensIn != 1500 || r.TokensOut != 200 || r.CostUSD != 0.09 {
		t.Errorf("saved record = %+v", r)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestMatchOverSavesAndShowsUsage -v`
Expected: FAIL — usage line missing, record zero

- [ ] **Step 3: Implement**

`settleAndNext`, inside the `!m.saved` block after the win/loss increment:

```go
			r.TokensIn += int64(m.sessionUsage.TotalIn())
			r.TokensOut += int64(m.sessionUsage.OutputTokens)
			r.CostUSD += m.sessionUsage.CostUSD
```

`View`, `case phaseMatchOver:` — after the stats line, before `enter/q to exit`:

```go
			usageLine := ""
			if m.sessionUsage != (agent.Usage{}) {
				usageLine = fmt.Sprintf("\n  tokens this match: %d in / %d out · $%.2f",
					m.sessionUsage.TotalIn(), m.sessionUsage.OutputTokens, m.sessionUsage.CostUSD)
			}
			b.WriteString("\n  ═══ " + winner + " ═══\n  " + m.stats.Line(m.opp.Key) + usageLine + "\n  enter/q to exit\n")
```

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
gofmt -l . && git add internal/tui/
git commit -m "feat: match-over screen shows and persists session token cost"
```

---

### Task 7: Model choice — `--model` flag + picker step

**Files:**
- Modify: `internal/agent/adapter.go` (Args signature, `Model`/`Models` fields, Asker call)
- Modify: `internal/tui/picker.go` (io.Reader + model step)
- Modify: `main.go` (`--model` flag, picker call, session_start already logs via NewModel — add model there too: `internal/tui/app.go` line 79)
- Test: `internal/agent/adapter_test.go`, `internal/tui/picker_test.go` (new), `internal/tui/app_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces: `Adapter{ …, Model string, Models []string, Args func(model, prompt string) []string }`; `Asker` passes `a.Args(a.Model, prompt)`; `RunPicker(roster []agent.Adapter, st stats.Stats, in io.Reader, presetModel string) (agent.Adapter, error)`; `main.go` flag `-model`; `session_start` debug event gains `"model"`.

- [ ] **Step 1: Write the failing tests**

Replace `TestKnownAdapterArgs` in `internal/agent/adapter_test.go`:

```go
func TestKnownAdapterArgs(t *testing.T) {
	for _, a := range knownAdapters {
		for _, model := range []string{"", "test-model"} {
			args := a.Args(model, "PROMPT")
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "PROMPT") {
				t.Errorf("%s(model=%q): prompt not in args %v", a.Key, model, args)
			}
			if model != "" && !strings.Contains(joined, "test-model") {
				t.Errorf("%s: model not in args %v", a.Key, args)
			}
			if model == "" && strings.Contains(joined, "--model") || model == "" && strings.Contains(joined, "-m ") {
				t.Errorf("%s: model flag present with empty model: %v", a.Key, args)
			}
		}
	}
}

func TestClaudeAdapterHasModelPresets(t *testing.T) {
	for _, a := range knownAdapters {
		if a.Key == "claude" && len(a.Models) == 0 {
			t.Error("claude adapter should suggest model presets")
		}
	}
}
```

Create `internal/tui/picker_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

func pickerRoster() []agent.Adapter {
	return []agent.Adapter{{Key: "claude", DisplayName: "Claude Code", Bin: "claude",
		Args:   func(m, p string) []string { return nil },
		Models: []string{"haiku", "sonnet", "opus"}}}
}

func TestPickerModelSelection(t *testing.T) {
	a, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("2\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", a.Model)
	}
}

func TestPickerModelDefaultOnEmpty(t *testing.T) {
	a, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "" {
		t.Errorf("Model = %q, want CLI default (empty)", a.Model)
	}
}

func TestPickerPresetModelSkipsPrompt(t *testing.T) {
	// no input available at all — preset must short-circuit before any Scan
	a, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader(""), "opus")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "opus" {
		t.Errorf("Model = %q, want opus", a.Model)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ ./internal/tui/ -run 'KnownAdapterArgs|ModelPresets|Picker' -v`
Expected: FAIL — Args arity, missing fields, RunPicker arity

- [ ] **Step 3: Implement adapter changes**

`internal/agent/adapter.go`:

```go
type Adapter struct {
	Key         string
	DisplayName string
	Bin         string
	Model       string   // empty = CLI default
	Models      []string // suggested presets for the picker; empty = flag-only
	Args        func(model, prompt string) []string
	Parse       func(raw string) Response // nil = plain-text output
}

var knownAdapters = []Adapter{
	{Key: "claude", DisplayName: "Claude Code", Bin: "claude",
		Models: []string{"haiku", "sonnet", "opus"},
		Args: func(m, p string) []string {
			args := []string{"-p", p, "--output-format", "json"}
			if m != "" {
				args = append(args, "--model", m)
			}
			return args
		},
		Parse: ParseClaudeJSON},
	{Key: "codex", DisplayName: "Codex", Bin: "codex",
		Args: func(m, p string) []string {
			args := []string{"exec"}
			if m != "" {
				args = append(args, "-m", m)
			}
			return append(args, p)
		}},
	{Key: "gemini", DisplayName: "Gemini CLI", Bin: "gemini",
		Args: func(m, p string) []string {
			var args []string
			if m != "" {
				args = append(args, "-m", m)
			}
			return append(args, "-p", p)
		}},
}
```

In the `Asker` method: `cmd := exec.CommandContext(cctx, a.Bin, a.Args(a.Model, prompt)...)`.
Custom adapter in `DetectRoster`: `Args: func(_, p string) []string { return append(append([]string{}, parts[1:]...), p) }` (custom commands don't get a model flag).

Update remaining `Args(` call sites/test stubs to the two-arg form: `internal/tui/app_test.go` `testModel` (`Args: func(m, p string) []string { return nil }`), `TestClaudeAdapterUsesJSONOutput` (`a.Args("", "PROMPT")`), `TestAskerAppliesParse`.

- [ ] **Step 4: Implement picker + main**

`internal/tui/picker.go`:

```go
// RunPicker chooses the opponent (and optionally its model) before the TUI
// starts. presetModel skips the model prompt (comes from --model).
func RunPicker(roster []agent.Adapter, st stats.Stats, in io.Reader, presetModel string) (agent.Adapter, error) {
	if len(roster) == 0 {
		return agent.Adapter{}, fmt.Errorf("no agent CLIs found on PATH (looked for: claude, codex, gemini)")
	}
	sc := bufio.NewScanner(in)
	chosen := roster[0]
	if len(roster) > 1 {
		fmt.Println("♠ choose your opponent:")
		for i, a := range roster {
			fmt.Printf("  %d. %-12s %s\n", i+1, a.DisplayName, st.Line(a.Key))
		}
		fmt.Print("> ")
		if !sc.Scan() {
			return agent.Adapter{}, fmt.Errorf("no selection")
		}
		n, err := strconv.Atoi(strings.TrimSpace(sc.Text()))
		if err != nil || n < 1 || n > len(roster) {
			return agent.Adapter{}, fmt.Errorf("pick a number 1-%d", len(roster))
		}
		chosen = roster[n-1]
	}
	if presetModel != "" {
		chosen.Model = presetModel
		return chosen, nil
	}
	if len(chosen.Models) > 0 {
		fmt.Printf("♦ model for %s (enter = default):\n", chosen.DisplayName)
		for i, mo := range chosen.Models {
			fmt.Printf("  %d. %s\n", i+1, mo)
		}
		fmt.Print("> ")
		if sc.Scan() {
			txt := strings.TrimSpace(sc.Text())
			if txt != "" {
				n, err := strconv.Atoi(txt)
				if err != nil || n < 1 || n > len(chosen.Models) {
					return agent.Adapter{}, fmt.Errorf("pick a number 1-%d or enter for default", len(chosen.Models))
				}
				chosen.Model = chosen.Models[n-1]
			}
		}
	}
	return chosen, nil
}
```

(add `"io"` to imports)

`main.go`:

```go
	modelFlag := flag.String("model", "", "model for the agent CLI (claude: haiku/sonnet/opus; codex/gemini: passed through)")
```

- `--agent` path: after `opp` is resolved, `opp.Model = *modelFlag`.
- picker path: `opp, err = tui.RunPicker(roster, st, os.Stdin, *modelFlag)`.

`internal/tui/app.go` `NewModel` session_start log gains `"model": opp.Model`.

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Manual smoke test (real claude call)**

Run: `go run . --agent claude --model haiku --debug` — play one decision, quit with `q`, then check the printed debug log path:
`grep -o '"cost_usd":[0-9.]*' ~/.showdown/debug-*.jsonl | tail -2`
Expected: `agent_call` events carry `cost_usd` > 0.

- [ ] **Step 7: Commit**

```bash
gofmt -l . && git add -A
git commit -m "feat: --model flag and picker model selection per adapter"
```

---

### Task 8: Docs + graph update

**Files:**
- Modify: `README.md` (flag list around line 14)
- Modify: `graphify-out/` via incremental rebuild

- [ ] **Step 1: README**

In the paragraph listing flags (README.md:14-15), after the `--agent codex` sentence add: ``Pick the model with `--model haiku` (cheaper and faster than the default). Match cost shows on the match-over screen and accumulates in your career stats.``

- [ ] **Step 2: Commit docs**

```bash
git add README.md docs/superpowers/plans/2026-07-03-token-cost-and-model-choice.md
git commit -m "docs: plan + README for token-cost measurement and model choice"
```

- [ ] **Step 3: Graph incremental update (repo convention)**

Run `/graphify . --update`, then:

```bash
git add graphify-out/
git commit -m "graph: incremental update for token-cost + model-choice features"
```
