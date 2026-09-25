# Configurable Starting Stack & Blinds Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let players pick the starting chip stack and starting small blind (big blind is always 2x) from the menu screen or via `--stack`/`--blind` flags, defaulting to today's fixed 1500/10/20.

**Architecture:** `poker.Match` already isolates all blind/stack state behind `NewMatch`/`Blinds()`; make both configurable there, scaling the existing 6-level blind schedule proportionally to the chosen starting small blind. Thread the two values through `tui.NewModel` (which currently hardcodes them into `poker.NewMatch()`/`agent.NewDigest()`) and through the menu dashboard (two new cycling rows, same pattern as the existing model/persona rows) and CLI flags (same pattern as `--model`).

**Tech Stack:** Go, bubbletea (menu TUI), existing `internal/poker`, `internal/agent`, `internal/tui` packages.

## Global Constraints

- Default starting stack stays **1500** per player; default starting small blind stays **10** (big blind **20**) — behavior for anyone who doesn't touch the new flags/menu rows must be byte-for-byte identical to today.
- Big blind is always exactly 2x the small blind (no independent bb configuration) — matches every existing blind-schedule entry.
- The blind escalation schedule keeps its existing ratios (1x, 1.5x, 2.5x, 5x, 10x, 20x of the starting small blind), still stepping every 10 hands, still capping at the 6th level — only the base value changes.
- `internal/agent.Digest`'s first-hand message must state the true configured starting stack, never a hardcoded number (table-truth rule from prior work — the agent's "memory" of the match must never say something false about game state).

---

### Task 1: `poker.Match` — configurable starting stack and blind schedule

**Files:**
- Modify: `internal/poker/match.go`
- Test: `internal/poker/match_test.go`

**Interfaces:**
- Produces: `poker.NewMatch(startStack, startSB int) *Match` (replaces the old zero-arg `NewMatch()`). `Match.Blinds() (sb, bb int)` behavior unchanged in shape, now scaled off the constructor's `startSB`.

- [ ] **Step 1: Update existing tests to the new constructor signature and add scaling tests**

Replace the full contents of `internal/poker/match_test.go`:

```go
package poker

import "testing"

func TestBlindSchedule(t *testing.T) {
	m := NewMatch(1500, 10)
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
	m := NewMatch(3000, 25)
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
	m := NewMatch(3000, 25)
	if m.Stacks != [2]int{3000, 3000} {
		t.Errorf("Stacks = %v, want [3000 3000]", m.Stacks)
	}
}

func TestButtonAlternates(t *testing.T) {
	m := NewMatch(1500, 10)
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
	m := NewMatch(1500, 10) // hand 1: human is button = seat 0
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
}
```

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./internal/poker/...`
Expected: FAIL — `too many arguments in call to NewMatch` (production code still has the zero-arg constructor).

- [ ] **Step 3: Implement the scaled schedule and configurable constructor**

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

	levels [][2]int
}

func NewMatch(startStack, startSB int) *Match {
	return &Match{Stacks: [2]int{startStack, startStack}, HandNum: 1, levels: blindLevelsFor(startSB)}
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

func (m *Match) Over() bool { return m.Stacks[0] == 0 || m.Stacks[1] == 0 }

func (m *Match) Winner() int {
	switch {
	case !m.Over():
		return -1
	case m.Stacks[0] == 0:
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/poker/...`
Expected: PASS (all of `TestBlindSchedule`, `TestBlindScheduleScalesWithStartSB`, `TestNewMatchCustomStartingStack`, `TestButtonAlternates`, `TestNextHandAndBust`).

- [ ] **Step 5: Commit**

```bash
git add internal/poker/match.go internal/poker/match_test.go
git commit -m "feat: configurable starting stack and scaled blind schedule"
```

---

### Task 2: `agent.Digest` — dynamic starting-stack message

**Files:**
- Modify: `internal/agent/digest.go`
- Test: `internal/agent/prompt_test.go:53` (the existing `TestDigestFlow`'s `NewDigest()` call) plus a new test

**Interfaces:**
- Consumes: nothing new.
- Produces: `agent.NewDigest(startStack int) *Digest` (replaces the zero-arg constructor). Task 3 calls this with the match's configured starting stack.

- [ ] **Step 1: Update the existing call site and add a failing test**

In `internal/agent/prompt_test.go`, change line 53 from:

```go
	d := NewDigest()
```

to:

```go
	d := NewDigest(1500)
```

Then add this test right after `TestDigestFlow` (which ends at line 68, just before the `TestNeedlingRuleLastInFormatRules` comment block):

```go
func TestDigestFirstHandMessageUsesStartStack(t *testing.T) {
	d := NewDigest(3000)
	if !strings.Contains(d.String(), "Stacks even at 3000") {
		t.Errorf("first-hand digest = %q, want mention of 3000", d.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./internal/agent/... -run TestDigest`
Expected: FAIL — `not enough arguments in call to NewDigest` (production code still takes zero args).

- [ ] **Step 3: Implement the dynamic constructor and message**

In `internal/agent/digest.go`, change the import and the struct/constructor/`String()`:

```go
package agent

import (
	"fmt"
	"strings"
)

// Digest is the agent's stateless "memory": hand summaries plus table talk.
type Digest struct {
	events     []string
	handTalk   []string
	startStack int
}

func NewDigest(startStack int) *Digest { return &Digest{startStack: startStack} }
```

And update `String()`:

```go
func (d *Digest) String() string {
	if len(d.events) == 0 {
		return fmt.Sprintf("First hand of the match. Stacks even at %d.", d.startStack)
	}
	return strings.Join(d.events, "\n")
}
```

Leave `AddEvent`, `AddTalk`, `HandTalk`, `EndHand` untouched.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/... -run TestDigest`
Expected: PASS (`TestDigestFlow`, `TestDigestFirstHandMessageUsesStartStack`).

- [ ] **Step 5: Commit**

```bash
git add internal/agent/digest.go internal/agent/prompt_test.go
git commit -m "fix: digest first-hand message states true configured starting stack"
```

---

### Task 3: `tui.NewModel` — thread starting stack/blind into Match and Digest

**Files:**
- Modify: `internal/tui/app.go:93-110`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `poker.NewMatch(startStack, startSB int) *Match` (Task 1), `agent.NewDigest(startStack int) *Digest` (Task 2).
- Produces: `tui.NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir, personality string, startStack, startSB int, log *debuglog.Logger) Model` — Task 5 (main.go) calls this with flag/menu values.

- [ ] **Step 1: Add a failing test for the new parameters**

Add this test to `internal/tui/app_test.go`, right after `testModel` (after line 23):

```go
func TestNewModelUsesCustomStackAndBlind(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 3000, 25, nil)
	if m.match.Stacks != [2]int{3000, 3000} {
		t.Errorf("Stacks = %v, want [3000 3000]", m.match.Stacks)
	}
	sb, bb := m.match.Blinds()
	if sb != 25 || bb != 50 {
		t.Errorf("Blinds = %d/%d, want 25/50", sb, bb)
	}
}
```

- [ ] **Step 2: Run test to verify it fails to compile**

Run: `go test ./internal/tui/... -run TestNewModelUsesCustomStackAndBlind`
Expected: FAIL — `too many arguments in call to NewModel`.

- [ ] **Step 3: Update `NewModel`'s signature and body**

In `internal/tui/app.go`, replace:

```go
func NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir, personality string, log *debuglog.Logger) Model {
	in := textinput.New()
	in.CharLimit = 120
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	seed := time.Now().UnixNano()
	log.Log("session_start", map[string]any{
		"agent_key": opp.Key, "agent_name": opp.DisplayName,
		"model": opp.Model, "quiet": quiet, "dir": dir, "seed": seed,
		"personality": personality,
	})
	return Model{
		opp: opp, stats: st, statsPath: statsPath, quiet: quiet, dir: dir,
		personality: personality, log: log,
		rng:   rand.New(rand.NewSource(seed)),
		match: poker.NewMatch(), digest: agent.NewDigest(),
		input: in, spin: sp,
	}
}
```

with:

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
		match: poker.NewMatch(startStack, startSB), digest: agent.NewDigest(startStack),
		input: in, spin: sp,
	}
}
```

- [ ] **Step 4: Fix the other 8 call sites in `internal/tui/app_test.go`**

These are the only other places `NewModel` is called (grep confirmed). Apply each change exactly:

1. Line 22 (`testModel` helper), change:
   ```go
   return NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", "test-persona", nil)
   ```
   to:
   ```go
   return NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", "test-persona", 1500, 10, nil)
   ```

2. The exact substring `NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", l)` appears 3 times (used to start persistent sessions in test setup). Replace **all 3** occurrences with:
   ```go
   NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, l)
   ```

3. The exact substring `NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", nil)` appears 2 times. Replace **both** occurrences with:
   ```go
   NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, nil)
   ```

4. The exact substring `NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "PERSONA-MARKER", l)` appears 2 times. Replace **both** occurrences with:
   ```go
   NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "PERSONA-MARKER", 1500, 10, l)
   ```

- [ ] **Step 5: Run the full tui test suite to verify everything passes**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: thread configurable starting stack/blind into NewModel"
```

---

### Task 4: Menu dashboard — stack and blind rows

**Files:**
- Modify: `internal/tui/menu.go` (full rewrite, file is small)
- Test: `internal/tui/menu_test.go` (full rewrite, same reason)

**Interfaces:**
- Consumes: nothing new from other tasks (menu.go's constructor/functions are the leaf here).
- Produces: `tui.DefaultStartStack = 1500`, `tui.DefaultStartSB = 10` (exported constants — Task 5's main.go uses these as flag defaults). `tui.RunMenu(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB int, presetQuiet bool) (agent.Adapter, string, bool, int, int, error)` — return order is `(adapter, persona, quiet, stack, smallBlind, err)`.

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
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
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
	m := newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", DefaultStartStack, DefaultStartSB, true, noPersonaFile(t))
	if m.modelOpts[0][m.modelIdx] != "sonnet" {
		t.Errorf("selected model = %q, want sonnet", m.modelOpts[0][m.modelIdx])
	}
	if m.talkOn {
		t.Error("presetQuiet=true should start talkOn=false")
	}
	// flag value not in any list → appended verbatim everywhere
	m = newMenuModel(menuRoster(), stats.Stats{}, "gpt-x", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	if got := m.modelOpts[0][m.modelInit[0]]; got != "gpt-x" {
		t.Errorf("claude prefill = %q, want gpt-x", got)
	}
	if got := m.modelOpts[1][m.modelInit[1]]; got != "gpt-x" {
		t.Errorf("codex prefill = %q, want gpt-x", got)
	}
}

func TestMenuPersonaOptions(t *testing.T) {
	// no user file: presets only, needler (index 0) selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	if m.personaOpts[0] != "needler" || m.personaIdx != 0 {
		t.Errorf("personaOpts[0]=%q idx=%d, want needler 0", m.personaOpts[0], m.personaIdx)
	}
	// user file exists: custom prepended and selected
	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("be weird"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, f)
	if m.personaOpts[0] != menuCustomPersona || m.personaIdx != 0 {
		t.Errorf("with file: personaOpts[0]=%q idx=%d, want custom 0", m.personaOpts[0], m.personaIdx)
	}
	// --personality preset name → selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "degen", DefaultStartStack, DefaultStartSB, false, f)
	if m.personaOpts[m.personaIdx] != "degen" {
		t.Errorf("prefill persona = %q, want degen", m.personaOpts[m.personaIdx])
	}
	// --personality path → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "/tmp/evil.md", DefaultStartStack, DefaultStartSB, false, f)
	if m.personaOpts[m.personaIdx] != "/tmp/evil.md" {
		t.Errorf("prefill persona = %q, want /tmp/evil.md", m.personaOpts[m.personaIdx])
	}
}

func TestMenuStackAndBlindOptions(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != DefaultStartStack {
		t.Errorf("default stack = %d, want %d", got, DefaultStartStack)
	}
	if got := m.blindOpts[m.blindIdx]; got != DefaultStartSB {
		t.Errorf("default small blind = %d, want %d", got, DefaultStartSB)
	}
}

func TestMenuStackAndBlindPrefill(t *testing.T) {
	// values already in the preset lists → selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 3000, 25, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != 3000 {
		t.Errorf("stack prefill = %d, want 3000", got)
	}
	if got := m.blindOpts[m.blindIdx]; got != 25 {
		t.Errorf("blind prefill = %d, want 25", got)
	}
	// values not in the preset lists → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", 1234, 7, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != 1234 {
		t.Errorf("stack custom prefill = %d, want 1234", got)
	}
	if got := m.blindOpts[m.blindIdx]; got != 7 {
		t.Errorf("blind custom prefill = %d, want 7", got)
	}
}

func TestMenuNavigationAndCycling(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))

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
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
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

func TestMenuModelResetsOnOpponentChange(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
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
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	v, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = v.(menuModel)
	if !m.entered || cmd == nil {
		t.Error("enter should set entered and return tea.Quit")
	}

	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	v, cmd = m.Update(key("q"))
	m = v.(menuModel)
	if !m.quitted || cmd == nil {
		t.Error("q should set quitted and return tea.Quit")
	}
}

func TestMenuResult(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	ad, persona, quiet, stack, sb := m.result()
	if ad.Key != "claude" || ad.Model != "" || persona != "needler" || quiet || stack != DefaultStartStack || sb != DefaultStartSB {
		t.Errorf("defaults: got key=%q model=%q persona=%q quiet=%v stack=%d sb=%d",
			ad.Key, ad.Model, persona, quiet, stack, sb)
	}

	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", 3000, 25, true, f)
	ad, persona, quiet, stack, sb = m.result()
	if ad.Model != "sonnet" || persona != "" || !quiet || stack != 3000 || sb != 25 {
		t.Errorf("got model=%q persona=%q quiet=%v stack=%d sb=%d, want sonnet \"\" true 3000 25",
			ad.Model, persona, quiet, stack, sb)
	}
}

func TestMenuView(t *testing.T) {
	st := stats.Stats{"claude": {Wins: 2, Losses: 1}}
	m := newMenuModel(menuRoster(), st, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	v := stripANSI(m.View())

	for _, want := range []string{
		"♠ SHOWDOWN",
		"◀ Claude Code ▶", // focused row wears the arrows
		"vs claude: 2–1",  // stats line for the selected opponent
		"needler",
		"1500",
		"10/20",
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
	_, _, _, _, _, err := RunMenu(nil, stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false)
	if err == nil || !strings.Contains(err.Error(), "no agent CLIs found on PATH") {
		t.Errorf("err = %v, want no-agents error", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./internal/tui/... -run TestMenu`
Expected: FAIL — `undefined: DefaultStartStack` (and related), since `menu.go` hasn't changed yet.

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
	rowTalk
	rowCount
)

const (
	menuDefaultModel  = "(default)"
	menuCustomPersona = "custom"

	// DefaultStartStack and DefaultStartSB are the sit-and-go defaults. main
	// uses these as --stack/--blind flag defaults, so "no flag passed" and
	// "flag passed with today's default" land on the same preset row.
	DefaultStartStack = 1500
	DefaultStartSB    = 10
)

var defaultStackOpts = []int{500, 1000, 1500, 2000, 3000, 5000}
var defaultBlindOpts = []int{5, 10, 25, 50} // small-blind presets; big blind is always 2x

// menuModel is the pre-game dashboard: pick opponent/model/persona/stack/
// blind/talk on one screen, enter to start. Replaces the old line-based
// RunPicker.
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

	talkOn  bool
	entered bool
	quitted bool
}

// newMenuModel builds the dashboard state. personaFile gates the "custom"
// persona entry (callers pass agent.PersonalityPath(); tests a temp path).
func newMenuModel(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB int, presetQuiet bool, personaFile string) menuModel {
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
		rowTalk:     talk,
	}
	labels := [rowCount]string{"opponent", "model", "persona", "stack", "blind", "table talk"}

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
	case rowTalk:
		m.talkOn = !m.talkOn
	}
}

// result maps menu selections back to the values main expects: "(default)"
// model → "", "custom" persona → "" (LoadPersonality's file-first default).
func (m menuModel) result() (agent.Adapter, string, bool, int, int) {
	ad := m.roster[m.oppIdx]
	if sel := m.modelOpts[m.oppIdx][m.modelIdx]; sel != menuDefaultModel {
		ad.Model = sel
	}
	persona := m.personaOpts[m.personaIdx]
	if persona == menuCustomPersona {
		persona = ""
	}
	return ad, persona, !m.talkOn, m.stackOpts[m.stackIdx], m.blindOpts[m.blindIdx]
}

// ErrMenuQuit reports the user backed out of the menu; main exits 0 silently.
var ErrMenuQuit = errors.New("menu quit")

// RunMenu shows the pre-game dashboard in the alt screen (same mode the
// game runs in, so menu → game is one seamless full-screen session) and
// blocks until the user starts a match or quits. Preset args come from
// --model, --personality, --stack, --blind, --quiet and pre-fill their rows.
// Returns (adapter, persona, quiet, startStack, startSmallBlind, err).
func RunMenu(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB int, presetQuiet bool) (agent.Adapter, string, bool, int, int, error) {
	if len(roster) == 0 {
		return agent.Adapter{}, "", false, 0, 0, errors.New("no agent CLIs found on PATH (looked for: claude, codex, gemini)")
	}
	m := newMenuModel(roster, st, presetModel, presetPersonality, presetStack, presetSB, presetQuiet, agent.PersonalityPath())
	out, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return agent.Adapter{}, "", false, 0, 0, err
	}
	final := out.(menuModel)
	if final.quitted {
		return agent.Adapter{}, "", false, 0, 0, ErrMenuQuit
	}
	ad, persona, quiet, stack, sb := final.result()
	return ad, persona, quiet, stack, sb, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run TestMenu`
Expected: PASS (all `TestMenu*` tests, plus `TestRunMenuEmptyRoster`).

Run: `go test ./internal/tui/...`
Expected: PASS (full package — confirms `NewModel`-related tests from Task 3 still pass alongside the menu changes).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/menu.go internal/tui/menu_test.go
git commit -m "feat: menu rows to pick starting stack and blind before the match"
```

---

### Task 5: `main.go` — `--stack`/`--blind` flags

**Files:**
- Modify: `main.go`

**Interfaces:**
- Consumes: `tui.DefaultStartStack`, `tui.DefaultStartSB` (Task 4), `tui.RunMenu(..., presetStack, presetSB int, ...) (..., stack, sb int, error)` (Task 4), `tui.NewModel(..., startStack, startSB int, log)` (Task 3).
- Produces: none (leaf — this is the CLI entrypoint).

No dedicated test file exists for `main.go` today (glue code, verified by `find . -maxdepth 1 -name "main_test.go"` returning nothing) — this task is implementation-only, verified by a full build and a manual smoke run.

- [ ] **Step 1: Add the flags**

In `main.go`, change:

```go
	agentFlag := flag.String("agent", "", "opponent agent key (claude, codex, gemini)")
	modelFlag := flag.String("model", "", "model for the agent CLI (claude: haiku/sonnet/opus; codex/gemini: passed through)")
	personalityFlag := flag.String("personality", "", "table persona: preset name (needler, unhinged, polite, silent, degen) or path to a .md file; default ~/.showdown/personality.md if present, else needler")
	quiet := flag.Bool("quiet", false, "disable table talk")
	debug := flag.Bool("debug", false, "write a JSONL debug transcript to ~/.showdown/")
	flag.Parse()
```

to:

```go
	agentFlag := flag.String("agent", "", "opponent agent key (claude, codex, gemini)")
	modelFlag := flag.String("model", "", "model for the agent CLI (claude: haiku/sonnet/opus; codex/gemini: passed through)")
	personalityFlag := flag.String("personality", "", "table persona: preset name (needler, unhinged, polite, silent, degen) or path to a .md file; default ~/.showdown/personality.md if present, else needler")
	stackFlag := flag.Int("stack", tui.DefaultStartStack, "starting chip stack per player")
	blindFlag := flag.Int("blind", tui.DefaultStartSB, "starting small blind (big blind is always 2x)")
	quiet := flag.Bool("quiet", false, "disable table talk")
	debug := flag.Bool("debug", false, "write a JSONL debug transcript to ~/.showdown/")
	flag.Parse()
```

- [ ] **Step 2: Thread the values through both the direct-agent and menu paths**

Change:

```go
	var opp agent.Adapter
	var personaArg string
	useQuiet := *quiet
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
		if *modelFlag != "" {
			opp.Model = *modelFlag
		}
		personaArg = *personalityFlag
	} else {
		opp, personaArg, useQuiet, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *quiet)
		if errors.Is(err, tui.ErrMenuQuit) {
			os.Exit(0)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
```

to:

```go
	var opp agent.Adapter
	var personaArg string
	useQuiet := *quiet
	startStack, startSB := *stackFlag, *blindFlag
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
		if *modelFlag != "" {
			opp.Model = *modelFlag
		}
		personaArg = *personalityFlag
	} else {
		opp, personaArg, useQuiet, startStack, startSB, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *stackFlag, *blindFlag, *quiet)
		if errors.Is(err, tui.ErrMenuQuit) {
			os.Exit(0)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
```

- [ ] **Step 3: Pass the values into `NewModel`**

Change:

```go
	dir, _ := os.Getwd()
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, dlog)
```

to:

```go
	dir, _ := os.Getwd()
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, dlog)
```

- [ ] **Step 4: Build and smoke-test**

Run: `go build ./...`
Expected: builds cleanly with no errors.

Run: `go vet ./...`
Expected: no warnings.

Run: `go run . --agent codex --stack 3000 --blind 25 --quiet` (bypasses menu; requires a `codex` binary on `PATH` — if none is installed, instead run `go run . --stack 3000 --blind 25` and pick any detected agent from the menu, confirming the menu shows "stack 3000" and "blind 25/50" as the pre-selected rows).
Expected: match starts, banner reads `hand 1 — blinds 25/50` (not `10/20`), first agent decision's debug digest (if `--debug` also passed) states "Stacks even at 3000."

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "feat: --stack and --blind flags for custom starting chips and blinds"
```

---

## Self-Review

**Spec coverage:**
- "add into menu the option to change blind and starting amounts before the game" → Task 4 (menu rows) + Task 5 (flags, since the direct-`--agent` path bypasses the menu entirely and still needs a way to set these).
- "load current as default" → `DefaultStartStack = 1500`, `DefaultStartSB = 10` in Task 4, used both as menu preset defaults and as the flag defaults in Task 5 — nobody who doesn't touch the new controls sees any behavior change.
- Prior investigation's flagged risk (digest.go:37 hardcoded "1500") → Task 2, done before Task 3 wires it in, so Task 3 never reintroduces a hardcoded value.
- Prior investigation's flagged risk (blindLevels hardcoded 10/20 schedule) → Task 1, scaling proportionally so the escalation feel is preserved at any starting blind.

**Placeholder scan:** none — every step above has complete code, exact file paths, and runnable commands with expected output.

**Type consistency:** `NewMatch(startStack, startSB int) *Match` (Task 1) → consumed identically in Task 3's `poker.NewMatch(startStack, startSB)`. `NewDigest(startStack int) *Digest` (Task 2) → consumed identically in Task 3's `agent.NewDigest(startStack)`. `RunMenu(...) (agent.Adapter, string, bool, int, int, error)` (Task 4) → consumed identically in Task 5's 6-value assignment `opp, personaArg, useQuiet, startStack, startSB, err = tui.RunMenu(...)`. `NewModel(..., startStack, startSB int, log *debuglog.Logger) Model` (Task 3) → consumed identically in Task 5's `tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, dlog)`.
