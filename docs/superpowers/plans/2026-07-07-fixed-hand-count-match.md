# Fixed Hand-Count Match Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let players cap a match at a fixed number of hands (instead of playing until someone busts) via `--hands` or a menu row, defaulting to today's unlimited/bust-only behavior.

**Architecture:** `poker.Match` already isolates match-end logic behind `Over()`/`Winner()`; add a `handLimit` field there. The match ends at the hand limit only if stacks differ — tied stacks at the limit keep playing ("sudden death") until a hand breaks the tie, so the match always resolves to a clean win/loss (no draw state, no stats schema change). Thread the new value through `tui.NewModel` (same pattern as the just-shipped `--stack`/`--blind` work) and the menu dashboard (a third cycling row, same pattern as stack/blind) and a `--hands` CLI flag.

**Tech Stack:** Go, bubbletea (menu TUI), existing `internal/poker`, `internal/tui`, `main` packages.

## Global Constraints

- Default hand limit stays **0 (unlimited — play until someone busts)** — behavior for anyone who doesn't touch `--hands`/the new menu row must be byte-for-byte identical to today.
- The hand-limit ending is a **cap alongside bust-out, not a replacement**: a stack hitting 0 always ends the match immediately, regardless of hand count.
- Tied stacks exactly at the hand limit do **not** end the match — the match continues one hand at a time ("sudden death") until either a bust occurs or the tie breaks, at which point the leader wins.
- `internal/agent`-facing text describing how the match ended must never say "busted" or "took every chip" when the match actually ended by hitting the hand limit with a chip lead — this is the same table-truth rule from the recently-merged stack/blind work: the agent's reaction prompt must never state something false about how the match ended.

---

### Task 1: `poker.Match` — hand limit and sudden-death tie logic

**Files:**
- Modify: `internal/poker/match.go`
- Test: `internal/poker/match_test.go`
- Unavoidable one-line shim: `internal/tui/app.go` (see Step 5 — `poker.NewMatch`'s signature grows by one parameter, and `internal/tui/app.go` is its only other caller; Task 2 owns and fully rewires this file, this step just keeps the repo compiling in the meantime)

**Interfaces:**
- Produces: `poker.NewMatch(startStack, startSB, handLimit int) *Match` (replaces the 2-arg constructor). `Match.EndedByBust() bool` — new method distinguishing a bust ending from a hand-limit ending. `Match.Over()`/`Match.Winner()` behavior unchanged in shape, now hand-limit-aware.

- [ ] **Step 1: Update existing tests to the new constructor signature and add hand-limit tests**

Replace the full contents of `internal/poker/match_test.go`:

```go
package poker

import "testing"

func TestBlindSchedule(t *testing.T) {
	m := NewMatch(1500, 10, 0)
	cases := []struct{ hand, sb, bb int }{
		{1, 10, 20}, {10, 10, 20}, {11, 15, 30}, {21, 25, 50},
		{31, 50, 100}, {41, 100, 200}, {51, 200, 400}, {99, 200, 400},
	}
	for _, c := range cases {
		m.HandNum = c.hand
		sb, bb := m.Blinds()
		if sb != c.sb || bb != c.bb {
			t.Errorf("hand %d: blinds %d/%d, want %d/%d", c.hand, sb, bb, c.sb, c.bb)
		}
	}
}

// TestBlindScheduleScalesWithStartSB checks a non-default starting small
// blind (25) scales the whole 6-level schedule by the same ratios as the
// 10/20 default (1x, 1.5x, 2.5x, 5x, 10x, 20x), rounding to the nearest chip.
func TestBlindScheduleScalesWithStartSB(t *testing.T) {
	m := NewMatch(3000, 25, 0)
	cases := []struct{ hand, sb, bb int }{
		{1, 25, 50}, {11, 38, 76}, {21, 63, 126},
		{31, 125, 250}, {41, 250, 500}, {51, 500, 1000}, {99, 500, 1000},
	}
	for _, c := range cases {
		m.HandNum = c.hand
		sb, bb := m.Blinds()
		if sb != c.sb || bb != c.bb {
			t.Errorf("hand %d: blinds %d/%d, want %d/%d", c.hand, sb, bb, c.sb, c.bb)
		}
	}
}

func TestNewMatchCustomStartingStack(t *testing.T) {
	m := NewMatch(3000, 25, 0)
	if m.Stacks != [2]int{3000, 3000} {
		t.Errorf("Stacks = %v, want [3000 3000]", m.Stacks)
	}
}

func TestButtonAlternates(t *testing.T) {
	m := NewMatch(1500, 10, 0)
	if m.ButtonPlayer() != 0 {
		t.Errorf("hand 1 button = %d, want 0 (human)", m.ButtonPlayer())
	}
	m.HandNum = 2
	if m.ButtonPlayer() != 1 {
		t.Errorf("hand 2 button = %d, want 1", m.ButtonPlayer())
	}
	if m.SeatOf(m.ButtonPlayer()) != 0 {
		t.Error("button player must sit in seat 0")
	}
	if m.PlayerAt(m.SeatOf(0)) != 0 || m.PlayerAt(m.SeatOf(1)) != 1 {
		t.Error("SeatOf/PlayerAt must be inverses")
	}
}

func TestNextHandAndBust(t *testing.T) {
	m := NewMatch(1500, 10, 0) // hand 1: human is button = seat 0
	m.NextHand([2]int{3000, 0})
	if m.HandNum != 2 {
		t.Errorf("HandNum = %d, want 2", m.HandNum)
	}
	if m.Stacks[0] != 3000 || m.Stacks[1] != 0 {
		t.Errorf("stacks = %v, want human 3000 / agent 0", m.Stacks)
	}
	if !m.Over() || m.Winner() != 0 {
		t.Errorf("Over %v Winner %d, want true/0", m.Over(), m.Winner())
	}
	if !m.EndedByBust() {
		t.Error("EndedByBust() should be true: agent stack is 0")
	}
}

func TestHandLimitZeroMeansUnlimited(t *testing.T) {
	m := NewMatch(1500, 10, 0)
	m.HandNum = 500
	m.Stacks = [2]int{1000, 2000}
	if m.Over() {
		t.Error("handLimit=0 should mean unlimited hands; match should only end on bust")
	}
}

func TestHandLimitEndsMatchWhenStacksDiffer(t *testing.T) {
	m := NewMatch(1500, 10, 5)
	m.HandNum = 6 // 5 hands completed
	m.Stacks = [2]int{1800, 1200}
	if !m.Over() {
		t.Fatal("match should be over: hand limit reached with unequal stacks")
	}
	if m.Winner() != 0 {
		t.Errorf("Winner() = %d, want 0 (higher stack)", m.Winner())
	}
	if m.EndedByBust() {
		t.Error("EndedByBust() should be false: match ended by hand limit, not bust")
	}
}

func TestHandLimitSuddenDeathContinuesWhileTied(t *testing.T) {
	m := NewMatch(1500, 10, 5)
	m.HandNum = 6
	m.Stacks = [2]int{1500, 1500}
	if m.Over() {
		t.Fatal("tied stacks at the hand limit should continue (sudden death), not end")
	}
	m.HandNum = 7
	m.Stacks = [2]int{1600, 1400}
	if !m.Over() {
		t.Fatal("match should end once a sudden-death hand breaks the tie")
	}
	if m.Winner() != 0 {
		t.Errorf("Winner() = %d, want 0 (higher stack)", m.Winner())
	}
}

func TestHandLimitDoesNotOverrideBust(t *testing.T) {
	m := NewMatch(1500, 10, 100)
	m.HandNum = 3 // well before the limit
	m.Stacks = [2]int{3000, 0}
	if !m.Over() {
		t.Error("bust should end the match immediately regardless of hand limit")
	}
	if m.Winner() != 0 {
		t.Errorf("Winner() = %d, want 0", m.Winner())
	}
	if !m.EndedByBust() {
		t.Error("EndedByBust() should be true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./internal/poker/...`
Expected: FAIL — `too many arguments in call to NewMatch` / `m.EndedByBust undefined` (production code still has the old constructor and no such method).

- [ ] **Step 3: Implement the hand limit and sudden-death logic**

Replace the full contents of `internal/poker/match.go`:

```go
package poker

import "math"

// blindFactors are the escalation ratios of the standard 10/20 schedule
// (10, 15, 25, 50, 100, 200 → 1x, 1.5x, 2.5x, 5x, 10x, 20x of the starting
// small blind). blindLevelsFor scales these to any starting small blind so
// the escalation feel stays the same regardless of buy-in size.
var blindFactors = []float64{1, 1.5, 2.5, 5, 10, 20}

func blindLevelsFor(startSB int) [][2]int {
	levels := make([][2]int, len(blindFactors))
	for i, f := range blindFactors {
		sb := int(math.Round(float64(startSB) * f))
		if sb < 1 {
			sb = 1
		}
		levels[i] = [2]int{sb, sb * 2}
	}
	return levels
}

// Match tracks the sit-and-go by player id: 0 = human, 1 = agent.
type Match struct {
	Stacks  [2]int
	HandNum int

	levels    [][2]int
	handLimit int // 0 = unlimited (bust-only); otherwise a hand cap
}

func NewMatch(startStack, startSB, handLimit int) *Match {
	return &Match{Stacks: [2]int{startStack, startStack}, HandNum: 1, levels: blindLevelsFor(startSB), handLimit: handLimit}
}

func (m *Match) Blinds() (int, int) {
	lvl := (m.HandNum - 1) / 10
	if lvl >= len(m.levels) {
		lvl = len(m.levels) - 1
	}
	return m.levels[lvl][0], m.levels[lvl][1]
}

func (m *Match) ButtonPlayer() int { return (m.HandNum - 1) % 2 }

// SeatOf maps a player id to a hand seat (seat 0 = button).
func (m *Match) SeatOf(player int) int {
	if player == m.ButtonPlayer() {
		return 0
	}
	return 1
}

func (m *Match) PlayerAt(seat int) int {
	if seat == 0 {
		return m.ButtonPlayer()
	}
	return 1 - m.ButtonPlayer()
}

// NextHand records final stacks (indexed by seat) and advances the hand counter.
func (m *Match) NextHand(finalSeatStacks [2]int) {
	m.Stacks[m.PlayerAt(0)] = finalSeatStacks[0]
	m.Stacks[m.PlayerAt(1)] = finalSeatStacks[1]
	m.HandNum++
}

// EndedByBust reports whether either player is out of chips. Distinguishes a
// bust ending from a hand-limit ending so callers (e.g. the agent's
// match-over reaction) can describe the true reason the match ended.
func (m *Match) EndedByBust() bool { return m.Stacks[0] == 0 || m.Stacks[1] == 0 }

// Over reports whether the match has ended: either someone busted, or the
// hand limit (if set) has been reached with stacks no longer tied. Tied
// stacks at the limit play on ("sudden death") until a hand breaks the tie.
func (m *Match) Over() bool {
	if m.EndedByBust() {
		return true
	}
	return m.handLimit > 0 && m.HandNum-1 >= m.handLimit && m.Stacks[0] != m.Stacks[1]
}

func (m *Match) Winner() int {
	switch {
	case !m.Over():
		return -1
	case m.Stacks[0] == 0:
		return 1
	case m.Stacks[1] == 0:
		return 0
	case m.Stacks[0] > m.Stacks[1]:
		return 0
	default:
		return 1
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/poker/...`
Expected: PASS (all of `TestBlindSchedule`, `TestBlindScheduleScalesWithStartSB`, `TestNewMatchCustomStartingStack`, `TestButtonAlternates`, `TestNextHandAndBust`, `TestHandLimitZeroMeansUnlimited`, `TestHandLimitEndsMatchWhenStacksDiffer`, `TestHandLimitSuddenDeathContinuesWhileTied`, `TestHandLimitDoesNotOverrideBust`).

- [ ] **Step 5: Keep the repo compiling — one-line shim in `internal/tui/app.go`**

`poker.NewMatch` is also called from `internal/tui/app.go`'s `NewModel` function (that file's actual owner is Task 2, which will fully rewire it). Find this line in `internal/tui/app.go` (inside `NewModel`, in the `return Model{...}` block):

```go
		match: poker.NewMatch(startStack, startSB), digest: agent.NewDigest(startStack),
```

Change it to pass a temporary hardcoded `0` (unlimited) for the new `handLimit` parameter, so the repo keeps building until Task 2 threads the real value through:

```go
		match: poker.NewMatch(startStack, startSB, 0), digest: agent.NewDigest(startStack), // TODO(Task 2): thread handLimit through here
```

Run: `go build ./...`
Expected: builds cleanly.

Run: `go test ./...`
Expected: PASS across all packages (this is the full-suite check before committing).

- [ ] **Step 6: Commit**

```bash
git add internal/poker/match.go internal/poker/match_test.go internal/tui/app.go
git commit -m "feat: fixed hand-count matches with sudden-death tie-break"
```

---

### Task 2: `tui.NewModel` — thread hand limit; fix match-over wording for table truth

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`
- Unavoidable one-line shim: `main.go` (see Step 5 — `NewModel`'s signature grows by one parameter; `main.go`'s owner for this feature is Task 4, which fully rewires it)

**Interfaces:**
- Consumes: `poker.NewMatch(startStack, startSB, handLimit int) *Match` (Task 1), `Match.EndedByBust() bool` (Task 1).
- Produces: `tui.NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir, personality string, startStack, startSB, handLimit int, log *debuglog.Logger) Model` — Task 4 (main.go) calls this with the real flag/menu value. `matchOverOutcome(endedByBust, agentWon bool) string` — pure function, no other task consumes it directly but its correctness is what closes the table-truth gap.

- [ ] **Step 1: Add failing tests for the new parameter and the wording fix**

Add this test to `internal/tui/app_test.go`, right after `TestNewModelUsesCustomStackAndBlind` (which ends at line 36, just before `func key(s string)`):

```go
func TestNewModelUsesCustomHandLimit(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 5, nil)
	m.match.HandNum = 6
	m.match.Stacks = [2]int{1800, 1200}
	if !m.match.Over() {
		t.Error("match should be over: hand limit 5 reached with unequal stacks")
	}
}

func TestMatchOverOutcomeStatesTrueEndingReason(t *testing.T) {
	cases := []struct {
		name             string
		endedByBust      bool
		agentWon         bool
		wantSubstring    string
		wantNotSubstring string
	}{
		{"agent wins by bust", true, true, "Your human is busted", "Hand limit"},
		{"agent loses by bust", true, false, "They took every chip", "Hand limit"},
		{"agent wins by hand limit", false, true, "Hand limit hit", "busted"},
		{"agent loses by hand limit", false, false, "Hand limit hit", "busted"},
	}
	for _, c := range cases {
		got := matchOverOutcome(c.endedByBust, c.agentWon)
		if !strings.Contains(got, c.wantSubstring) {
			t.Errorf("%s: outcome = %q, want substring %q", c.name, got, c.wantSubstring)
		}
		if strings.Contains(got, c.wantNotSubstring) {
			t.Errorf("%s: outcome = %q, should not mention %q", c.name, got, c.wantNotSubstring)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./internal/tui/... -run 'TestNewModelUsesCustomHandLimit|TestMatchOverOutcomeStatesTrueEndingReason'`
Expected: FAIL — `too many arguments in call to NewModel` and/or `undefined: matchOverOutcome`.

- [ ] **Step 3: Update `NewModel`'s signature and body**

In `internal/tui/app.go`, find the `NewModel` function and change:

```go
func NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir, personality string, startStack, startSB int, log *debuglog.Logger) Model {
	in := textinput.New()
	in.CharLimit = 120
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	seed := time.Now().UnixNano()
	log.Log("session_start", map[string]any{
		"agent_key": opp.Key, "agent_name": opp.DisplayName,
		"model": opp.Model, "quiet": quiet, "dir": dir, "seed": seed,
		"personality": personality, "start_stack": startStack, "start_sb": startSB,
	})
	return Model{
		opp: opp, stats: st, statsPath: statsPath, quiet: quiet, dir: dir,
		personality: personality, log: log,
		rng:   rand.New(rand.NewSource(seed)),
		match: poker.NewMatch(startStack, startSB, 0), digest: agent.NewDigest(startStack), // TODO(Task 2): thread handLimit through here
		input: in, spin: sp,
	}
}
```

to:

```go
func NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir, personality string, startStack, startSB, handLimit int, log *debuglog.Logger) Model {
	in := textinput.New()
	in.CharLimit = 120
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	seed := time.Now().UnixNano()
	log.Log("session_start", map[string]any{
		"agent_key": opp.Key, "agent_name": opp.DisplayName,
		"model": opp.Model, "quiet": quiet, "dir": dir, "seed": seed,
		"personality": personality, "start_stack": startStack, "start_sb": startSB, "hand_limit": handLimit,
	})
	return Model{
		opp: opp, stats: st, statsPath: statsPath, quiet: quiet, dir: dir,
		personality: personality, log: log,
		rng:   rand.New(rand.NewSource(seed)),
		match: poker.NewMatch(startStack, startSB, handLimit), digest: agent.NewDigest(startStack),
		input: in, spin: sp,
	}
}
```

(If your checkout's line reads slightly differently — e.g. the `// TODO(Task 2)` comment text isn't byte-identical — locate `func NewModel` directly and apply the same intent: add `handLimit int` as the third of the three trailing ints, add it to the `log.Log` map, pass it as `poker.NewMatch`'s third argument, and drop the TODO comment.)

- [ ] **Step 4: Fix the match-over outcome wording (table-truth fix)**

In `internal/tui/app.go`, find `func (m *Model) settleAndNext() tea.Cmd {` and, immediately above it, add this new function:

```go
// matchOverOutcome states how the match ended, from the agent's own
// perspective, for its match-over reaction prompt. It must never claim a
// bust that didn't happen (table-truth rule) — a match can also end by
// hitting a configured hand limit with the bigger stack, which is not a
// bust, and the agent's own "memory" of the match must never state
// something false about how it ended.
func matchOverOutcome(endedByBust, agentWon bool) string {
	switch {
	case endedByBust && agentWon:
		return "MATCH OVER: you WON the match. Your human is busted."
	case endedByBust:
		return "MATCH OVER: you LOST the match to your human. They took every chip."
	case agentWon:
		return "MATCH OVER: you WON the match. Hand limit hit — you had the bigger stack."
	default:
		return "MATCH OVER: you LOST the match to your human. Hand limit hit — they had the bigger stack."
	}
}
```

Then, inside `settleAndNext`, change:

```go
		if !m.quiet {
			outcome := "MATCH OVER: you LOST the match to your human. They took every chip."
			if m.match.Winner() == 1 {
				outcome = "MATCH OVER: you WON the match. Your human is busted."
			}
```

to:

```go
		if !m.quiet {
			outcome := matchOverOutcome(m.match.EndedByBust(), m.match.Winner() == 1)
```

Everything below this block (the `name, digest, persona := ...` line onward) is unchanged — only these lines are replaced.

- [ ] **Step 5: Fix the other 9 `NewModel` call sites in `internal/tui/app_test.go`**

These are the only other places `NewModel` is called (grep confirmed). All 9 currently pass `1500, 10` or `3000, 25` immediately before the trailing `log`/`l`/`nil` argument. Apply these substring replacements (search for the exact substring across the whole file; each covers multiple identical call sites at once):

1. Replace **every** occurrence of the substring `1500, 10, nil)` with `1500, 10, 0, nil)`. This covers 3 call sites: the `testModel` helper, and two single-hand tests that pass `nil` as the log.
2. Replace **every** occurrence of the substring `1500, 10, l)` with `1500, 10, 0, l)`. This covers 5 call sites across various debug-log-backed tests (including the two using `"PERSONA-MARKER"` as the persona argument).
3. Replace the one occurrence of `3000, 25, nil)` with `3000, 25, 0, nil)`. This is `TestNewModelUsesCustomStackAndBlind`'s call.

After these three replacements, every `NewModel` call in the file should have exactly 10 positional arguments, with `0` as the hand-limit value (unlimited) in all pre-existing tests, and `5` only in the new `TestNewModelUsesCustomHandLimit`.

- [ ] **Step 6: Keep the repo compiling — one-line shim in `main.go`**

`main.go` also calls `tui.NewModel` (that file's actual owner is Task 4, which will fully rewire it). Find this line in `main.go`:

```go
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, dlog)
```

Change it to pass a temporary hardcoded `0` (unlimited) for the new `handLimit` parameter, with a TODO marking it as tracked separately:

```go
	// TODO: thread configurable --hands flag through here (tracked separately).
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, 0, dlog)
```

Run: `go build ./...`
Expected: builds cleanly.

- [ ] **Step 7: Run the full test suite to verify everything passes**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go main.go
git commit -m "feat: thread hand limit into NewModel; fix match-over wording for table truth"
```

---

### Task 3: Menu dashboard — hands row

**Files:**
- Modify: `internal/tui/menu.go` (full rewrite, file is small)
- Test: `internal/tui/menu_test.go` (full rewrite, same reason)
- Unavoidable one-line shim: `main.go` (see Step 4 — `RunMenu`'s signature and return arity both grow by one; `main.go`'s owner for this feature is Task 4, which fully rewires it)

**Interfaces:**
- Consumes: nothing new from other tasks (this task is a leaf with respect to Tasks 1-2 — menu.go/menu_test.go were untouched by them).
- Produces: `tui.DefaultHandLimit = 0` (exported constant — Task 4's main.go uses this as the `--hands` flag default). `tui.RunMenu(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB, presetHands int, presetQuiet bool) (agent.Adapter, string, bool, int, int, int, error)` — return order is `(adapter, persona, quiet, stack, smallBlind, handLimit, err)`.

- [ ] **Step 1: Rewrite `internal/tui/menu_test.go` with updated signatures and new coverage**

Replace the full contents of `internal/tui/menu_test.go`:

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

func menuRoster() []agent.Adapter {
	return []agent.Adapter{
		{Key: "claude", DisplayName: "Claude Code", Models: []string{"haiku", "sonnet", "opus"}},
		{Key: "codex", DisplayName: "Codex"}, // no Models: flag-only adapter
	}
}

// personaFile that does not exist
func noPersonaFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "personality.md")
}

func TestMenuModelOptions(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	want := []string{"(default)", "haiku", "sonnet", "opus"}
	if len(m.modelOpts[0]) != 4 {
		t.Fatalf("claude modelOpts = %v, want %v", m.modelOpts[0], want)
	}
	for i, w := range want {
		if m.modelOpts[0][i] != w {
			t.Errorf("modelOpts[0][%d] = %q, want %q", i, m.modelOpts[0][i], w)
		}
	}
	if len(m.modelOpts[1]) != 1 || m.modelOpts[1][0] != menuDefaultModel {
		t.Errorf("codex modelOpts = %v, want [(default)]", m.modelOpts[1])
	}
	if m.modelIdx != 0 || m.talkOn != true {
		t.Errorf("defaults: modelIdx=%d talkOn=%v, want 0 true", m.modelIdx, m.talkOn)
	}
}

func TestMenuModelPrefill(t *testing.T) {
	// flag value in the Models list → selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, true, noPersonaFile(t))
	if m.modelOpts[0][m.modelIdx] != "sonnet" {
		t.Errorf("selected model = %q, want sonnet", m.modelOpts[0][m.modelIdx])
	}
	if m.talkOn {
		t.Error("presetQuiet=true should start talkOn=false")
	}
	// flag value not in any list → appended verbatim everywhere
	m = newMenuModel(menuRoster(), stats.Stats{}, "gpt-x", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	if got := m.modelOpts[0][m.modelInit[0]]; got != "gpt-x" {
		t.Errorf("claude prefill = %q, want gpt-x", got)
	}
	if got := m.modelOpts[1][m.modelInit[1]]; got != "gpt-x" {
		t.Errorf("codex prefill = %q, want gpt-x", got)
	}
}

func TestMenuPersonaOptions(t *testing.T) {
	// no user file: presets only, needler (index 0) selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	if m.personaOpts[0] != "needler" || m.personaIdx != 0 {
		t.Errorf("personaOpts[0]=%q idx=%d, want needler 0", m.personaOpts[0], m.personaIdx)
	}
	// user file exists: custom prepended and selected
	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("be weird"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, f)
	if m.personaOpts[0] != menuCustomPersona || m.personaIdx != 0 {
		t.Errorf("with file: personaOpts[0]=%q idx=%d, want custom 0", m.personaOpts[0], m.personaIdx)
	}
	// --personality preset name → selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "degen", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, f)
	if m.personaOpts[m.personaIdx] != "degen" {
		t.Errorf("prefill persona = %q, want degen", m.personaOpts[m.personaIdx])
	}
	// --personality path → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "/tmp/evil.md", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, f)
	if m.personaOpts[m.personaIdx] != "/tmp/evil.md" {
		t.Errorf("prefill persona = %q, want /tmp/evil.md", m.personaOpts[m.personaIdx])
	}
}

func TestMenuStackAndBlindOptions(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != DefaultStartStack {
		t.Errorf("default stack = %d, want %d", got, DefaultStartStack)
	}
	if got := m.blindOpts[m.blindIdx]; got != DefaultStartSB {
		t.Errorf("default small blind = %d, want %d", got, DefaultStartSB)
	}
}

func TestMenuStackAndBlindPrefill(t *testing.T) {
	// values already in the preset lists → selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 3000, 25, DefaultHandLimit, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != 3000 {
		t.Errorf("stack prefill = %d, want 3000", got)
	}
	if got := m.blindOpts[m.blindIdx]; got != 25 {
		t.Errorf("blind prefill = %d, want 25", got)
	}
	// values not in the preset lists → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", 1234, 7, DefaultHandLimit, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != 1234 {
		t.Errorf("stack custom prefill = %d, want 1234", got)
	}
	if got := m.blindOpts[m.blindIdx]; got != 7 {
		t.Errorf("blind custom prefill = %d, want 7", got)
	}
}

func TestMenuHandsOptions(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	if got := m.handOpts[m.handIdx]; got != DefaultHandLimit {
		t.Errorf("default hand limit = %d, want %d", got, DefaultHandLimit)
	}
}

func TestMenuHandsPrefill(t *testing.T) {
	// value already in the preset list → selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, 25, false, noPersonaFile(t))
	if got := m.handOpts[m.handIdx]; got != 25 {
		t.Errorf("hands prefill = %d, want 25", got)
	}
	// value not in the preset list → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, 17, false, noPersonaFile(t))
	if got := m.handOpts[m.handIdx]; got != 17 {
		t.Errorf("hands custom prefill = %d, want 17", got)
	}
}

func TestMenuNavigationAndCycling(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))

	// focus moves down and clamps at the last row
	for i := 0; i < 10; i++ {
		v, _ := m.Update(key("j"))
		m = v.(menuModel)
	}
	if m.focus != rowTalk {
		t.Fatalf("focus = %d, want rowTalk (%d)", m.focus, rowTalk)
	}
	// talk row toggles
	v, _ := m.Update(key("l"))
	m = v.(menuModel)
	if m.talkOn {
		t.Error("cycling talk row should toggle talkOn to false")
	}

	// up clamps at the top
	for i := 0; i < 10; i++ {
		v, _ = m.Update(key("k"))
		m = v.(menuModel)
	}
	if m.focus != rowOpponent {
		t.Fatalf("focus = %d, want rowOpponent", m.focus)
	}
	// opponent cycles with wrap: right twice on a 2-roster wraps to start
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.oppIdx != 1 {
		t.Fatalf("oppIdx = %d, want 1", m.oppIdx)
	}
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.oppIdx != 0 {
		t.Fatalf("oppIdx = %d, want 0 (wrap)", m.oppIdx)
	}
	// left from 0 wraps to end
	v, _ = m.Update(key("h"))
	m = v.(menuModel)
	if m.oppIdx != 1 {
		t.Fatalf("oppIdx = %d, want 1 (left wrap)", m.oppIdx)
	}
}

func TestMenuStackAndBlindCycling(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	// opponent -> model -> persona -> stack
	for i := 0; i < 3; i++ {
		v, _ := m.Update(key("j"))
		m = v.(menuModel)
	}
	if m.focus != rowStack {
		t.Fatalf("focus = %d, want rowStack (%d)", m.focus, rowStack)
	}
	before := m.stackOpts[m.stackIdx]
	v, _ := m.Update(key("l"))
	m = v.(menuModel)
	if m.stackOpts[m.stackIdx] == before {
		t.Error("cycling right on stack row should change the selected stack")
	}

	// stack -> blind
	v, _ = m.Update(key("j"))
	m = v.(menuModel)
	if m.focus != rowBlind {
		t.Fatalf("focus = %d, want rowBlind (%d)", m.focus, rowBlind)
	}
	beforeSB := m.blindOpts[m.blindIdx]
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.blindOpts[m.blindIdx] == beforeSB {
		t.Error("cycling right on blind row should change the selected small blind")
	}
}

func TestMenuHandsCycling(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	// opponent -> model -> persona -> stack -> blind -> hands
	for i := 0; i < 5; i++ {
		v, _ := m.Update(key("j"))
		m = v.(menuModel)
	}
	if m.focus != rowHands {
		t.Fatalf("focus = %d, want rowHands (%d)", m.focus, rowHands)
	}
	before := m.handOpts[m.handIdx]
	v, _ := m.Update(key("l"))
	m = v.(menuModel)
	if m.handOpts[m.handIdx] == before {
		t.Error("cycling right on hands row should change the selected hand limit")
	}
}

func TestMenuModelResetsOnOpponentChange(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	// focus model row, cycle to haiku
	v, _ := m.Update(key("j"))
	m = v.(menuModel)
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if got := m.modelOpts[0][m.modelIdx]; got != "haiku" {
		t.Fatalf("selected model = %q, want haiku", got)
	}
	// switch opponent: model resets to that adapter's initial index
	v, _ = m.Update(key("k"))
	m = v.(menuModel)
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.modelIdx != m.modelInit[1] {
		t.Errorf("modelIdx = %d, want reset to modelInit[1]=%d", m.modelIdx, m.modelInit[1])
	}
}

func TestMenuEnterAndQuit(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	v, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = v.(menuModel)
	if !m.entered || cmd == nil {
		t.Error("enter should set entered and return tea.Quit")
	}

	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	v, cmd = m.Update(key("q"))
	m = v.(menuModel)
	if !m.quitted || cmd == nil {
		t.Error("q should set quitted and return tea.Quit")
	}
}

func TestMenuResult(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	ad, persona, quiet, stack, sb, hands := m.result()
	if ad.Key != "claude" || ad.Model != "" || persona != "needler" || quiet || stack != DefaultStartStack || sb != DefaultStartSB || hands != DefaultHandLimit {
		t.Errorf("defaults: got key=%q model=%q persona=%q quiet=%v stack=%d sb=%d hands=%d",
			ad.Key, ad.Model, persona, quiet, stack, sb, hands)
	}

	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", 3000, 25, 50, true, f)
	ad, persona, quiet, stack, sb, hands = m.result()
	if ad.Model != "sonnet" || persona != "" || !quiet || stack != 3000 || sb != 25 || hands != 50 {
		t.Errorf("got model=%q persona=%q quiet=%v stack=%d sb=%d hands=%d, want sonnet \"\" true 3000 25 50",
			ad.Model, persona, quiet, stack, sb, hands)
	}
}

func TestMenuView(t *testing.T) {
	st := stats.Stats{"claude": {Wins: 2, Losses: 1}}
	m := newMenuModel(menuRoster(), st, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	v := stripANSI(m.View())

	for _, want := range []string{
		"♠ SHOWDOWN",
		"◀ Claude Code ▶", // focused row wears the arrows
		"vs claude: 2–1",  // stats line for the selected opponent
		"needler",
		"1500",
		"10/20",
		"unlimited",
		"on",
		"[ enter ] deal me in",
		"[ q ] quit",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q\n%s", want, v)
		}
	}
	if strings.Contains(v, "◀ (default) ▶") {
		t.Error("unfocused model row should not wear arrows")
	}

	// focus the model row: arrows move
	u, _ := m.Update(key("j"))
	m = u.(menuModel)
	v = stripANSI(m.View())
	if !strings.Contains(v, "◀ (default) ▶") {
		t.Errorf("focused model row should wear arrows\n%s", v)
	}
	if strings.Contains(v, "◀ Claude Code ▶") {
		t.Error("unfocused opponent row should not wear arrows")
	}
}

func TestRunMenuEmptyRoster(t *testing.T) {
	_, _, _, _, _, _, err := RunMenu(nil, stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false)
	if err == nil || !strings.Contains(err.Error(), "no agent CLIs found on PATH") {
		t.Errorf("err = %v, want no-agents error", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./internal/tui/... -run TestMenu`
Expected: FAIL — `undefined: rowHands` (and related), since `menu.go` hasn't changed yet.

- [ ] **Step 3: Rewrite `internal/tui/menu.go`**

Replace the full contents of `internal/tui/menu.go`:

```go
package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

// Menu rows, top to bottom.
const (
	rowOpponent = iota
	rowModel
	rowPersona
	rowStack
	rowBlind
	rowHands
	rowTalk
	rowCount
)

const (
	menuDefaultModel  = "(default)"
	menuCustomPersona = "custom"

	// DefaultStartStack, DefaultStartSB, and DefaultHandLimit are the
	// sit-and-go defaults. main uses these as --stack/--blind/--hands flag
	// defaults, so "no flag passed" and "flag passed with today's default"
	// land on the same preset row.
	DefaultStartStack = 1500
	DefaultStartSB    = 10
	DefaultHandLimit  = 0 // unlimited: play until someone busts
)

var defaultStackOpts = []int{500, 1000, 1500, 2000, 3000, 5000}
var defaultBlindOpts = []int{5, 10, 25, 50}     // small-blind presets; big blind is always 2x
var defaultHandOpts = []int{0, 10, 25, 50, 100} // 0 = unlimited (bust-only)

// menuModel is the pre-game dashboard: pick opponent/model/persona/stack/
// blind/hands/talk on one screen, enter to start. Replaces the old
// line-based RunPicker.
type menuModel struct {
	roster []agent.Adapter
	st     stats.Stats

	focus  int
	oppIdx int

	modelOpts [][]string // per-adapter options, "(default)" first
	modelInit []int      // per-adapter initial index (--model prefill)
	modelIdx  int

	personaOpts []string
	personaIdx  int

	stackOpts []int
	stackIdx  int

	blindOpts []int // small-blind presets; big blind is always 2x
	blindIdx  int

	handOpts []int // hand-limit presets; 0 = unlimited
	handIdx  int

	talkOn  bool
	entered bool
	quitted bool
}

// newMenuModel builds the dashboard state. personaFile gates the "custom"
// persona entry (callers pass agent.PersonalityPath(); tests a temp path).
func newMenuModel(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB, presetHands int, presetQuiet bool, personaFile string) menuModel {
	m := menuModel{roster: roster, st: st, talkOn: !presetQuiet}

	m.modelOpts = make([][]string, len(roster))
	m.modelInit = make([]int, len(roster))
	for i, a := range roster {
		opts := append([]string{menuDefaultModel}, a.Models...)
		sel := 0
		if presetModel != "" {
			sel = -1
			for j, o := range opts {
				if o == presetModel {
					sel = j
				}
			}
			if sel < 0 {
				opts = append(opts, presetModel)
				sel = len(opts) - 1
			}
		}
		m.modelOpts[i], m.modelInit[i] = opts, sel
	}
	m.modelIdx = m.modelInit[0]

	m.personaOpts = agent.PresetNames()
	if _, err := os.Stat(personaFile); err == nil {
		m.personaOpts = append([]string{menuCustomPersona}, m.personaOpts...)
	}
	if presetPersonality != "" {
		m.personaIdx = -1
		for j, o := range m.personaOpts {
			if o == presetPersonality {
				m.personaIdx = j
			}
		}
		if m.personaIdx < 0 {
			m.personaOpts = append(m.personaOpts, presetPersonality)
			m.personaIdx = len(m.personaOpts) - 1
		}
	}

	m.stackOpts = append([]int{}, defaultStackOpts...)
	if idx := indexOfInt(m.stackOpts, presetStack); idx >= 0 {
		m.stackIdx = idx
	} else {
		m.stackOpts = append(m.stackOpts, presetStack)
		m.stackIdx = len(m.stackOpts) - 1
	}

	m.blindOpts = append([]int{}, defaultBlindOpts...)
	if idx := indexOfInt(m.blindOpts, presetSB); idx >= 0 {
		m.blindIdx = idx
	} else {
		m.blindOpts = append(m.blindOpts, presetSB)
		m.blindIdx = len(m.blindOpts) - 1
	}

	m.handOpts = append([]int{}, defaultHandOpts...)
	if idx := indexOfInt(m.handOpts, presetHands); idx >= 0 {
		m.handIdx = idx
	} else {
		m.handOpts = append(m.handOpts, presetHands)
		m.handIdx = len(m.handOpts) - 1
	}

	return m
}

func indexOfInt(s []int, v int) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func (m menuModel) Init() tea.Cmd { return nil }

var menuDim = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

// handsLabel renders a hand-limit preset: 0 is "unlimited", otherwise the
// bare hand count.
func handsLabel(n int) string {
	if n == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", n)
}

func (m menuModel) View() string {
	talk := "on"
	if !m.talkOn {
		talk = "off"
	}
	sb := m.blindOpts[m.blindIdx]
	vals := [rowCount]string{
		rowOpponent: m.roster[m.oppIdx].DisplayName,
		rowModel:    m.modelOpts[m.oppIdx][m.modelIdx],
		rowPersona:  m.personaOpts[m.personaIdx],
		rowStack:    fmt.Sprintf("%d", m.stackOpts[m.stackIdx]),
		rowBlind:    fmt.Sprintf("%d/%d", sb, sb*2),
		rowHands:    handsLabel(m.handOpts[m.handIdx]),
		rowTalk:     talk,
	}
	labels := [rowCount]string{"opponent", "model", "persona", "stack", "blind", "hands", "table talk"}

	var b strings.Builder
	b.WriteString("♠ SHOWDOWN\n\n")
	for r := 0; r < rowCount; r++ {
		val := "  " + vals[r]
		if r == m.focus {
			val = "◀ " + vals[r] + " ▶"
		}
		line := fmt.Sprintf("  %-12s %s", labels[r], val)
		if r == rowOpponent {
			line += menuDim.Render("      " + m.st.Line(m.roster[m.oppIdx].Key))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + menuDim.Render("  [ enter ] deal me in    [ q ] quit") + "\n")
	return b.String()
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "up", "k":
		if m.focus > 0 {
			m.focus--
		}
	case "down", "j":
		if m.focus < rowCount-1 {
			m.focus++
		}
	case "left", "h":
		m.cycle(-1)
	case "right", "l":
		m.cycle(1)
	case "enter":
		m.entered = true
		return m, tea.Quit
	case "q", "ctrl+c":
		m.quitted = true
		return m, tea.Quit
	}
	return m, nil
}

// cycle moves the focused row's value by d, wrapping.
func (m *menuModel) cycle(d int) {
	wrap := func(i, n int) int { return ((i+d)%n + n) % n }
	switch m.focus {
	case rowOpponent:
		m.oppIdx = wrap(m.oppIdx, len(m.roster))
		m.modelIdx = m.modelInit[m.oppIdx]
	case rowModel:
		m.modelIdx = wrap(m.modelIdx, len(m.modelOpts[m.oppIdx]))
	case rowPersona:
		m.personaIdx = wrap(m.personaIdx, len(m.personaOpts))
	case rowStack:
		m.stackIdx = wrap(m.stackIdx, len(m.stackOpts))
	case rowBlind:
		m.blindIdx = wrap(m.blindIdx, len(m.blindOpts))
	case rowHands:
		m.handIdx = wrap(m.handIdx, len(m.handOpts))
	case rowTalk:
		m.talkOn = !m.talkOn
	}
}

// result maps menu selections back to the values main expects: "(default)"
// model → "", "custom" persona → "" (LoadPersonality's file-first default).
func (m menuModel) result() (agent.Adapter, string, bool, int, int, int) {
	ad := m.roster[m.oppIdx]
	if sel := m.modelOpts[m.oppIdx][m.modelIdx]; sel != menuDefaultModel {
		ad.Model = sel
	}
	persona := m.personaOpts[m.personaIdx]
	if persona == menuCustomPersona {
		persona = ""
	}
	return ad, persona, !m.talkOn, m.stackOpts[m.stackIdx], m.blindOpts[m.blindIdx], m.handOpts[m.handIdx]
}

// ErrMenuQuit reports the user backed out of the menu; main exits 0 silently.
var ErrMenuQuit = errors.New("menu quit")

// RunMenu shows the pre-game dashboard in the alt screen (same mode the
// game runs in, so menu → game is one seamless full-screen session) and
// blocks until the user starts a match or quits. Preset args come from
// --model, --personality, --stack, --blind, --hands, --quiet and pre-fill
// their rows. Returns (adapter, persona, quiet, startStack, startSmallBlind,
// handLimit, err).
func RunMenu(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB, presetHands int, presetQuiet bool) (agent.Adapter, string, bool, int, int, int, error) {
	if len(roster) == 0 {
		return agent.Adapter{}, "", false, 0, 0, 0, errors.New("no agent CLIs found on PATH (looked for: claude, codex, gemini)")
	}
	m := newMenuModel(roster, st, presetModel, presetPersonality, presetStack, presetSB, presetHands, presetQuiet, agent.PersonalityPath())
	out, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return agent.Adapter{}, "", false, 0, 0, 0, err
	}
	final := out.(menuModel)
	if final.quitted {
		return agent.Adapter{}, "", false, 0, 0, 0, ErrMenuQuit
	}
	ad, persona, quiet, stack, sb, hands := final.result()
	return ad, persona, quiet, stack, sb, hands, nil
}
```

- [ ] **Step 4: Keep the repo compiling — one-line shim in `main.go`**

`RunMenu`'s signature (one new input) and return arity (one new output) both changed. `main.go` also calls it (that file's actual owner is Task 4, which will fully rewire it). Find this line in `main.go`:

```go
		opp, personaArg, useQuiet, startStack, startSB, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *stackFlag, *blindFlag, *quiet)
```

Change it to pass `tui.DefaultHandLimit` as the new input (since `--hands` doesn't exist as a flag yet) and discard the new hand-limit return value with `_` (since there's no variable to receive it yet):

```go
		opp, personaArg, useQuiet, startStack, startSB, _, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *stackFlag, *blindFlag, tui.DefaultHandLimit, *quiet)
```

Run: `go build ./...`
Expected: builds cleanly.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run TestMenu`
Expected: PASS (all `TestMenu*` tests, plus `TestRunMenuEmptyRoster`).

Run: `go test ./...`
Expected: PASS (full suite — confirms Tasks 1-2's tests still pass alongside these changes).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/menu.go internal/tui/menu_test.go main.go
git commit -m "feat: menu row to pick a hand-count cap before the match"
```

---

### Task 4: `main.go` — `--hands` flag

**Files:**
- Modify: `main.go`

**Interfaces:**
- Consumes: `tui.DefaultHandLimit` (Task 3), `tui.RunMenu(..., presetHands int, ...) (..., handLimit int, error)` (Task 3), `tui.NewModel(..., handLimit int, log)` (Task 2).
- Produces: none (leaf — this is the CLI entrypoint).

No dedicated test file exists for `main.go` (glue code) — this task is implementation-only, verified by a full build, vet, test run, and a manual smoke check.

- [ ] **Step 1: Add the flag**

In `main.go`, change:

```go
	stackFlag := flag.Int("stack", tui.DefaultStartStack, "starting chip stack per player")
	blindFlag := flag.Int("blind", tui.DefaultStartSB, "starting small blind (big blind is always 2x)")
	quiet := flag.Bool("quiet", false, "disable table talk")
```

to:

```go
	stackFlag := flag.Int("stack", tui.DefaultStartStack, "starting chip stack per player")
	blindFlag := flag.Int("blind", tui.DefaultStartSB, "starting small blind (big blind is always 2x)")
	handsFlag := flag.Int("hands", tui.DefaultHandLimit, "cap the match at this many hands (0 = unlimited, play until someone busts; tied stacks at the cap continue until untied)")
	quiet := flag.Bool("quiet", false, "disable table talk")
```

- [ ] **Step 2: Thread the value through both the direct-agent and menu paths**

Change:

```go
	var opp agent.Adapter
	var personaArg string
	useQuiet := *quiet
	startStack, startSB := *stackFlag, *blindFlag
	if *agentFlag != "" {
```

to:

```go
	var opp agent.Adapter
	var personaArg string
	useQuiet := *quiet
	startStack, startSB, handLimit := *stackFlag, *blindFlag, *handsFlag
	if *agentFlag != "" {
```

Then change the menu-branch `RunMenu` call (which currently discards the hand-limit return with `_`, from Task 3's shim):

```go
		opp, personaArg, useQuiet, startStack, startSB, _, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *stackFlag, *blindFlag, tui.DefaultHandLimit, *quiet)
```

to:

```go
		opp, personaArg, useQuiet, startStack, startSB, handLimit, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *stackFlag, *blindFlag, *handsFlag, *quiet)
```

(If your checkout's line differs slightly from Task 3's shim text shown above, locate the `tui.RunMenu(...)` call directly and apply the same intent: pass `*handsFlag` instead of `tui.DefaultHandLimit`, and assign its hand-limit return into `handLimit` instead of discarding it with `_`.)

- [ ] **Step 3: Pass the value into `NewModel`**

Change (this is Task 2's shim, with its TODO comment):

```go
	dir, _ := os.Getwd()
	// TODO: thread configurable --hands flag through here (tracked separately).
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, 0, dlog)
```

to:

```go
	dir, _ := os.Getwd()
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, handLimit, dlog)
```

(If your checkout's line differs slightly from Task 2's shim text shown above — e.g. the TODO comment wording — locate the `tui.NewModel(...)` call directly, remove any such TODO comment above it, and replace the hardcoded `0` with `handLimit`.)

- [ ] **Step 4: Build, vet, and test**

Run: `go build ./...`
Expected: builds cleanly with no errors.

Run: `go vet ./...`
Expected: no warnings.

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 5: Smoke-test**

Run: `go run . --help` and confirm the `-hands` flag is listed with default `0` and the help text above.

If an agent CLI is available on `PATH` (e.g. `codex`): run `go run . --agent codex --hands 3 --quiet` and confirm the match ends after 3 hands (or sooner on a bust) rather than continuing indefinitely. If no agent CLI is available, note this in the report and rely on build/vet/test plus the `--help` check.

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "feat: --hands flag to cap match length"
```

---

## Self-Review

**Spec coverage:**
- "fixed hand-count match" → Task 1 (`Match.handLimit`, `Over()`/`Winner()` logic) is the core mechanic; Tasks 2-4 thread it through the same layers the recent `--stack`/`--blind` feature already established.
- "cap alongside bust, whichever first" (user's choice) → `Over()` in Task 1 checks `EndedByBust()` before the hand-limit condition, and the hand-limit condition never fires when a bust already happened — a bust always wins regardless of hand count.
- "sudden-death on ties" (user's choice) → `Over()`'s hand-limit branch requires `m.Stacks[0] != m.Stacks[1]`; a tie at the limit simply returns `false` and the existing game loop (unchanged) deals another hand automatically, re-checking `Over()` after it. No new loop or state was needed — confirmed by `TestHandLimitSuddenDeathContinuesWhileTied`.
- "--hands flag + menu row" (user's choice) → Task 3 (menu row, mirrors stack/blind exactly) + Task 4 (flag, mirrors `--stack`/`--blind` exactly).
- Table-truth rule (this plan's own finding, not explicitly requested but required by the codebase's established precedent from the digest.go fix) → Task 2's `matchOverOutcome` ensures the agent's own match-over reaction never claims a bust that didn't happen.

**Placeholder scan:** none — every step has complete code, exact file paths, and runnable commands with expected output. The three "if your checkout's line differs slightly" notes in Tasks 2 and 4 are explicit fallback instructions (locate the function/line by name, apply a named intent), not vague placeholders — they exist because each task's shim text is produced by the *previous* task's implementer and could vary in incidental details (comment wording) while the code's shape stays fixed.

**Type consistency:** `NewMatch(startStack, startSB, handLimit int) *Match` (Task 1) → consumed identically in Task 2's `poker.NewMatch(startStack, startSB, handLimit)`. `NewModel(..., startStack, startSB, handLimit int, log)` (Task 2) → consumed identically in Task 4's `tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, handLimit, dlog)`. `RunMenu(...) (agent.Adapter, string, bool, int, int, int, error)` (Task 3) → consumed identically in Task 4's 7-value assignment `opp, personaArg, useQuiet, startStack, startSB, handLimit, err = tui.RunMenu(...)`.
